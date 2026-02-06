package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sapliy/fintech-ecosystem/internal/flow"
	"github.com/sapliy/fintech-ecosystem/internal/flow/domain"
	"github.com/sapliy/fintech-ecosystem/internal/flow/testutil"
)

func TestFlowServer_StartDebugSession(t *testing.T) {
	// Setup
	repo := testutil.NewMockFlowRepository()
	debugService := flow.NewDebugService(repo)
	server := NewFlowServer(debugService, repo)

	// Create a test flow
	testFlow := &domain.Flow{
		ID:      "flow_test",
		OrgID:   "org_123",
		ZoneID:  "zone_456",
		Name:    "Test Flow",
		Enabled: true,
		Nodes: []domain.Node{
			{ID: "trigger", Type: domain.NodeTrigger},
		},
	}

	// Use same repo instance for everything
	if err := repo.CreateFlow(context.Background(), testFlow); err != nil {
		t.Fatalf("Failed to create test flow: %v", err)
	}

	// Test request
	reqBody := map[string]interface{}{
		"level": "info",
	}
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/flows/flow_test/zones/zone_456/debug", bytes.NewBuffer(reqBodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	server.StartDebugSession(w, req)

	// Log response for debugging
	t.Logf("Response status: %d", w.Code)
	t.Logf("Response body: %s", w.Body.String())

	// Verify
	if w.Code == http.StatusOK {
		var session domain.DebugSession
		if err := json.Unmarshal(w.Body.Bytes(), &session); err == nil {
			if session.FlowID != "flow_test" {
				t.Errorf("Expected flow ID flow_test, got %s", session.FlowID)
			}

			if session.ZoneID != "zone_456" {
				t.Errorf("Expected zone ID zone_456, got %s", session.ZoneID)
			}

			if !session.Active {
				t.Error("Session should be active")
			}
		} else {
			t.Errorf("Failed to unmarshal response: %v", err)
		}
	} else {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
