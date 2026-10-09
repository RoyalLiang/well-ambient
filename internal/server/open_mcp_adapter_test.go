package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

func TestOpenMCPAuditCanBeLocatedByTransportHeader(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		name := "completed"
		if cancelled {
			name = "cancelled"
		}
		t.Run(name, func(t *testing.T) {
			srv, credential, _ := setupOpenCapabilityServer(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelled {
				const callbackName = "test_open_mcp_cancel_after_admission"
				if err := db.DB.Callback().Create().After("gorm:create").Register(callbackName, func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "capability_invocations" {
						cancel()
					}
				}); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { db.DB.Callback().Create().Remove(callbackName) })
			}
			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"audit-test","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}},"name":"jira_describe_schema","arguments":{}}}`)).WithContext(ctx)
			request.Header.Set("Authorization", "Bearer "+credential.Secret)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Accept", "application/json, text/event-stream")
			request.Header.Set("MCP-Protocol-Version", "2026-07-28")
			request.Header.Set("Mcp-Method", "tools/call")
			request.Header.Set("Mcp-Name", "jira_describe_schema")
			recorder := httptest.NewRecorder()
			srv.handler.ServeHTTP(recorder, request)
			transportID := recorder.Header().Get("X-Request-ID")
			if !strings.HasPrefix(transportID, "mcp_http_") {
				t.Fatalf("MCP transport ID = %q, response=%s", transportID, recorder.Body.String())
			}
			var invocations []db.CapabilityInvocation
			if err := db.DB.Where("substr(request_id, 1, ?) = ?", len(transportID)+1, transportID+"_").Find(&invocations).Error; err != nil {
				t.Fatal(err)
			}
			if len(invocations) != 1 {
				t.Fatalf("cannot locate tool audit by header %q: %+v", transportID, invocations)
			}
			invocation := invocations[0]
			if len(invocation.RequestID) > 64 || invocation.RequestID == transportID || invocation.Outcome == "started" {
				t.Fatalf("MCP correlated completion audit = %+v", invocation)
			}
			if cancelled && ctx.Err() == nil {
				t.Fatal("cancellation fixture never cancelled the actual MCP request")
			}
			if cancelled && invocation.Outcome != "failed" {
				t.Fatalf("MCP request cancellation did not reach the business handler: %+v", invocation)
			}
		})
	}
}

func TestOpenMCPMultipleCallsKeepIndependentCorrelatedIDs(t *testing.T) {
	srv, credential, policy := setupOpenCapabilityServer(t)
	transportID, err := openaccess.RandomID("mcp_http")
	if err != nil {
		t.Fatal(err)
	}
	invoker := openMCPLocalInvoker{server: srv, call: openaccess.CallContext{
		RequestID: transportID,
		Identity:  openaccess.Identity{SourceID: "trusted-agent", KeyID: credential.KeyID},
		Policy:    policy,
	}}
	for range 2 {
		if _, err := invoker.Invoke(context.Background(), "jira_describe_schema", nil); err != nil {
			t.Fatal(err)
		}
	}
	var invocations []db.CapabilityInvocation
	if err := db.DB.Where("substr(request_id, 1, ?) = ?", len(transportID)+1, transportID+"_").Find(&invocations).Error; err != nil {
		t.Fatal(err)
	}
	if len(invocations) != 2 || invocations[0].RequestID == invocations[1].RequestID {
		t.Fatalf("calls lost transport correlation or independent identities: %+v", invocations)
	}
	for _, invocation := range invocations {
		if len(invocation.RequestID) != len(transportID)+25 || len(invocation.RequestID) > 64 || invocation.Outcome != "succeeded" {
			t.Fatalf("correlated tool ID format/outcome = %+v", invocation)
		}
	}
}

func TestOpenMCPDirectInvokerWithoutTransportIDKeepsUniqueFallback(t *testing.T) {
	srv, credential, policy := setupOpenCapabilityServer(t)
	invoker := openMCPLocalInvoker{server: srv, call: openaccess.CallContext{
		Identity: openaccess.Identity{SourceID: "trusted-agent", KeyID: credential.KeyID},
		Policy:   policy,
	}}
	for range 2 {
		if _, err := invoker.Invoke(context.Background(), "jira_describe_schema", nil); err != nil {
			t.Fatal(err)
		}
	}
	var invocations []db.CapabilityInvocation
	if err := db.DB.Find(&invocations).Error; err != nil {
		t.Fatal(err)
	}
	if len(invocations) != 2 || invocations[0].RequestID == invocations[1].RequestID {
		t.Fatalf("direct calls did not retain independent IDs: %+v", invocations)
	}
	for _, invocation := range invocations {
		if !strings.HasPrefix(invocation.RequestID, "mcp_call_") || len(invocation.RequestID) > 64 {
			t.Fatalf("direct call fallback ID = %q", invocation.RequestID)
		}
	}
}

func TestOpenMCPRejectsOversizedTransportTraceBeforeExecution(t *testing.T) {
	srv, credential, policy := setupOpenCapabilityServer(t)
	invoker := openMCPLocalInvoker{server: srv, call: openaccess.CallContext{
		RequestID: strings.Repeat("x", 64),
		Identity:  openaccess.Identity{SourceID: "trusted-agent", KeyID: credential.KeyID},
		Policy:    policy,
	}}
	result, err := invoker.Invoke(context.Background(), "jira_describe_schema", nil)
	if openaccess.ErrorCode(err) != "internal_error" || result.Value != nil {
		t.Fatalf("oversized invocation trace returned result=%v err=%v", result.Value, err)
	}
	var count int64
	if err := db.DB.Model(&db.CapabilityInvocation{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("oversized trace reached invocation storage: %d rows", count)
	}
}

func TestOpenMCPInvocationAuditFailurePreventsExecution(t *testing.T) {
	srv, credential, policy := setupOpenCapabilityServer(t)
	const callbackName = "test_open_mcp_audit_failure"
	if err := db.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "capability_invocations" {
			tx.AddError(errors.New("audit storage unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.DB.Callback().Create().Remove(callbackName) })
	invoker := openMCPLocalInvoker{server: srv, call: openaccess.CallContext{
		Identity: openaccess.Identity{SourceID: "trusted-agent", KeyID: credential.KeyID},
		Policy:   policy,
	}}
	result, err := invoker.Invoke(context.Background(), "jira_describe_schema", nil)
	if openaccess.ErrorCode(err) != "audit_unavailable" || result.Value != nil {
		t.Fatalf("MCP invocation without audit returned value=%v err=%v", result.Value, err)
	}
	if !strings.Contains(err.Error(), "business operation was not started") {
		t.Fatalf("MCP audit admission error = %v", err)
	}
}

func TestOpenMCPInvocationCompletionFailureRequiresReconciliation(t *testing.T) {
	srv, credential, policy := setupOpenCapabilityServer(t)
	const callbackName = "test_open_mcp_completion_failure"
	creates := 0
	if err := db.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "capability_invocations" {
			creates++
			if creates == 2 {
				tx.AddError(errors.New("audit completion unavailable"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.DB.Callback().Create().Remove(callbackName) })
	invoker := openMCPLocalInvoker{server: srv, call: openaccess.CallContext{
		Identity: openaccess.Identity{SourceID: "trusted-agent", KeyID: credential.KeyID},
		Policy:   policy,
	}}
	result, err := invoker.Invoke(context.Background(), "jira_describe_schema", nil)
	if openaccess.ErrorCode(err) != "audit_unavailable" || result.Value != nil {
		t.Fatalf("MCP completion failure returned value=%v err=%v", result.Value, err)
	}
	var invocation db.CapabilityInvocation
	if queryErr := db.DB.Where("transport = ?", "mcp-http").First(&invocation).Error; queryErr != nil {
		t.Fatal(queryErr)
	}
	if invocation.Outcome != "started" || creates != 2 {
		t.Fatalf("MCP completion evidence = %+v, creates=%d", invocation, creates)
	}
	for _, text := range []string{"business result may already have occurred", invocation.RequestID, "do not retry blindly"} {
		if !strings.Contains(err.Error(), text) {
			t.Fatalf("MCP completion error missing %q: %v", text, err)
		}
	}
}

func TestOpenMCPInvocationAuditSurvivesCancellation(t *testing.T) {
	srv, credential, policy := setupOpenCapabilityServer(t)
	invoker := openMCPLocalInvoker{server: srv, call: openaccess.CallContext{
		Identity: openaccess.Identity{SourceID: "trusted-agent", KeyID: credential.KeyID},
		Policy:   policy,
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, resultErr := invoker.Invoke(ctx, "jira_describe_schema", nil)
	var invocation db.CapabilityInvocation
	if err := db.DB.Where("transport = ?", "mcp-http").First(&invocation).Error; err != nil {
		t.Fatal(err)
	}
	wantOutcome := "succeeded"
	if resultErr != nil {
		wantOutcome = "failed"
	}
	if invocation.Outcome != wantOutcome {
		t.Fatalf("canceled MCP invocation audit = %+v, err=%v", invocation, resultErr)
	}
}

func TestOpenMCPInvocationPanicIsNeverAuditedAsSuccess(t *testing.T) {
	srv, credential, policy := setupOpenCapabilityServer(t)
	srv.jiraQuery = nil // Force a business-handler panic without any external work.
	invoker := openMCPLocalInvoker{server: srv, call: openaccess.CallContext{
		Identity: openaccess.Identity{SourceID: "trusted-agent", KeyID: credential.KeyID},
		Policy:   policy,
	}}
	var panicValue any
	func() {
		defer func() { panicValue = recover() }()
		_, _ = invoker.Invoke(context.Background(), "jira_describe_schema", nil)
	}()
	if panicValue == nil {
		t.Fatal("business-handler panic was swallowed")
	}
	var invocation db.CapabilityInvocation
	if err := db.DB.Where("transport = ?", "mcp-http").First(&invocation).Error; err != nil {
		t.Fatal(err)
	}
	if invocation.Outcome != "failed" || invocation.ErrorCode != "internal_error" {
		t.Fatalf("panicking MCP invocation audit = %+v", invocation)
	}
}
