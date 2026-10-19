package openmcp

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"well-ambient/internal/decisioncommands"
	"well-ambient/internal/jiraquery"
	"well-ambient/internal/opennames"
	"well-ambient/internal/reviewread"
	"well-ambient/internal/runtimeknowledge"
)

const (
	ServerName    = "well-ambient"
	ServerVersion = "1.0.0"
)

var ToolNames = []string{
	"jira_describe_schema",
	"jira_search_issues",
	"jira_get_issue",
	"jira_aggregate_issues",
	"decision_get_context",
	"decision_prepare",
	"decision_execute",
	"decision_get_operation",
	"review_search",
	"review_get",
	"knowledge_describe",
	"knowledge_get_context",
	"knowledge_get_evidence",
}

type InvocationResult struct {
	Value      map[string]any
	TargetRefs []string
}

type Invoker interface {
	Invoke(ctx context.Context, toolName string, input json.RawMessage) (InvocationResult, error)
}

type EmptyInput struct{}

type OperationInput struct {
	OperationID string `json:"operation_id" jsonschema:"the persistent operation identifier"`
}

func NewServer(invoker Invoker, prefixes ...string) *mcp.Server {
	prefix := opennames.DefaultPrefix
	server := mcp.NewServer(&mcp.Implementation{
		Name: opennames.DefaultPrefix, Title: "well-ambient",
		Description: "Governed Jira analysis, board decisions, and code review reading.",
		Version:     ServerVersion,
	}, nil)
	addTool[EmptyInput](server, invoker, prefix, "knowledge_describe", "Describe authorized runtime knowledge sources, refresh state, scope and evidence limits. Use before assessing a requirement.")
	addTool[runtimeknowledge.ContextRequest](server, invoker, prefix, "knowledge_get_context", "Retrieve compressed requirement background from authorized Jira, published designs, documents, facts, reviews and registered capabilities, with source versions and gaps. Evidence does not prove deployed behavior.")
	addTool[runtimeknowledge.EvidenceRequest](server, invoker, prefix, "knowledge_get_evidence", "Expand one authorized evidence reference from a retained snapshot. Changed or withdrawn evidence is unavailable; source bodies remain bounded and untrusted.")
	addTool[EmptyInput](server, invoker, prefix, "jira_describe_schema", "Describe published Jira fields, dimensions, metrics, versions, and completeness.")
	addTool[jiraquery.SearchRequest](server, invoker, prefix, "jira_search_issues", "Search the complete authorized Jira issue projection with a stable cursor.")
	addTool[jiraquery.GetRequest](server, invoker, prefix, "jira_get_issue", "Read one authorized Jira issue and optionally page through its synchronized history.")
	addTool[jiraquery.AggregateRequest](server, invoker, prefix, "jira_aggregate_issues", "Aggregate the complete authorized Jira issue set using server-defined metrics.")
	addTool[decisioncommands.ContextRequest](server, invoker, prefix, "decision_get_context", "Read current Jira state and the decision actions allowed by active policy and execution bindings.")
	addTool[decisioncommands.PrepareRequest](server, invoker, prefix, "decision_prepare", "Freeze an exact single-issue decision plan with remote preconditions and expiry.")
	addTool[decisioncommands.ExecuteRequest](server, invoker, prefix, "decision_execute", "Execute an existing plan using only plan_id and idempotency_key.")
	addTool[OperationInput](server, invoker, prefix, "decision_get_operation", "Read queued, dispatching, confirmed, failed, partial, or unknown operation state.")
	addTool[reviewread.SearchRequest](server, invoker, prefix, "review_search", "Search published code review reports by repository, ref, SHA, state, and time.")
	addTool[reviewread.GetRequest](server, invoker, prefix, "review_get", "Read a version-specific review report with filtered findings and evidence references.")
	return server
}

func addTool[In any](server *mcp.Server, invoker Invoker, prefix, name, description string) {
	mcp.AddTool(server, &mcp.Tool{
		Name: opennames.Name(prefix, name), Title: opennames.Name(prefix, name), Description: description,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, map[string]any, error) {
		encoded, err := json.Marshal(input)
		if err != nil {
			return nil, nil, err
		}
		result, err := invoker.Invoke(ctx, name, encoded)
		if err != nil {
			return nil, nil, err
		}
		return nil, result.Value, nil
	})
}

func PublicToolNames(prefix string) []string {
	prefix = opennames.DefaultPrefix
	names := make([]string, len(ToolNames))
	for i, name := range ToolNames {
		names[i] = opennames.Name(prefix, name)
	}
	return names
}
