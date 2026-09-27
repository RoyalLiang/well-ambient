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
	var source []byte
	for _, filename := range []string{"server.go", "open_capability_handlers.go"} {
		content, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("read server routes from %s: %v", filename, err)
		}
		source = append(source, content...)
	}
	routePattern := regexp.MustCompile(`HandleFunc\("GET (/(?:api|open/v1)/[^" ]+)"`)
	registered := map[string]struct{}{}
	for _, match := range routePattern.FindAllStringSubmatch(string(source), -1) {
		registered[match[1]] = struct{}{}
	}
	const expectedGETAPIRoutes = 102
	if len(registered) != expectedGETAPIRoutes {
		t.Fatalf("route discovery found %d GET API routes, want %d", len(registered), expectedGETAPIRoutes)
	}

	contracts := allPageReadContracts()
	if len(contracts) != expectedGETAPIRoutes {
		t.Fatalf("read contract inventory has %d routes, want %d", len(contracts), expectedGETAPIRoutes)
	}
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
	expectedSurfaces := map[string]struct{}{
		"shell":                 {},
		"decision.agenda":       {},
		"decision.daily_jira":   {},
		"schedule.schedule":     {},
		"schedule.releases":     {},
		"schedule.board":        {},
		"schedule.projects":     {},
		"solutions.catalog":     {},
		"evidence.health":       {},
		"tasks.status":          {},
		"tasks.execution":       {},
		"tasks.review":          {},
		"open.jira":             {},
		"open.decision":         {},
		"open.review":           {},
		"kpi.overview":          {},
		"kpi.calculation":       {},
		"ai_governance.skills":  {},
		"ai_governance.prompts": {},
		"ai_governance.rules":   {},
		"ai_governance.context": {},
		"settings.gitlab":       {},
		"settings.email":        {},
		"settings.feishu":       {},
		"settings.jira":         {},
		"settings.performance":  {},
		"settings.projects":     {},
		"settings.versions":     {},
		"settings.ai":           {},
		"settings.wellos":       {},
		"settings.users":        {},
		"settings.matrix":       {},
		"settings.policies":     {},
		"settings.audit":        {},
	}
	covered := map[string]struct{}{}
	for _, contract := range allPageReadContracts() {
		for _, surface := range contract.Surfaces {
			covered[surface] = struct{}{}
		}
	}
	if len(covered) != len(expectedSurfaces) {
		t.Errorf("reachable surface count = %d, want %d", len(covered), len(expectedSurfaces))
	}
	missing := make([]string, 0)
	for surface := range expectedSurfaces {
		if _, ok := covered[surface]; !ok {
			missing = append(missing, surface)
		}
	}
	unexpected := make([]string, 0)
	for surface := range covered {
		if _, ok := expectedSurfaces[surface]; !ok {
			unexpected = append(unexpected, surface)
		}
	}
	sort.Strings(missing)
	sort.Strings(unexpected)
	if len(missing) > 0 || len(unexpected) > 0 {
		t.Fatalf("reachable surface inventory mismatch; missing=%v unexpected=%v", missing, unexpected)
	}
}

func TestReadContractRouteOwnershipMatchesCurrentConsumers(t *testing.T) {
	expected := map[string][]string{
		"/api/agent-runtime/capabilities":       {"ai_governance.skills"},
		"/api/agent-runtime/capabilities/{id}":  {"ai_governance.skills"},
		"/api/agent-runtime/runs":               {"ai_governance.skills", "evidence.health"},
		"/api/agent-runtime/runs/{id}/lockfile": {"ai_governance.skills", "evidence.health"},
		"/api/agent-runtime/runs/{id}/trace":    {"ai_governance.skills", "evidence.health"},
		"/api/agent-runtime/replays/{id}":       {"ai_governance.skills", "evidence.health"},
		"/api/code-reviews/repos":               {"tasks.review", "ai_governance.rules"},
		"/api/solution-prompts":                 {"ai_governance.prompts"},
		"/api/solution-prompts/public-url":      {"ai_governance.prompts"},
		"/api/ai/context-readiness":             {"ai_governance.context"},
		"/api/ai/output-trace":                  {"ai_governance.context"},
		"/api/ai/requirement-clarification":     {"ai_governance.context"},
		"/api/ai/traces":                        {"ai_governance.context"},
		"/api/requirements/clarification":       {"ai_governance.context"},
		"/api/context/documents":                {"ai_governance.context"},
		"/api/context/documents/{id}":           {"ai_governance.context"},
		"/api/context/facts":                    {"ai_governance.context"},
		"/api/corpus-candidates":                {"ai_governance.context"},
		"/api/corpus-candidates/{id}/impact":    {"ai_governance.context"},
		"/api/context/pack/replay":              {"ai_governance.context", "evidence.health"},
		"/api/context/packs/{id}/replay":        {"ai_governance.context", "evidence.health"},
		"/api/strongest-brain/ai-traces":        {"evidence.health", "ai_governance.context"},
		"/api/config":                           {"shell", "settings.gitlab", "settings.email", "settings.feishu", "settings.jira", "settings.performance", "settings.projects", "settings.ai"},
		"/api/config/wellos-maintenance":        {"settings.wellos"},
		"/api/config/versions":                  {"settings.gitlab", "settings.email", "settings.feishu", "settings.jira", "settings.performance", "settings.projects", "settings.ai", "settings.wellos", "settings.versions", "settings.audit"},
	}
	contracts := allPageReadContracts()
	for route, want := range expected {
		contract, ok := contracts[route]
		if !ok {
			t.Errorf("missing read contract for ownership assertion: %s", route)
			continue
		}
		got := append([]string(nil), contract.Surfaces...)
		want = append([]string(nil), want...)
		sort.Strings(got)
		sort.Strings(want)
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Errorf("%s surfaces = %v, want %v", route, contract.Surfaces, expected[route])
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

func TestReadContractMaturityCountsMatchInventory(t *testing.T) {
	counts := map[readmodel.Maturity]int{}
	for _, contract := range allPageReadContracts() {
		counts[contract.Maturity]++
	}
	expected := map[readmodel.Maturity]int{
		readmodel.MaturityVerified: 20,
		readmodel.MaturityBounded:  54,
		readmodel.MaturityPending:  28,
	}
	for maturity, want := range expected {
		if got := counts[maturity]; got != want {
			t.Errorf("%s read contracts = %d, want %d", maturity, got, want)
		}
	}
	if total := counts[readmodel.MaturityVerified] + counts[readmodel.MaturityBounded] + counts[readmodel.MaturityPending]; total != 102 {
		t.Fatalf("maturity inventory totals %d routes, want 102", total)
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
