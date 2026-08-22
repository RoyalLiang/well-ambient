package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"well-ambient/internal/config"
	"well-ambient/internal/readmodel"
)

func TestEveryGETAPIRouteDeclaresABoundedReadContract(t *testing.T) {
	source, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server routes: %v", err)
	}
	routePattern := regexp.MustCompile(`HandleFunc\("GET (/api/[^" ]+)"`)
	registered := map[string]struct{}{}
	for _, match := range routePattern.FindAllStringSubmatch(string(source), -1) {
		registered[match[1]] = struct{}{}
	}
	if len(registered) < 40 {
		t.Fatalf("route discovery found only %d GET API routes", len(registered))
	}

	contracts := allPageReadContracts()
	missing := make([]string, 0)
	for route := range registered {
		contract, ok := contracts[route]
		if !ok {
			missing = append(missing, route)
			continue
		}
		if err := contract.Validate(); err != nil {
			t.Errorf("read contract %s is not bounded: %v", route, err)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("GET API routes without read contracts:\n%s", strings.Join(missing, "\n"))
	}
	for route := range contracts {
		if _, ok := registered[route]; !ok {
			t.Errorf("stale read contract has no GET API route: %s", route)
		}
	}
}

func TestAllReachablePageStatesAreCoveredByReadContracts(t *testing.T) {
	expectedSurfaces := []string{
		"decision.agenda",
		"decision.daily_jira",
		"schedule.schedule",
		"schedule.releases",
		"schedule.board",
		"schedule.projects",
		"solutions.catalog",
		"evidence.health",
		"tasks.status",
		"tasks.execution",
		"kpi.overview",
		"kpi.calculation",
		"settings.gitlab",
		"settings.feishu",
		"settings.jira",
		"settings.performance",
		"settings.projects",
		"settings.ai",
		"settings.solution_prompts",
		"settings.ai_context",
		"settings.users",
		"settings.matrix",
		"settings.policies",
		"settings.audit",
		"shell",
	}
	covered := map[string]bool{}
	for _, contract := range allPageReadContracts() {
		for _, surface := range contract.Surfaces {
			covered[surface] = true
		}
	}
	for _, surface := range expectedSurfaces {
		if !covered[surface] {
			t.Errorf("page surface has no read contract coverage: %s", surface)
		}
	}
}

func TestKnownHighCardinalityRoutesCannotUseSingletonContracts(t *testing.T) {
	expected := map[string]readContractClass{
		"/api/tasks":                          readClassPage,
		"/api/execution/tasks":                readClassAggregate,
		"/api/logs":                           readClassTimeline,
		"/api/decision/daily-jira":            readClassPage,
		"/api/data-assets/events":             readClassTimeline,
		"/api/audit-logs":                     readClassTimeline,
		"/api/authz/audit-logs":               readClassTimeline,
		"/api/schedule":                       readClassAggregate,
		"/api/schedule/risk-calendar":         readClassAggregate,
		"/api/demand-specs":                   readClassPage,
		"/api/solution-catalog":               readClassPage,
		"/api/solution-standards":             readClassPage,
		"/api/execution/runs":                 readClassPage,
		"/api/corpus-candidates":              readClassPage,
		"/api/releases":                       readClassPage,
		"/api/work-items":                     readClassPage,
		"/api/performance/explanation":        readClassAggregate,
		"/api/kpi/performance":                readClassAggregate,
		"/api/kpi/report-preview":             readClassAggregate,
		"/api/strongest-brain/override-audit": readClassTimeline,
	}
	contracts := allPageReadContracts()
	for route, class := range expected {
		contract, ok := contracts[route]
		if !ok {
			t.Errorf("missing high-cardinality read contract: %s", route)
			continue
		}
		if contract.Class != class {
			t.Errorf("%s class = %s, want %s", route, contract.Class, class)
		}
	}
}

func TestReadContractMaturityCannotRegress(t *testing.T) {
	counts := map[readmodel.Maturity]int{}
	for _, contract := range allPageReadContracts() {
		counts[contract.Maturity]++
	}
	if counts[readmodel.MaturityVerified] < 17 {
		t.Fatalf("verified read contracts regressed to %d, want at least 17", counts[readmodel.MaturityVerified])
	}
	if counts[readmodel.MaturityVerified]+counts[readmodel.MaturityBounded] < 49 {
		t.Fatalf("bounded-or-verified read contracts regressed to %d, want at least 49", counts[readmodel.MaturityVerified]+counts[readmodel.MaturityBounded])
	}
	if counts[readmodel.MaturityPending] > 28 {
		t.Fatalf("migration-pending read contracts increased to %d, want at most 28", counts[readmodel.MaturityPending])
	}
}

func TestServerReadContractWrapperPublishesRouteMetrics(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	if server.handler == nil {
		t.Fatal("production server handler is not wrapped by the read registry")
	}
	first := httptest.NewRecorder()
	server.handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first status code = %d, body = %s", first.Code, first.Body.String())
	}
	if first.Header().Get("X-Well-Ambient-Read-Contract") != "system-status" {
		t.Fatalf("read contract header = %q", first.Header().Get("X-Well-Ambient-Read-Contract"))
	}

	second := httptest.NewRecorder()
	server.handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/api/status?read_contracts=full", nil))
	var payload struct {
		ReadContracts struct {
			Total            int                     `json:"total"`
			Verified         int                     `json:"verified"`
			Bounded          int                     `json:"bounded"`
			MigrationPending int                     `json:"migration_pending"`
			Declarations     []readmodel.Declaration `json:"declarations"`
		} `json:"read_contracts"`
		ReadPaths []struct {
			ContractID string `json:"contract_id"`
			Requests   uint64 `json:"requests"`
		} `json:"read_paths"`
	}
	if err := json.NewDecoder(second.Body).Decode(&payload); err != nil {
		t.Fatalf("decode status metrics: %v", err)
	}
	if len(payload.ReadPaths) != 1 || payload.ReadPaths[0].ContractID != "system-status" || payload.ReadPaths[0].Requests != 1 {
		t.Fatalf("unexpected read path metrics: %+v", payload.ReadPaths)
	}
	if payload.ReadContracts.Total != len(allPageReadContracts()) || len(payload.ReadContracts.Declarations) != payload.ReadContracts.Total {
		t.Fatalf("unexpected read contract inventory: %+v", payload.ReadContracts)
	}
	if payload.ReadContracts.Verified+payload.ReadContracts.Bounded+payload.ReadContracts.MigrationPending != payload.ReadContracts.Total {
		t.Fatalf("maturity inventory does not sum to total: %+v", payload.ReadContracts)
	}
}
