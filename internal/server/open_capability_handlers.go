package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/decisioncommands"
	"well-ambient/internal/jiraquery"
	"well-ambient/internal/openaccess"
	"well-ambient/internal/reviewread"
)

const maxOpenRequestBody = 1 << 20

type openCapabilityFeatures struct {
	Read    bool
	Prepare bool
	Execute bool
}

type openCall struct {
	Identity  openaccess.Identity
	Policy    openaccess.Policy
	RequestID string
}

type openResult struct {
	Status     int
	Body       any
	TargetRefs []string
}

type openHandler func(*http.Request, openCall) (openResult, error)

func (s *Server) ensureOpenCapabilities() {
	if s == nil || s.config == nil || db.DB == nil {
		return
	}
	if s.openAccess == nil {
		s.openAccess = openaccess.New(db.DB)
	}
	s.syncOpenFeaturesFromConfig()
	if s.openLimiter == nil {
		s.openLimiter = openaccess.NewLimiter(db.DB, nil)
	}
	if s.jiraQuery == nil {
		s.jiraQuery = jiraquery.New(db.DB)
	}
	if s.reviewRead == nil {
		s.reviewRead = reviewread.New(db.DB)
	}
	if s.decisionCommands == nil {
		s.decisionCommands = decisioncommands.New(db.DB, s.openAccess, openJiraDecisionAdapter{server: s})
	}
	s.ensureOpenMCPHandler()
}

func (s *Server) registerOpenCapabilityRoutes() {
	if s.openAccess == nil {
		return
	}
	s.mux.HandleFunc("GET /open/v1/jira/schema", s.withOpenCapability("jira_describe_schema", s.openJiraSchema))
	s.mux.HandleFunc("POST /open/v1/jira/issues/search", s.withOpenCapability("jira_search_issues", s.openJiraSearch))
	s.mux.HandleFunc("GET /open/v1/jira/issues/{key}", s.withOpenCapability("jira_get_issue", s.openJiraGet))
	s.mux.HandleFunc("POST /open/v1/jira/aggregations", s.withOpenCapability("jira_aggregate_issues", s.openJiraAggregate))
	s.mux.HandleFunc("POST /open/v1/decisions/context", s.withOpenCapability("decision_get_context", s.openDecisionContext))
	s.mux.HandleFunc("POST /open/v1/decisions/plans", s.withOpenCapability("decision_prepare", s.openDecisionPrepare))
	s.mux.HandleFunc("POST /open/v1/decisions/executions", s.withOpenCapability("decision_execute", s.openDecisionExecute))
	s.mux.HandleFunc("GET /open/v1/operations/{id}", s.withOpenCapability("decision_get_operation", s.openDecisionOperation))
	s.mux.HandleFunc("POST /open/v1/reviews/search", s.withOpenCapability("review_search", s.openReviewSearch))
	s.mux.HandleFunc("GET /open/v1/reviews/{id}", s.withOpenCapability("review_get", s.openReviewGet))
	if s.openMCPHandler != nil {
		s.mux.Handle("/mcp", s.withOpenMCP(s.openMCPHandler))
	}
	s.mux.HandleFunc("GET /open/v1/skills", s.handleGetOpenSkillsList)
	s.mux.HandleFunc("GET /open/v1/skills/{name}", s.handleGetOpenSkillInfo)
	s.mux.HandleFunc("GET /open/v1/skills/{name}/skill.md", s.handleGetOpenSkillDoc)
	s.mux.HandleFunc("GET /open/v1/skills/{name}/manifest", s.handleGetOpenSkillManifest)
	s.mux.HandleFunc("GET /open/v1/skills/{name}/archive", s.handleGetOpenSkillArchive)
	s.mux.HandleFunc("GET /open/v1/install.sh", s.handleGetOpenInstallScript)
}

func (s *Server) withOpenCapability(toolName string, next openHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID, err := openaccess.RandomID("req")
		if err != nil {
			writeOpenError(w, "", err)
			return
		}
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("Cache-Control", "no-store")
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeOpenError(w, requestID, openaccess.NewError("unauthenticated", "Bearer integration credential is required"))
			return
		}
		secret := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		identity, err := s.openAccess.Authenticate(r.Context(), secret)
		if err != nil {
			writeOpenError(w, requestID, err)
			return
		}
		policy, err := s.openAccess.ActivePolicy(r.Context())
		if err != nil {
			writeOpenError(w, requestID, err)
			return
		}
		started := time.Now()
		release, err := s.openLimiter.Acquire(r.Context(), identity)
		if err != nil {
			_ = s.openAccess.RecordInvocation(r.Context(), openaccess.Invocation{
				RequestID: requestID, Identity: identity, Transport: "http",
				ToolName: toolName, ToolVersion: "1", PolicyVersion: policy.Version,
				Duration: time.Since(started), Outcome: "failed",
				ErrorCode: openaccess.ErrorCode(err),
			})
			writeOpenError(w, requestID, err)
			return
		}
		defer release()

		call := openCall{Identity: identity, Policy: policy, RequestID: requestID}
		if featureErr := s.openFeatureError(toolName); featureErr != nil {
			_ = s.openAccess.RecordInvocation(r.Context(), openaccess.Invocation{
				RequestID: requestID, Identity: identity, Transport: "http",
				ToolName: toolName, ToolVersion: "1", PolicyVersion: policy.Version,
				Duration: time.Since(started), Outcome: "failed",
				ErrorCode: openaccess.ErrorCode(featureErr),
			})
			writeOpenError(w, requestID, featureErr)
			return
		}
		result, callErr := next(r, call)
		outcome := "succeeded"
		errorCode := ""
		if callErr != nil {
			outcome = "failed"
			errorCode = openaccess.ErrorCode(callErr)
		}
		_ = s.openAccess.RecordInvocation(r.Context(), openaccess.Invocation{
			RequestID: requestID, Identity: identity, Transport: "http",
			ToolName: toolName, ToolVersion: "1", PolicyVersion: policy.Version,
			TargetRefs: result.TargetRefs, Duration: time.Since(started),
			Outcome: outcome, ErrorCode: errorCode,
		})
		if callErr != nil {
			writeOpenError(w, requestID, callErr)
			return
		}
		if result.Status == 0 {
			result.Status = http.StatusOK
		}
		writeOpenJSON(w, result.Status, result.Body)
	}
}

func (s *Server) syncOpenFeaturesFromConfig() {
	if s == nil || s.config == nil {
		return
	}
	cfg := s.config.OpenCapabilities
	if cfg.Enabled {
		s.openFeatures = openCapabilityFeatures{
			Read:    cfg.ReadEnabled,
			Prepare: cfg.PrepareEnabled,
			Execute: cfg.ExecuteEnabled,
		}
		return
	}
	envFeatures := loadOpenCapabilityFeatures()
	if envFeatures.Read || envFeatures.Prepare || envFeatures.Execute {
		s.openFeatures = envFeatures
		return
	}
	s.openFeatures = openCapabilityFeatures{
		Read:    false,
		Prepare: false,
		Execute: false,
	}
}

func loadOpenCapabilityFeatures() openCapabilityFeatures {
	return openCapabilityFeatures{
		Read:    environmentEnabled("WELL_AMBIENT_OPEN_READ_ENABLED"),
		Prepare: environmentEnabled("WELL_AMBIENT_OPEN_PREPARE_ENABLED"),
		Execute: environmentEnabled("WELL_AMBIENT_OPEN_EXECUTE_ENABLED"),
	}
}

func environmentEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (s *Server) openFeatureError(toolName string) error {
	switch toolName {
	case "jira_describe_schema", "jira_search_issues", "jira_get_issue", "jira_aggregate_issues",
		"review_search", "review_get", "decision_get_context", "decision_get_operation":
		if !s.openFeatures.Read {
			return openaccess.NewError("forbidden", "open read capabilities are disabled")
		}
	case "decision_prepare":
		if !s.openFeatures.Prepare {
			return openaccess.NewError("forbidden", "decision prepare is disabled")
		}
	case "decision_execute":
		if !s.openFeatures.Execute {
			return openaccess.NewError("forbidden", "decision execute is disabled")
		}
	default:
		return openaccess.NewError("forbidden", "open capability is disabled")
	}
	return nil
}

func (s *Server) openJiraSchema(r *http.Request, call openCall) (openResult, error) {
	response, err := s.jiraQuery.DescribeSchema(r.Context(), call.Policy, call.RequestID)
	return openResult{Body: response}, err
}

func (s *Server) openJiraSearch(r *http.Request, call openCall) (openResult, error) {
	var request jiraquery.SearchRequest
	if err := decodeOpenJSON(r, &request); err != nil {
		return openResult{}, err
	}
	response, err := s.jiraQuery.Search(r.Context(), call.Policy, call.RequestID, request)
	return openResult{Body: response, TargetRefs: response.Meta.SourceRefs}, err
}

func (s *Server) openJiraGet(r *http.Request, call openCall) (openResult, error) {
	historyLimit, err := optionalPositiveInt(r.URL.Query().Get("history_limit"))
	if err != nil {
		return openResult{}, err
	}
	request := jiraquery.GetRequest{
		Key: r.PathValue("key"), Fields: splitCSV(r.URL.Query().Get("fields")),
		IncludeHistory: strings.EqualFold(r.URL.Query().Get("include_history"), "true"),
		HistoryCursor:  r.URL.Query().Get("history_cursor"), HistoryLimit: historyLimit,
	}
	response, err := s.jiraQuery.Get(r.Context(), call.Policy, call.RequestID, request)
	return openResult{Body: response, TargetRefs: response.Meta.SourceRefs}, err
}

func (s *Server) openJiraAggregate(r *http.Request, call openCall) (openResult, error) {
	var request jiraquery.AggregateRequest
	if err := decodeOpenJSON(r, &request); err != nil {
		return openResult{}, err
	}
	response, err := s.jiraQuery.Aggregate(r.Context(), call.Policy, call.RequestID, request)
	return openResult{Body: response}, err
}

func (s *Server) openDecisionContext(r *http.Request, call openCall) (openResult, error) {
	var request decisioncommands.ContextRequest
	if err := decodeOpenJSON(r, &request); err != nil {
		return openResult{}, err
	}
	response, err := s.decisionCommands.GetContext(r.Context(), call.Identity, call.Policy, request)
	return openResult{Body: response, TargetRefs: []string{request.IssueKey}}, err
}

func (s *Server) openDecisionPrepare(r *http.Request, call openCall) (openResult, error) {
	var request decisioncommands.PrepareRequest
	if err := decodeOpenJSON(r, &request); err != nil {
		return openResult{}, err
	}
	response, err := s.decisionCommands.Prepare(r.Context(), call.Identity, call.Policy, request)
	return openResult{Status: http.StatusCreated, Body: response, TargetRefs: []string{request.IssueKey}}, err
}

func (s *Server) openDecisionExecute(r *http.Request, call openCall) (openResult, error) {
	var request decisioncommands.ExecuteRequest
	if err := decodeOpenJSON(r, &request); err != nil {
		return openResult{}, err
	}
	response, err := s.decisionCommands.Execute(r.Context(), call.Identity, call.Policy, request)
	status := http.StatusAccepted
	if response.State == decisioncommands.StateSucceeded ||
		response.State == decisioncommands.StateFailed ||
		response.State == decisioncommands.StateUnknown ||
		response.State == decisioncommands.StatePartial {
		status = http.StatusOK
	}
	return openResult{Status: status, Body: response, TargetRefs: []string{request.PlanID}}, err
}

func (s *Server) openDecisionOperation(r *http.Request, call openCall) (openResult, error) {
	response, err := s.decisionCommands.GetOperation(r.Context(), call.Identity, r.PathValue("id"))
	return openResult{Body: response, TargetRefs: []string{r.PathValue("id")}}, err
}

func (s *Server) openReviewSearch(r *http.Request, call openCall) (openResult, error) {
	var request reviewread.SearchRequest
	if err := decodeOpenJSON(r, &request); err != nil {
		return openResult{}, err
	}
	response, err := s.reviewRead.Search(r.Context(), call.Policy, call.RequestID, request)
	return openResult{Body: response, TargetRefs: response.Meta.SourceRefs}, err
}

func (s *Server) openReviewGet(r *http.Request, call openCall) (openResult, error) {
	runID, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || runID == 0 {
		return openResult{}, openaccess.NewError("invalid_query", "review run id must be a positive integer")
	}
	offset, err := optionalNonNegativeInt(r.URL.Query().Get("offset"))
	if err != nil {
		return openResult{}, err
	}
	limit, err := optionalPositiveInt(r.URL.Query().Get("limit"))
	if err != nil {
		return openResult{}, err
	}
	request := reviewread.GetRequest{
		RunID: uint(runID), Severities: splitCSV(r.URL.Query().Get("severities")),
		Dimensions: splitCSV(r.URL.Query().Get("dimensions")), Files: splitCSV(r.URL.Query().Get("files")),
		CurrentHeadSHA: r.URL.Query().Get("current_head_sha"), Offset: offset, Limit: limit,
	}
	response, err := s.reviewRead.Get(r.Context(), call.Policy, call.RequestID, request)
	return openResult{Body: response, TargetRefs: response.Meta.SourceRefs}, err
}

func decodeOpenJSON(r *http.Request, target any) error {
	reader := http.MaxBytesReader(nil, r.Body, maxOpenRequestBody)
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return openaccess.NewError("invalid_query", "request body is required")
		}
		return openaccess.NewError("invalid_query", "request body is not valid JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return openaccess.NewError("invalid_query", "request body must contain one JSON object")
	}
	return nil
}

func optionalPositiveInt(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, openaccess.NewError("invalid_query", "numeric query parameter must be positive")
	}
	return value, nil
}

func optionalNonNegativeInt(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, openaccess.NewError("invalid_query", "numeric query parameter must be non-negative")
	}
	return value, nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	values := strings.Split(raw, ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func writeOpenError(w http.ResponseWriter, requestID string, err error) {
	code := openaccess.ErrorCode(err)
	status := http.StatusInternalServerError
	message := "open capability request failed"
	var domainErr *openaccess.Error
	if errors.As(err, &domainErr) {
		message = domainErr.Message
	}
	switch code {
	case "unauthenticated":
		status = http.StatusUnauthorized
	case "forbidden":
		status = http.StatusForbidden
	case "not_found":
		status = http.StatusNotFound
	case "invalid_query", "invalid_request":
		status = http.StatusBadRequest
	case "idempotency_conflict", "stale_plan":
		status = http.StatusConflict
	case "rate_limited":
		status = http.StatusTooManyRequests
	case "upstream_unavailable":
		status = http.StatusServiceUnavailable
	case "outcome_unknown":
		status = http.StatusAccepted
	}
	writeOpenJSON(w, status, map[string]any{
		"error": map[string]any{"code": code, "message": message, "request_id": requestID},
	})
}

func writeOpenJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeLegacyJiraSenderDisabled(w http.ResponseWriter) {
	writeOpenJSON(w, http.StatusConflict, map[string]any{
		"error": map[string]any{
			"code":    "legacy_sender_disabled",
			"message": "legacy Jira writes are disabled while open decision execute owns Jira changes",
		},
	})
}
