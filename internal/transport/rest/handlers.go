package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

type handlers struct {
	mnemos *core.Mnemos
}

func (h *handlers) storeMemory(w http.ResponseWriter, r *http.Request) {
	var req domain.StoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	result, err := h.mnemos.Store(r.Context(), &req)
	if err != nil {
		handleDomainError(w, err)
		return
	}

	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
	}
	writeJSON(w, status, result)
}

func (h *handlers) getMemory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	mem, err := h.mnemos.Get(r.Context(), id)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mem)
}

func (h *handlers) updateMemory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	var req domain.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	req.ID = id

	mem, err := h.mnemos.Update(r.Context(), &req)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mem)
}

func (h *handlers) deleteMemory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	hard := r.URL.Query().Get("hard") == "true"
	var err error
	if hard {
		err = h.mnemos.HardDelete(r.Context(), id)
	} else {
		err = h.mnemos.Delete(r.Context(), id)
	}
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

func (h *handlers) listMemories(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(q.Get("offset"))

	memories, err := h.mnemos.List(r.Context(), storage.ListQuery{
		ProjectID: q.Get("project_id"),
		Limit:     limit,
		Offset:    offset,
		SortBy:    "created_at",
		SortDesc:  true,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func (h *handlers) searchMemories(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query     string `json:"query"`
		ProjectID string `json:"project_id"`
		Limit     int    `json:"limit"`
		Mode      string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Query == "" {
		writeError(w, http.StatusBadRequest, "query required")
		return
	}
	if body.Limit <= 0 {
		body.Limit = 10
	}

	var results []*storage.SearchResult
	var err error

	switch strings.ToLower(body.Mode) {
	case "text":
		results, err = h.mnemos.TextSearch(r.Context(), storage.TextSearchQuery{
			Query:     body.Query,
			ProjectID: body.ProjectID,
			Limit:     body.Limit,
		})
	default:
		results, err = h.mnemos.Search(r.Context(), body.Query, body.ProjectID, body.Limit)
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *handlers) relateMemory(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("id")
	if sourceID == "" {
		writeError(w, http.StatusBadRequest, "source id required")
		return
	}

	var req domain.RelateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	req.SourceID = sourceID

	rel, err := h.mnemos.Relate(r.Context(), &req)
	if err != nil {
		handleDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, rel)
}

func (h *handlers) getStats(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	stats, err := h.mnemos.Stats(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *handlers) maintain(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProjectID string `json:"project_id"`
	}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck

	if err := h.mnemos.Maintain(r.Context(), body.ProjectID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleDomainError(w http.ResponseWriter, err error) {
	var notFound *domain.NotFoundError
	var validation *domain.ValidationErrors
	var validationSingle *domain.ValidationError

	switch {
	case errors.As(err, &notFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.As(err, &validation):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.As(err, &validationSingle):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrDuplicate):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
