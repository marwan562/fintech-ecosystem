package api

import (
	"encoding/json"
	"net/http"

	"github.com/sapliy/sapliy-core/internal/flow/domain"
	"github.com/sapliy/sapliy-core/pkg/apierror"
	"github.com/sapliy/sapliy-core/pkg/jsonutil"
)

type FlowHandler struct {
	repo   domain.Repository
	runner *domain.FlowRunner
}

func NewFlowHandler(repo domain.Repository, runner *domain.FlowRunner) *FlowHandler {
	return &FlowHandler{repo: repo, runner: runner}
}

func (h *FlowHandler) CreateFlow(w http.ResponseWriter, r *http.Request) {
	var flow domain.Flow
	if err := json.NewDecoder(r.Body).Decode(&flow); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}
	if err := h.repo.CreateFlow(r.Context(), &flow); err != nil {
		apierror.Internal(err.Error()).Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusCreated, flow)
}

func (h *FlowHandler) GetFlow(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	flow, err := h.repo.GetFlow(r.Context(), id)
	if err != nil {
		apierror.Internal(err.Error()).Write(w)
		return
	}
	if flow == nil {
		apierror.NotFound("Flow not found").Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, flow)
}

func (h *FlowHandler) ListFlows(w http.ResponseWriter, r *http.Request) {
	zoneID := r.URL.Query().Get("zone_id")
	flows, err := h.repo.ListFlows(r.Context(), zoneID)
	if err != nil {
		apierror.Internal(err.Error()).Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, flows)
}
func (h *FlowHandler) UpdateFlow(w http.ResponseWriter, r *http.Request) {
	var flow domain.Flow
	if err := json.NewDecoder(r.Body).Decode(&flow); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}
	if err := h.repo.UpdateFlow(r.Context(), &flow); err != nil {
		apierror.Internal(err.Error()).Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, flow)
}

func (h *FlowHandler) GetExecution(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	exec, err := h.repo.GetExecution(r.Context(), id)
	if err != nil || exec == nil {
		apierror.NotFound("Execution not found").Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, exec)
}

func (h *FlowHandler) ResumeExecution(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExecutionID string                 `json:"execution_id"`
		Input       map[string]interface{} `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}
	// Simplified logic for migration
	jsonutil.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "resumed"})
}

func (h *FlowHandler) BulkUpdateFlows(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs     []string `json:"ids"`
		Enabled bool     `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	if err := h.repo.BulkUpdateFlowsEnabled(r.Context(), req.IDs, req.Enabled); err != nil {
		apierror.Internal(err.Error()).Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
