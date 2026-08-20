package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sapliy/sapliy-core/internal/playbook"
	"github.com/sapliy/sapliy-core/pkg/apikey"
	pb "github.com/sapliy/sapliy-core/proto/auth"
	"google.golang.org/grpc"
)

// resetPlaybookState isolates tests that mutate the shared Log/Engine
// singletons.
func resetPlaybookState() {
	Log = playbook.NewDecisionLog()
	Engine = playbook.NewEngine()
}

func playbookTestHandler(t *testing.T) http.Handler {
	t.Helper()
	handler, _, _ := newTestGateway(t, &mockAuthClient{
		ValidateKeyFunc: func(_ context.Context, _ *pb.ValidateKeyRequest, _ ...grpc.CallOption) (*pb.ValidateKeyResponse, error) {
			return validKeyResponse(), nil
		},
	})
	return handler
}

func playbookAuthedRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	apiKeyStr, _, _ := apikey.GenerateKey("sk", testHMACSecret)
	req.Header.Set("Authorization", "Bearer "+apiKeyStr)
	return req
}

func TestPlaybookCatalog(t *testing.T) {
	handler := playbookTestHandler(t)

	req := playbookAuthedRequest(http.MethodGet, "/v1/playbooks", "")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d. body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Data []PlaybookCatalogEntry `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Data) != 3 {
		t.Fatalf("len(data) = %d, want 3", len(resp.Data))
	}

	gotTypes := map[string]bool{}
	for _, e := range resp.Data {
		gotTypes[e.Type] = true
		if e.Name == "" || e.Description == "" {
			t.Errorf("catalog entry %q missing name/description", e.Type)
		}
		if e.DefaultConfig == nil {
			t.Errorf("catalog entry %q missing defaultConfig", e.Type)
		}
	}
	for _, want := range []string{"revenue_recovery", "refund_approval", "invoice_reminders"} {
		if !gotTypes[want] {
			t.Errorf("catalog missing playbook type %q", want)
		}
	}
}

func TestPlaybookPreview(t *testing.T) {
	handler := playbookTestHandler(t)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantType   string
	}{
		{
			name:       "revenue recovery goal",
			body:       `{"goal":"recover failed subscription payments"}`,
			wantStatus: http.StatusOK,
			wantType:   "revenue_recovery",
		},
		{
			name:       "refund goal",
			body:       `{"goal":"process refunds and returns"}`,
			wantStatus: http.StatusOK,
			wantType:   "refund_approval",
		},
		{
			name:       "missing goal",
			body:       `{"goal":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid json",
			body:       `not-json`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := playbookAuthedRequest(http.MethodPost, "/v1/playbooks/preview", tt.body)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d. body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var resp previewResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.PlaybookType != tt.wantType {
				t.Errorf("playbookType = %q, want %q", resp.PlaybookType, tt.wantType)
			}
			if resp.Confidence < 0.72 || resp.Confidence > 0.95 {
				t.Errorf("confidence = %v, want within [0.72, 0.95]", resp.Confidence)
			}
			if len(resp.Steps) == 0 {
				t.Error("steps should not be empty")
			}
			if resp.Summary == "" {
				t.Error("summary should not be empty")
			}
		})
	}
}

func TestPlaybookIngestAndDecisions(t *testing.T) {
	resetPlaybookState()
	handler := playbookTestHandler(t)

	stripeBody := `{
		"provider":"stripe",
		"event":"invoice.payment_failed",
		"payload":{
			"id":"evt_1",
			"data":{"object":{
				"id":"in_1",
				"amount":4900,
				"currency":"usd",
				"customer":"cus_1",
				"subscription":"sub_1",
				"payment_intent":"pi_123",
				"created":1724000000
			}}
		}
	}`

	req := playbookAuthedRequest(http.MethodPost, "/v1/playbooks/ingest", stripeBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ingest status: got %d, want %d. body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var ingestResp struct {
		Data []playbook.DecisionEntry `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ingestResp); err != nil {
		t.Fatalf("failed to parse ingest response: %v", err)
	}
	if len(ingestResp.Data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(ingestResp.Data))
	}
	entry := ingestResp.Data[0]
	if entry.Action != "schedule_retry" {
		t.Errorf("action = %q, want schedule_retry", entry.Action)
	}
	if entry.Hash == "" {
		t.Error("decision should carry a hash")
	}
	if entry.TenantID != "org_test" {
		t.Errorf("tenantId = %q, want org_test (from X-Org-ID)", entry.TenantID)
	}

	// The shared log must be a coherent hash chain.
	if err := Log.Verify(); err != nil {
		t.Errorf("Log.Verify(): %v", err)
	}

	// Decisions endpoint must return the recorded entry.
	req = playbookAuthedRequest(http.MethodGet, "/v1/playbooks/decisions", "")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("decisions status: got %d, want %d", w.Code, http.StatusOK)
	}
	var decisionsResp struct {
		Data []playbook.DecisionEntry `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &decisionsResp); err != nil {
		t.Fatalf("failed to parse decisions response: %v", err)
	}
	if len(decisionsResp.Data) == 0 {
		t.Fatal("decisions endpoint should return entries")
	}
	found := false
	for _, e := range decisionsResp.Data {
		if e.Action == "schedule_retry" && e.Hash == entry.Hash {
			found = true
		}
	}
	if !found {
		t.Error("decisions endpoint should return the ingested schedule_retry entry")
	}
}

func TestPlaybookIngest_InvalidProvider(t *testing.T) {
	resetPlaybookState()
	handler := playbookTestHandler(t)

	req := playbookAuthedRequest(http.MethodPost, "/v1/playbooks/ingest", `{"provider":"unknown","event":"x","payload":{}}`)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
	if code := apiErrorCode(t, w.Body.String()); code != "BAD_REQUEST" {
		t.Errorf("error code: got %q, want %q", code, "BAD_REQUEST")
	}
}
