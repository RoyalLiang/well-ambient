package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"well-ambient/internal/decisioncommands"
	"well-ambient/internal/jiraquery"
	"well-ambient/internal/openaccess"
	"well-ambient/internal/openmcp"
	"well-ambient/internal/reviewread"
)

type openMCPLocalInvoker struct {
	server *Server
	call   openaccess.CallContext
}

func (s *Server) ensureOpenMCPHandler() {
	if s.openMCPHandler != nil {
		return
	}
	s.openMCPHandler = mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		call, _ := openaccess.CallContextFrom(request.Context())
		return openmcp.NewServer(openMCPLocalInvoker{server: s, call: call})
	}, &mcp.StreamableHTTPOptions{
		Stateless:                    true,
		JSONResponse:                 true,
		MaxRequestBodyBytes:          maxOpenRequestBody,
		PropagateRequestCancellation: true,
	})
}

func (s *Server) withOpenMCP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID, err := openaccess.RandomID("mcp_http")
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
		identity, err := s.openAccess.Authenticate(
			r.Context(),
			strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")),
		)
		if err != nil {
			writeOpenError(w, requestID, err)
			return
		}
		policy, err := s.openAccess.ActivePolicy(r.Context())
		if err != nil {
			writeOpenError(w, requestID, err)
			return
		}
		release, err := s.openLimiter.Acquire(r.Context(), identity)
		if err != nil {
			writeOpenError(w, requestID, err)
			return
		}
		defer release()
		call := openaccess.CallContext{
			Identity: identity, Policy: policy, RequestID: requestID, Transport: "mcp-http",
		}
		next.ServeHTTP(w, r.WithContext(openaccess.WithCallContext(r.Context(), call)))
	})
}

func (i openMCPLocalInvoker) Invoke(
	ctx context.Context,
	toolName string,
	input json.RawMessage,
) (result openmcp.InvocationResult, resultErr error) {
	call, ok := openaccess.CallContextFrom(ctx)
	if !ok {
		call = i.call
	}
	if call.Identity.SourceID == "" {
		return openmcp.InvocationResult{}, openaccess.NewError("unauthenticated", "MCP integration identity is unavailable")
	}
	requestID, err := openaccess.RandomID("mcp_call")
	if err != nil {
		return openmcp.InvocationResult{}, err
	}
	started := time.Now()
	defer func() {
		outcome := "succeeded"
		errorCode := ""
		if resultErr != nil {
			outcome = "failed"
			errorCode = openaccess.ErrorCode(resultErr)
		}
		_ = i.server.openAccess.RecordInvocation(ctx, openaccess.Invocation{
			RequestID: requestID, Identity: call.Identity, Transport: "mcp-http",
			ToolName: toolName, ToolVersion: openmcp.ServerVersion,
			PolicyVersion: call.Policy.Version, TargetRefs: result.TargetRefs,
			Duration: time.Since(started), Outcome: outcome, ErrorCode: errorCode,
		})
	}()
	if err := i.server.openFeatureError(toolName); err != nil {
		return result, err
	}

	switch toolName {
	case "jira_describe_schema":
		value, err := i.server.jiraQuery.DescribeSchema(ctx, call.Policy, requestID)
		return mcpResult(value, nil), publicMCPError(err)
	case "jira_search_issues":
		var request jiraquery.SearchRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return result, openaccess.NewError("invalid_query", "invalid jira_search_issues input")
		}
		value, err := i.server.jiraQuery.Search(ctx, call.Policy, requestID, request)
		return mcpResult(value, value.Meta.SourceRefs), publicMCPError(err)
	case "jira_get_issue":
		var request jiraquery.GetRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return result, openaccess.NewError("invalid_query", "invalid jira_get_issue input")
		}
		value, err := i.server.jiraQuery.Get(ctx, call.Policy, requestID, request)
		return mcpResult(value, value.Meta.SourceRefs), publicMCPError(err)
	case "jira_aggregate_issues":
		var request jiraquery.AggregateRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return result, openaccess.NewError("invalid_query", "invalid jira_aggregate_issues input")
		}
		value, err := i.server.jiraQuery.Aggregate(ctx, call.Policy, requestID, request)
		return mcpResult(value, nil), publicMCPError(err)
	case "decision_get_context":
		var request decisioncommands.ContextRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return result, openaccess.NewError("invalid_query", "invalid decision_get_context input")
		}
		value, err := i.server.decisionCommands.GetContext(ctx, call.Identity, call.Policy, request)
		return mcpResult(value, []string{request.IssueKey}), publicMCPError(err)
	case "decision_prepare":
		var request decisioncommands.PrepareRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return result, openaccess.NewError("invalid_query", "invalid decision_prepare input")
		}
		value, err := i.server.decisionCommands.Prepare(ctx, call.Identity, call.Policy, request)
		return mcpResult(value, []string{request.IssueKey}), publicMCPError(err)
	case "decision_execute":
		var request decisioncommands.ExecuteRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return result, openaccess.NewError("invalid_query", "invalid decision_execute input")
		}
		value, err := i.server.decisionCommands.Execute(ctx, call.Identity, call.Policy, request)
		return mcpResult(value, []string{request.PlanID}), publicMCPError(err)
	case "decision_get_operation":
		var request openmcp.OperationInput
		if err := json.Unmarshal(input, &request); err != nil {
			return result, openaccess.NewError("invalid_query", "invalid decision_get_operation input")
		}
		value, err := i.server.decisionCommands.GetOperation(ctx, call.Identity, request.OperationID)
		return mcpResult(value, []string{request.OperationID}), publicMCPError(err)
	case "review_search":
		var request reviewread.SearchRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return result, openaccess.NewError("invalid_query", "invalid review_search input")
		}
		value, err := i.server.reviewRead.Search(ctx, call.Policy, requestID, request)
		return mcpResult(value, value.Meta.SourceRefs), publicMCPError(err)
	case "review_get":
		var request reviewread.GetRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return result, openaccess.NewError("invalid_query", "invalid review_get input")
		}
		value, err := i.server.reviewRead.Get(ctx, call.Policy, requestID, request)
		return mcpResult(value, value.Meta.SourceRefs), publicMCPError(err)
	default:
		return result, openaccess.NewError("invalid_query", "unknown MCP tool")
	}
}

func mcpResult(value any, targets []string) openmcp.InvocationResult {
	encoded, _ := json.Marshal(value)
	var mapped map[string]any
	_ = json.Unmarshal(encoded, &mapped)
	return openmcp.InvocationResult{Value: mapped, TargetRefs: targets}
}

func publicMCPError(err error) error {
	if err == nil {
		return nil
	}
	var domainErr *openaccess.Error
	if errors.As(err, &domainErr) {
		return domainErr
	}
	return openaccess.NewError("internal_error", "open capability tool failed")
}
