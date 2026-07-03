package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sapliy/sapliy-core/internal/zone"
	"github.com/sapliy/sapliy-core/internal/zone/domain"
	"github.com/sapliy/sapliy-core/pkg/apierror"
	"github.com/sapliy/sapliy-core/pkg/jsonutil"
)

type ZoneHandler struct {
	service         *zone.Service
	templateService *zone.TemplateService
}

func NewZoneHandler(service *zone.Service, templateService *zone.TemplateService) *ZoneHandler {
	return &ZoneHandler{service: service, templateService: templateService}
}

func (h *ZoneHandler) CreateZone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrgID        string `json:"org_id"`
		Name         string `json:"name"`
		Mode         string `json:"mode"`
		TemplateName string `json:"template_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	mode := domain.Mode(req.Mode)
	z, err := h.service.CreateZone(r.Context(), domain.CreateZoneParams{
		OrgID:        req.OrgID,
		Name:         req.Name,
		Mode:         mode,
		TemplateName: req.TemplateName,
	})
	if err != nil {
		apierror.Internal(fmt.Sprintf("Failed to create zone: %v", err)).Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusCreated, z)
}

func (h *ZoneHandler) GetZone(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	z, err := h.service.GetZone(r.Context(), id)
	if err != nil || z == nil {
		apierror.NotFound("Zone not found").Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, z)
}

func (h *ZoneHandler) BulkUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ZoneIDs  []string          `json:"zone_ids"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}

	count, err := h.service.BulkUpdateMetadata(r.Context(), req.ZoneIDs, req.Metadata)
	if err != nil {
		apierror.Internal("Failed to update metadata").Write(w)
		return
	}

	jsonutil.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"updated_count": count,
	})
}

func (h *ZoneHandler) ListZones(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	zones, err := h.service.ListZones(r.Context(), orgID)
	if err != nil {
		apierror.Internal("Failed to list zones").Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, zones)
}

func (h *ZoneHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	templateType := r.URL.Query().Get("type")
	tmpl, err := h.templateService.Get(zone.TemplateType(templateType))
	if err != nil {
		apierror.NotFound("Template not found").Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, tmpl)
}

func (h *ZoneHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	tmpls := h.templateService.List()
	jsonutil.WriteJSON(w, http.StatusOK, tmpls)
}

func (h *ZoneHandler) ApplyTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ZoneID       string `json:"zone_id"`
		TemplateName string `json:"template_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest("Invalid request body").Write(w)
		return
	}
	result, err := h.templateService.Apply(r.Context(), req.ZoneID, zone.TemplateType(req.TemplateName))
	if err != nil {
		apierror.Internal(err.Error()).Write(w)
		return
	}
	jsonutil.WriteJSON(w, http.StatusOK, result)
}
