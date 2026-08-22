package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"well-ambient/internal/solutioncatalog"
)

func (s *Server) handleListSolutionCatalog(w http.ResponseWriter, r *http.Request) {
	scope, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	items, err := s.solutionCatalog.List(r.Context(), solutioncatalog.ListFilter{
		ProjectScope: scope,
		ProjectKey:   r.URL.Query().Get("project"),
		Search:       r.URL.Query().Get("q"),
		Limit:        queryInt(r, "limit"),
		Offset:       queryInt(r, "offset"),
	})
	if err != nil {
		writeSolutionCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleListSolutionCatalogProjects(w http.ResponseWriter, r *http.Request) {
	scope, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	projects, err := s.solutionCatalog.ListProjects(r.Context(), scope)
	if err != nil {
		writeSolutionCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": projects})
}

func (s *Server) handleGetSolutionCatalogEntry(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil || id == 0 {
		http.Error(w, "invalid catalog entry id", http.StatusBadRequest)
		return
	}
	scope, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	detail, err := s.solutionCatalog.Get(r.Context(), id, scope)
	if err != nil {
		writeSolutionCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleListSolutionStandards(w http.ResponseWriter, r *http.Request) {
	scope, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	standards, err := s.solutionCatalog.ListStandards(r.Context(), scope)
	if err != nil {
		writeSolutionCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": standards, "total": len(standards)})
}

func (s *Server) handleGetSolutionStandard(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil || id == 0 {
		http.Error(w, "invalid standard id", http.StatusBadRequest)
		return
	}
	scope, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	detail, err := s.solutionCatalog.GetStandard(r.Context(), id, scope)
	if err != nil {
		writeSolutionCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleReconcileSolutionCatalog(w http.ResponseWriter, r *http.Request) {
	queued, err := s.solutionCatalog.Reconcile(r.Context())
	if err != nil {
		writeSolutionCatalogError(w, err)
		return
	}
	processed, processErr := s.solutionCatalog.ProcessPending(r.Context(), 20)
	if processErr != nil {
		writeSolutionCatalogError(w, processErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"queued": queued, "processed": processed})
}

func (s *Server) handleReviewSolutionProposal(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint(r, "id")
	if err != nil || id == 0 {
		http.Error(w, "invalid proposal id", http.StatusBadRequest)
		return
	}
	var request struct {
		Action     string `json:"action"`
		ReviewNote string `json:"review_note"`
		StandardID uint   `json:"standard_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	scope, err := requestProjectPreferenceKeys(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply project preferences: %v", err), http.StatusInternalServerError)
		return
	}
	proposal, standard, err := s.solutionCatalog.ReviewProposal(r.Context(), solutioncatalog.ReviewProposalCommand{
		ProposalID: id, Action: request.Action, ReviewNote: request.ReviewNote,
		Actor: authenticatedActor(r), StandardID: request.StandardID, ProjectScope: scope,
	})
	if err != nil {
		writeSolutionCatalogError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"proposal": proposal, "standard": standard})
}

func queryInt(r *http.Request, key string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get(key)))
	return value
}

func writeSolutionCatalogError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, solutioncatalog.ErrNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, solutioncatalog.ErrConflict):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, solutioncatalog.ErrInvalid):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, solutioncatalog.ErrIntegrity):
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
