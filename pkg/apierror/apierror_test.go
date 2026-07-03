package apierror_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sapliy/sapliy-core/pkg/apierror"
)

// envelope mirrors the private type used in Write() so we can decode responses.
type envelope struct {
	Error *errorBody `json:"error"`
}

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	Details   any    `json:"details,omitempty"`
}

// decodeResponse is a helper that records a Write() call and decodes the JSON body.
func decodeResponse(t *testing.T, apiErr *apierror.APIError) (statusCode int, body envelope) {
	t.Helper()
	w := httptest.NewRecorder()
	apiErr.Write(w)
	res := w.Result()
	statusCode = res.StatusCode

	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return
}

// --- Constructor tests ---

func TestBadRequest(t *testing.T) {
	e := apierror.BadRequest("missing field")
	if e.HTTPStatus != http.StatusBadRequest {
		t.Errorf("HTTPStatus: got %d, want %d", e.HTTPStatus, http.StatusBadRequest)
	}
	if e.Code != apierror.CodeBadRequest {
		t.Errorf("Code: got %q, want %q", e.Code, apierror.CodeBadRequest)
	}
	if e.Message != "missing field" {
		t.Errorf("Message: got %q, want %q", e.Message, "missing field")
	}
}

func TestUnauthorized(t *testing.T) {
	e := apierror.Unauthorized("not authenticated")
	if e.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus: got %d, want %d", e.HTTPStatus, http.StatusUnauthorized)
	}
	if e.Code != apierror.CodeUnauthorized {
		t.Errorf("Code: got %q, want %q", e.Code, apierror.CodeUnauthorized)
	}
}

func TestForbidden(t *testing.T) {
	e := apierror.Forbidden("access denied")
	if e.HTTPStatus != http.StatusForbidden {
		t.Errorf("HTTPStatus: got %d, want %d", e.HTTPStatus, http.StatusForbidden)
	}
	if e.Code != apierror.CodeForbidden {
		t.Errorf("Code: got %q, want %q", e.Code, apierror.CodeForbidden)
	}
}

func TestForbiddenWithDetails(t *testing.T) {
	details := map[string]string{"required_scope": "payments:write"}
	e := apierror.ForbiddenWithDetails("insufficient scope", details)
	if e.HTTPStatus != http.StatusForbidden {
		t.Errorf("HTTPStatus: got %d, want %d", e.HTTPStatus, http.StatusForbidden)
	}
	if e.Code != apierror.CodeForbidden {
		t.Errorf("Code: got %q, want %q", e.Code, apierror.CodeForbidden)
	}
	if e.Details == nil {
		t.Error("Details should not be nil")
	}
}

func TestNotFound(t *testing.T) {
	e := apierror.NotFound("resource missing")
	if e.HTTPStatus != http.StatusNotFound {
		t.Errorf("HTTPStatus: got %d, want %d", e.HTTPStatus, http.StatusNotFound)
	}
	if e.Code != apierror.CodeNotFound {
		t.Errorf("Code: got %q, want %q", e.Code, apierror.CodeNotFound)
	}
}

func TestConflict(t *testing.T) {
	e := apierror.Conflict("already exists")
	if e.HTTPStatus != http.StatusConflict {
		t.Errorf("HTTPStatus: got %d, want %d", e.HTTPStatus, http.StatusConflict)
	}
	if e.Code != apierror.CodeConflict {
		t.Errorf("Code: got %q, want %q", e.Code, apierror.CodeConflict)
	}
}

func TestRateLimited(t *testing.T) {
	e := apierror.RateLimited("30")
	if e.HTTPStatus != http.StatusTooManyRequests {
		t.Errorf("HTTPStatus: got %d, want %d", e.HTTPStatus, http.StatusTooManyRequests)
	}
	if e.Code != apierror.CodeRateLimitExceeded {
		t.Errorf("Code: got %q, want %q", e.Code, apierror.CodeRateLimitExceeded)
	}
	// Message should embed the retry-after value.
	if e.Message == "" {
		t.Error("Message should not be empty")
	}
}

func TestValidationFailed(t *testing.T) {
	fields := map[string]string{"email": "invalid format", "amount": "must be positive"}
	e := apierror.ValidationFailed("validation error", fields)
	if e.HTTPStatus != http.StatusUnprocessableEntity {
		t.Errorf("HTTPStatus: got %d, want %d", e.HTTPStatus, http.StatusUnprocessableEntity)
	}
	if e.Code != apierror.CodeValidationFailed {
		t.Errorf("Code: got %q, want %q", e.Code, apierror.CodeValidationFailed)
	}
	if e.Details == nil {
		t.Error("Details should not be nil for ValidationFailed")
	}
}

func TestInternal(t *testing.T) {
	e := apierror.Internal("something went wrong")
	if e.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("HTTPStatus: got %d, want %d", e.HTTPStatus, http.StatusInternalServerError)
	}
	if e.Code != apierror.CodeInternalError {
		t.Errorf("Code: got %q, want %q", e.Code, apierror.CodeInternalError)
	}
}

func TestServiceUnavailable(t *testing.T) {
	e := apierror.ServiceUnavailable("downstream down")
	if e.HTTPStatus != http.StatusServiceUnavailable {
		t.Errorf("HTTPStatus: got %d, want %d", e.HTTPStatus, http.StatusServiceUnavailable)
	}
	if e.Code != apierror.CodeServiceUnavailable {
		t.Errorf("Code: got %q, want %q", e.Code, apierror.CodeServiceUnavailable)
	}
}

// --- Error() string interface ---

func TestErrorString(t *testing.T) {
	e := apierror.BadRequest("bad input")
	got := e.Error()
	want := "[BAD_REQUEST] bad input"
	if got != want {
		t.Errorf("Error(): got %q, want %q", got, want)
	}
}

// --- WithRequestID / WithTraceID chaining ---

func TestWithRequestID(t *testing.T) {
	e := apierror.Internal("oops").WithRequestID("req-abc-123")
	if e.RequestID != "req-abc-123" {
		t.Errorf("RequestID: got %q, want %q", e.RequestID, "req-abc-123")
	}
	// Chaining should return the same pointer (mutates in place).
	if e.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("HTTPStatus should be preserved after WithRequestID, got %d", e.HTTPStatus)
	}
}

func TestWithTraceID(t *testing.T) {
	e := apierror.Internal("oops").WithTraceID("trace-xyz-789")
	if e.TraceID != "trace-xyz-789" {
		t.Errorf("TraceID: got %q, want %q", e.TraceID, "trace-xyz-789")
	}
}

func TestChaining(t *testing.T) {
	e := apierror.Unauthorized("bad token").
		WithRequestID("req-1").
		WithTraceID("trace-1")

	if e.RequestID != "req-1" {
		t.Errorf("RequestID: got %q, want %q", e.RequestID, "req-1")
	}
	if e.TraceID != "trace-1" {
		t.Errorf("TraceID: got %q, want %q", e.TraceID, "trace-1")
	}
	if e.HTTPStatus != http.StatusUnauthorized {
		t.Errorf("HTTPStatus should be preserved after chaining, got %d", e.HTTPStatus)
	}
}

// --- Write() method: HTTP status, Content-Type, JSON envelope ---

func TestWrite_SetsStatusCode(t *testing.T) {
	tests := []struct {
		name     string
		err      *apierror.APIError
		wantCode int
	}{
		{"BadRequest", apierror.BadRequest("x"), http.StatusBadRequest},
		{"Unauthorized", apierror.Unauthorized("x"), http.StatusUnauthorized},
		{"Forbidden", apierror.Forbidden("x"), http.StatusForbidden},
		{"NotFound", apierror.NotFound("x"), http.StatusNotFound},
		{"Conflict", apierror.Conflict("x"), http.StatusConflict},
		{"RateLimited", apierror.RateLimited("60"), http.StatusTooManyRequests},
		{"ValidationFailed", apierror.ValidationFailed("x", nil), http.StatusUnprocessableEntity},
		{"Internal", apierror.Internal("x"), http.StatusInternalServerError},
		{"ServiceUnavailable", apierror.ServiceUnavailable("x"), http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			tt.err.Write(w)
			if w.Code != tt.wantCode {
				t.Errorf("Write() status: got %d, want %d", w.Code, tt.wantCode)
			}
		})
	}
}

func TestWrite_ContentType(t *testing.T) {
	w := httptest.NewRecorder()
	apierror.BadRequest("test").Write(w)
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
	}
}

func TestWrite_JSONEnvelope(t *testing.T) {
	statusCode, body := decodeResponse(t, apierror.BadRequest("invalid input"))

	if statusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", statusCode, http.StatusBadRequest)
	}
	if body.Error == nil {
		t.Fatal("response should have an 'error' key at the top level")
	}
	if body.Error.Code != "BAD_REQUEST" {
		t.Errorf("error.code: got %q, want %q", body.Error.Code, "BAD_REQUEST")
	}
	if body.Error.Message != "invalid input" {
		t.Errorf("error.message: got %q, want %q", body.Error.Message, "invalid input")
	}
}

func TestWrite_HTTPStatusNotInJSON(t *testing.T) {
	// HTTPStatus must NOT appear in the serialized JSON (it has json:"-").
	w := httptest.NewRecorder()
	apierror.Internal("oops").Write(w)

	var raw map[string]any
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, ok := raw["error"].(map[string]any)
	if !ok {
		t.Fatal("expected 'error' key in response")
	}
	if _, found := errObj["http_status"]; found {
		t.Error("http_status should NOT appear in the JSON response body")
	}
	if _, found := errObj["HTTPStatus"]; found {
		t.Error("HTTPStatus should NOT appear in the JSON response body")
	}
}

func TestWrite_RequestIDAndTraceIDInJSON(t *testing.T) {
	statusCode, body := decodeResponse(t,
		apierror.Internal("oops").WithRequestID("req-999").WithTraceID("trace-888"),
	)

	if statusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", statusCode, http.StatusInternalServerError)
	}
	if body.Error.RequestID != "req-999" {
		t.Errorf("error.request_id: got %q, want %q", body.Error.RequestID, "req-999")
	}
	if body.Error.TraceID != "trace-888" {
		t.Errorf("error.trace_id: got %q, want %q", body.Error.TraceID, "trace-888")
	}
}

func TestWrite_OmitsEmptyOptionalFields(t *testing.T) {
	// request_id and trace_id should be omitted when not set.
	w := httptest.NewRecorder()
	apierror.BadRequest("x").Write(w)

	var raw map[string]any
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, ok := raw["error"].(map[string]any)
	if !ok {
		t.Fatal("expected 'error' key in response")
	}
	if _, found := errObj["request_id"]; found {
		t.Error("request_id should be omitted when empty")
	}
	if _, found := errObj["trace_id"]; found {
		t.Error("trace_id should be omitted when empty")
	}
}

func TestWrite_DetailsInJSON(t *testing.T) {
	fields := map[string]string{"email": "invalid format"}
	_, body := decodeResponse(t, apierror.ValidationFailed("validation error", fields))

	if body.Error.Details == nil {
		t.Error("details should be present in ValidationFailed response")
	}
}

func TestWrite_NilDetailsOmitted(t *testing.T) {
	// details should be omitted when nil.
	w := httptest.NewRecorder()
	apierror.BadRequest("x").Write(w)

	var raw map[string]any
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, ok := raw["error"].(map[string]any)
	if !ok {
		t.Fatal("expected 'error' key in response")
	}
	if _, found := errObj["details"]; found {
		t.Error("details should be omitted when nil")
	}
}

// --- ForbiddenWithDetails: details surface in JSON ---

func TestForbiddenWithDetails_InJSON(t *testing.T) {
	details := map[string]string{"required_scope": "payments:write"}
	_, body := decodeResponse(t, apierror.ForbiddenWithDetails("insufficient scope", details))

	if body.Error.Code != "FORBIDDEN" {
		t.Errorf("code: got %q, want %q", body.Error.Code, "FORBIDDEN")
	}
	if body.Error.Details == nil {
		t.Error("details should be present in ForbiddenWithDetails response")
	}
}
