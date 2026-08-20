package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sapliy/sapliy-core/internal/playbook"
	"github.com/sapliy/sapliy-core/pkg/apierror"
	"github.com/sapliy/sapliy-core/pkg/jsonutil"
)

// Engine and Log are the package-level singletons shared by all playbook
// handlers so decisions accumulate into one hash-chained audit trail.
var (
	Engine = playbook.NewEngine()
	Log    = playbook.NewDecisionLog()
)

// PlaybookCatalogEntry describes an available operational playbook with its
// default configuration.
type PlaybookCatalogEntry struct {
	Type          string `json:"type"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultConfig any    `json:"defaultConfig"`
}

// playbookCatalog returns the static MVP catalog using internal/playbook
// defaults.
func playbookCatalog() []PlaybookCatalogEntry {
	return []PlaybookCatalogEntry{
		{
			Type:          string(playbook.PlaybookRevenueRecovery),
			Name:          "Revenue Recovery & Dunning",
			Description:   "Recover failed subscription payments with smart retries and multi-channel dunning",
			DefaultConfig: playbook.DefaultDunningConfig(),
		},
		{
			Type:          string(playbook.PlaybookRefundApproval),
			Name:          "Refund Approval",
			Description:   "Route refunds and invoice adjustments through the policy engine for approval",
			DefaultConfig: playbook.DefaultRefundApprovalConfig(),
		},
		{
			Type:          string(playbook.PlaybookInvoiceReminders),
			Name:          "Invoice Reminders",
			Description:   "Send automated reminders for overdue invoices",
			DefaultConfig: playbook.DefaultInvoiceReminderConfig(),
		},
	}
}

// routePlaybooks dispatches the /v1/playbooks* endpoints.
func (h *GatewayHandler) routePlaybooks(w http.ResponseWriter, r *http.Request, p string) {
	switch {
	case p == "/playbooks" && r.Method == http.MethodGet:
		h.handlePlaybookCatalog(w, r)
	case p == "/playbooks/preview" && r.Method == http.MethodPost:
		h.handlePlaybookPreview(w, r)
	case p == "/playbooks/decisions" && r.Method == http.MethodGet:
		h.handlePlaybookDecisions(w, r)
	case p == "/playbooks/ingest" && r.Method == http.MethodPost:
		h.handlePlaybookIngest(w, r)
	default:
		apierror.NotFound("Not Found").Write(w)
	}
}

// handlePlaybookCatalog lists the available playbook types, descriptions and
// default configurations.
func (h *GatewayHandler) handlePlaybookCatalog(w http.ResponseWriter, _ *http.Request) {
	jsonutil.WriteJSON(w, http.StatusOK, map[string]any{
		"data": playbookCatalog(),
	})
}

// previewRequest is the body of POST /v1/playbooks/preview.
type previewRequest struct {
	Goal string `json:"goal"`
}

// previewResponse is the deterministic intent preview returned for a goal.
type previewResponse struct {
	PlaybookType     string                `json:"playbookType"`
	Summary          string                `json:"summary"`
	Steps            []playbook.IntentStep `json:"steps"`
	Confidence       float64               `json:"confidence"`
	RequiresApproval bool                  `json:"requiresApproval"`
}

// handlePlaybookPreview runs the deterministic intent parser over a
// natural-language goal. No LLM output is ever executed here.
func (h *GatewayHandler) handlePlaybookPreview(w http.ResponseWriter, r *http.Request) {
	var req previewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}
	if strings.TrimSpace(req.Goal) == "" {
		apierror.BadRequest("goal is required").Write(w)
		return
	}

	p := playbook.ParseIntent(req.Goal)
	jsonutil.WriteJSON(w, http.StatusOK, previewResponse{
		PlaybookType:     string(p.PlaybookType),
		Summary:          p.Summary,
		Steps:            p.Steps,
		Confidence:       p.Confidence,
		RequiresApproval: p.RequiresApproval,
	})
}

// handlePlaybookDecisions returns the entries of the shared DecisionLog.
func (h *GatewayHandler) handlePlaybookDecisions(w http.ResponseWriter, _ *http.Request) {
	jsonutil.WriteJSON(w, http.StatusOK, map[string]any{
		"data": Log.Entries(),
	})
}

// ingestRequest is the body of POST /v1/playbooks/ingest. Payload carries the
// raw provider webhook; Event is the provider event type and is applied to the
// payload when the payload does not already carry it.
type ingestRequest struct {
	Provider string         `json:"provider"`
	Event    string         `json:"event"`
	Payload  map[string]any `json:"payload"`
}

// handlePlaybookIngest normalizes a provider event, runs it through the
// playbook engine, and appends the resulting decisions to the shared
// DecisionLog.
func (h *GatewayHandler) handlePlaybookIngest(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}
	if req.Provider == "" {
		apierror.BadRequest("provider is required").Write(w)
		return
	}

	raw := req.Payload
	if raw == nil {
		raw = map[string]any{}
	}
	if req.Event != "" {
		raw["type"] = req.Event
	}

	ev, err := playbook.NormalizeProviderEvent(req.Provider, raw)
	if err != nil {
		apierror.BadRequest(err.Error()).Write(w)
		return
	}

	tenantID := r.Header.Get("X-Org-ID")
	if tenantID == "" {
		tenantID = "tenant_default"
	}

	entries, err := Engine.HandleEvent(ev, tenantID)
	if err != nil {
		h.logger.Error("Playbook engine rejected event", "error", err)
		apierror.BadRequest(err.Error()).Write(w)
		return
	}

	recorded := make([]playbook.DecisionEntry, 0, len(entries))
	for _, e := range entries {
		rec, err := Log.Append(e)
		if err != nil {
			h.logger.Error("Failed to append decision to log", "error", err)
			apierror.Internal("Failed to record decision").Write(w)
			return
		}
		recorded = append(recorded, rec)
	}

	h.logger.Info("Playbook event ingested", "provider", req.Provider, "event", ev.Type, "tenant", tenantID, "decisions", len(recorded))
	jsonutil.WriteJSON(w, http.StatusOK, map[string]any{
		"data": recorded,
	})
}
