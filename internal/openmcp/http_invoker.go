package openmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"well-ambient/internal/jiraquery"
	"well-ambient/internal/openaccess"
	"well-ambient/internal/reviewread"
)

type HTTPInvoker struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewHTTPInvoker(baseURL, apiKey string, client *http.Client) (*HTTPInvoker, error) {
	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return nil, fmt.Errorf("WELL_AMBIENT_BASE_URL must be an absolute HTTP(S) origin")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return nil, fmt.Errorf("WELL_AMBIENT_BASE_URL must use HTTPS outside loopback development")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("WELL_AMBIENT_API_KEY is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPInvoker{
		baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, client: client,
	}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}

func (i *HTTPInvoker) Invoke(ctx context.Context, toolName string, input json.RawMessage) (InvocationResult, error) {
	switch toolName {
	case "jira_describe_schema":
		return i.get(ctx, "/open/v1/jira/schema")
	case "jira_search_issues":
		return i.post(ctx, "/open/v1/jira/issues/search", input)
	case "jira_get_issue":
		var request jiraquery.GetRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return InvocationResult{}, err
		}
		values := url.Values{}
		addCSV(values, "fields", request.Fields)
		if request.IncludeHistory {
			values.Set("include_history", "true")
		}
		if request.HistoryCursor != "" {
			values.Set("history_cursor", request.HistoryCursor)
		}
		if request.HistoryLimit > 0 {
			values.Set("history_limit", strconv.Itoa(request.HistoryLimit))
		}
		return i.get(ctx, "/open/v1/jira/issues/"+url.PathEscape(request.Key)+queryString(values))
	case "jira_aggregate_issues":
		return i.post(ctx, "/open/v1/jira/aggregations", input)
	case "decision_get_context":
		return i.post(ctx, "/open/v1/decisions/context", input)
	case "decision_prepare":
		return i.post(ctx, "/open/v1/decisions/plans", input)
	case "decision_execute":
		return i.post(ctx, "/open/v1/decisions/executions", input)
	case "decision_get_operation":
		var request OperationInput
		if err := json.Unmarshal(input, &request); err != nil {
			return InvocationResult{}, err
		}
		return i.get(ctx, "/open/v1/operations/"+url.PathEscape(request.OperationID))
	case "review_search":
		return i.post(ctx, "/open/v1/reviews/search", input)
	case "review_get":
		var request reviewread.GetRequest
		if err := json.Unmarshal(input, &request); err != nil {
			return InvocationResult{}, err
		}
		values := url.Values{}
		addCSV(values, "severities", request.Severities)
		addCSV(values, "dimensions", request.Dimensions)
		addCSV(values, "files", request.Files)
		if request.CurrentHeadSHA != "" {
			values.Set("current_head_sha", request.CurrentHeadSHA)
		}
		if request.Offset > 0 {
			values.Set("offset", strconv.Itoa(request.Offset))
		}
		if request.Limit > 0 {
			values.Set("limit", strconv.Itoa(request.Limit))
		}
		return i.get(ctx, "/open/v1/reviews/"+strconv.FormatUint(uint64(request.RunID), 10)+queryString(values))
	default:
		return InvocationResult{}, openaccess.NewError("invalid_query", "unknown MCP tool")
	}
}

func (i *HTTPInvoker) get(ctx context.Context, path string) (InvocationResult, error) {
	return i.request(ctx, http.MethodGet, path, nil)
}

func (i *HTTPInvoker) post(ctx context.Context, path string, body []byte) (InvocationResult, error) {
	return i.request(ctx, http.MethodPost, path, bytes.NewReader(body))
}

func (i *HTTPInvoker) request(ctx context.Context, method, path string, body io.Reader) (InvocationResult, error) {
	request, err := http.NewRequestWithContext(ctx, method, i.baseURL+path, body)
	if err != nil {
		return InvocationResult{}, err
	}
	request.Header.Set("Authorization", "Bearer "+i.apiKey)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := i.client.Do(request)
	if err != nil {
		return InvocationResult{}, openaccess.NewError("upstream_unavailable", "open capability HTTP endpoint is unavailable")
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return InvocationResult{}, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(payload, &envelope) == nil && envelope.Error.Code != "" {
			return InvocationResult{}, openaccess.NewError(envelope.Error.Code, envelope.Error.Message)
		}
		return InvocationResult{}, openaccess.NewError("upstream_unavailable", "open capability HTTP request failed")
	}
	var value map[string]any
	if err := json.Unmarshal(payload, &value); err != nil {
		return InvocationResult{}, fmt.Errorf("decode open capability response: %w", err)
	}
	return InvocationResult{Value: value}, nil
}

func addCSV(values url.Values, key string, items []string) {
	if len(items) > 0 {
		values.Set(key, strings.Join(items, ","))
	}
}

func queryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	return "?" + values.Encode()
}
