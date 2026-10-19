package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
	"well-ambient/internal/runtimeknowledge"
)

func (s *Server) startRuntimeKnowledgeWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		// Resolve the current repository mapping on each tick, as with external reads.
		if err := s.runtimeKnowledge().Tick(ctx); err != nil && ctx.Err() == nil {
			log.Print("Runtime knowledge refresh failed; previous generation retained")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) runtimeKnowledge() *runtimeknowledge.Service {
	if s.config == nil {
		return runtimeknowledge.New(db.DB)
	}
	return runtimeknowledge.New(db.DB, s.config.GitLab.Repos...)
}

func (s *Server) handleRuntimeKnowledgeSettings(w http.ResponseWriter, r *http.Request) {
	var value any
	var err error
	switch r.Method {
	case http.MethodPut:
		var req runtimeknowledge.SettingsRequest
		if err = decodeOpenJSON(r, &req); err == nil {
			value, err = s.runtimeKnowledge().Configure(r.Context(), req)
		}
	case http.MethodPost:
		value, err = s.runtimeKnowledge().Refresh(r.Context())
	default:
		value, err = s.runtimeKnowledge().Settings(r.Context())
	}
	if err != nil {
		status := http.StatusInternalServerError
		if openaccess.ErrorCode(err) == "invalid_query" {
			status = http.StatusBadRequest
		}
		if openaccess.ErrorCode(err) == "conflict" {
			status = http.StatusConflict
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}

func (s *Server) openKnowledgeDescribe(r *http.Request, call openCall) (openResult, error) {
	value, err := s.runtimeKnowledge().Describe(r.Context(), call.Policy)
	return openResult{Body: value}, err
}
func (s *Server) openKnowledgeContext(r *http.Request, call openCall) (openResult, error) {
	var req runtimeknowledge.ContextRequest
	if err := decodeOpenJSON(r, &req); err != nil {
		return openResult{}, err
	}
	value, err := s.runtimeKnowledge().Context(r.Context(), call.Policy, req)
	refs := make([]string, 0, len(value.Evidence))
	for _, e := range value.Evidence {
		refs = append(refs, e.SourceRef)
	}
	return openResult{Body: value, TargetRefs: refs}, err
}
func (s *Server) openKnowledgeEvidence(r *http.Request, call openCall) (openResult, error) {
	var req runtimeknowledge.EvidenceRequest
	if err := decodeOpenJSON(r, &req); err != nil {
		return openResult{}, err
	}
	value, err := s.runtimeKnowledge().Evidence(r.Context(), call.Policy, req)
	return openResult{Body: value, TargetRefs: []string{req.SourceRef}}, err
}
