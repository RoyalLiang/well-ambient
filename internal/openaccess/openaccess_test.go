package openaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"well-ambient/internal/db"
)

func testService(t *testing.T) *Service {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "openaccess.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := conn.AutoMigrate(
		&db.IntegrationSource{},
		&db.IntegrationCredential{},
		&db.IntegrationQuotaWindow{},
		&db.IntegrationQuotaLease{},
		&db.IntegrationPolicyVersion{},
		&db.JiraExecutionBinding{},
		&db.CapabilityInvocation{},
	); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return New(conn)
}

func TestCredentialLifecycleAndRotation(t *testing.T) {
	ctx := context.Background()
	service := testService(t)
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	if _, err := service.CreateSource(ctx, "agent-host", "Agent Host", "AI Platform", "standard"); err != nil {
		t.Fatalf("create source: %v", err)
	}
	first, err := service.IssueCredential(ctx, "agent-host", 24*time.Hour)
	if err != nil {
		t.Fatalf("issue first credential: %v", err)
	}
	second, err := service.IssueCredential(ctx, "agent-host", 48*time.Hour)
	if err != nil {
		t.Fatalf("issue second credential: %v", err)
	}
	if first.Secret == second.Secret || first.Prefix == second.Prefix {
		t.Fatal("rotated credentials must be distinct")
	}

	for _, credential := range []IssuedCredential{first, second} {
		identity, authErr := service.Authenticate(ctx, credential.Secret)
		if authErr != nil {
			t.Fatalf("authenticate %s: %v", credential.KeyID, authErr)
		}
		if identity.SourceID != "agent-host" || identity.KeyID != credential.KeyID {
			t.Fatalf("unexpected identity: %+v", identity)
		}
	}

	var stored db.IntegrationCredential
	if err := service.db.First(&stored, "key_id = ?", first.KeyID).Error; err != nil {
		t.Fatalf("load stored credential: %v", err)
	}
	if stored.Verifier == "" || stored.Verifier == first.Secret {
		t.Fatal("credential plaintext was persisted")
	}

	if err := service.RevokeCredential(ctx, first.KeyID); err != nil {
		t.Fatalf("revoke first credential: %v", err)
	}
	if _, err := service.Authenticate(ctx, first.Secret); ErrorCode(err) != "unauthenticated" {
		t.Fatalf("revoked credential error = %v, want unauthenticated", err)
	}
	if _, err := service.Authenticate(ctx, second.Secret); err != nil {
		t.Fatalf("overlapping rotated credential should remain valid: %v", err)
	}

	if err := service.SetSourceStatus(ctx, "agent-host", SourceDisabled); err != nil {
		t.Fatalf("disable source: %v", err)
	}
	if _, err := service.Authenticate(ctx, second.Secret); ErrorCode(err) != "forbidden" {
		t.Fatalf("disabled source error = %v, want forbidden", err)
	}
}

func TestCredentialExpiryAndMalformedSecretsAreRejected(t *testing.T) {
	ctx := context.Background()
	service := testService(t)
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	if _, err := service.CreateSource(ctx, "system", "System", "", "standard"); err != nil {
		t.Fatal(err)
	}
	credential, err := service.IssueCredential(ctx, "system", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"", "Bearer " + credential.Secret, credential.Prefix + ".wrong"} {
		if _, err := service.Authenticate(ctx, secret); ErrorCode(err) != "unauthenticated" {
			t.Fatalf("malformed secret %q error = %v", secret, err)
		}
	}
	service.now = func() time.Time { return now.Add(2 * time.Hour) }
	if _, err := service.Authenticate(ctx, credential.Secret); ErrorCode(err) != "unauthenticated" {
		t.Fatalf("expired credential error = %v, want unauthenticated", err)
	}
}

func TestCredentialRequiresTTLAndSupportsDisableMetadataLifecycle(t *testing.T) {
	ctx := context.Background()
	service := testService(t)
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	if _, err := service.CreateSource(ctx, "managed", "Managed", "", "standard"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.IssueCredential(ctx, "managed", 0); ErrorCode(err) != "invalid_request" {
		t.Fatalf("zero ttl error = %v, want invalid_request", err)
	}
	credential, err := service.IssueCredential(ctx, "managed", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetCredentialStatus(ctx, credential.KeyID, CredentialDisabled); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, credential.Secret); ErrorCode(err) != "unauthenticated" {
		t.Fatalf("disabled credential error = %v", err)
	}
	if err := service.SetCredentialStatus(ctx, credential.KeyID, CredentialActive); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, credential.Secret); err != nil {
		t.Fatalf("re-enabled credential: %v", err)
	}
	listed, err := service.Credentials(ctx, "managed")
	if err != nil || len(listed) != 1 || listed[0].Verifier == "" {
		t.Fatalf("credential metadata = %+v, %v", listed, err)
	}
	encoded, _ := json.Marshal(listed[0])
	if strings.Contains(string(encoded), listed[0].Verifier) || strings.Contains(string(encoded), credential.Secret) {
		t.Fatalf("credential metadata exposed secret material: %s", encoded)
	}
	shown, err := service.Credential(ctx, credential.KeyID)
	if err != nil || shown.Prefix != credential.Prefix || shown.ExpiresAt == nil {
		t.Fatalf("credential show = %+v, %v", shown, err)
	}
}

func TestPolicyIsVersionedNormalizedAndFailClosed(t *testing.T) {
	ctx := context.Background()
	service := testService(t)
	if _, err := service.ActivePolicy(ctx); ErrorCode(err) != "forbidden" {
		t.Fatalf("missing policy error = %v, want forbidden", err)
	}

	policy, err := service.ActivatePolicy(ctx, PolicySpec{
		AllowedProjects:     []string{" wa ", "WA", "ns2"},
		AllowedRepositories: []string{"platform/api", "Platform/API"},
		Actions:             []string{"jira.read", "decision.reassign"},
		FieldRules: map[string][]string{
			"jira_search_issues": {"key", "summary", "assignee"},
		},
		QueryLimits: QueryLimits{MaxPageSize: 1000, MaxScanRows: -1},
	}, "admin")
	if err != nil {
		t.Fatalf("activate policy: %v", err)
	}
	if policy.Version != 1 || !policy.AllowsProject("wa") || !policy.AllowsProject("NS2") {
		t.Fatalf("unexpected normalized policy: %+v", policy)
	}
	if !policy.AllowsRepository("PLATFORM/API") || !policy.AllowsAction("JIRA.READ") {
		t.Fatalf("policy normalization failed: %+v", policy)
	}
	if !policy.AllowsFields("jira_search_issues", []string{"key", "summary"}) ||
		policy.AllowsFields("jira_search_issues", []string{"description"}) {
		t.Fatal("field publishing rule was not enforced")
	}
	if policy.QueryLimits.MaxPageSize != 100 || policy.QueryLimits.MaxScanRows != 10000 {
		t.Fatalf("query limits = %+v, want bounded defaults", policy.QueryLimits)
	}

	second, err := service.ActivatePolicy(ctx, PolicySpec{
		AllowedProjects: []string{"HIT"},
		Actions:         []string{"jira.read"},
	}, "admin")
	if err != nil {
		t.Fatalf("activate second policy: %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("second version = %d, want 2", second.Version)
	}
	active, err := service.ActivePolicy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if active.Version != 2 || active.AllowsProject("WA") || !active.AllowsProject("HIT") {
		t.Fatalf("active policy = %+v", active)
	}
}

func TestLimiterAggregatesBySourceAcrossRotatedKeys(t *testing.T) {
	ctx := context.Background()
	service := testService(t)
	if _, err := service.CreateSource(ctx, "shared-source", "Shared Source", "", "test"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	profiles := map[string]Quota{
		"test": {RequestsPerMinute: 2, MaxConcurrent: 1},
	}
	firstLimiter := NewLimiter(service.db, profiles)
	secondLimiter := NewLimiter(service.db, profiles)
	firstLimiter.now = func() time.Time { return now }
	secondLimiter.now = func() time.Time { return now }
	first := Identity{SourceID: "shared-source", KeyID: "one", QuotaProfile: "test"}
	second := Identity{SourceID: "shared-source", KeyID: "two", QuotaProfile: "test"}

	release, err := firstLimiter.Acquire(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := secondLimiter.Acquire(ctx, second); ErrorCode(err) != "rate_limited" {
		t.Fatalf("concurrent rotated key error = %v, want rate_limited", err)
	}
	release()
	releaseSecond, err := secondLimiter.Acquire(ctx, second)
	if err != nil {
		t.Fatalf("second request after release: %v", err)
	}
	releaseSecond()
	if _, err := firstLimiter.Acquire(ctx, first); ErrorCode(err) != "rate_limited" {
		t.Fatalf("per-minute source quota error = %v, want rate_limited", err)
	}

	now = now.Add(time.Minute)
	release, err = firstLimiter.Acquire(ctx, first)
	if err != nil {
		t.Fatalf("new window should allow requests: %v", err)
	}
	release()
}

func TestLimiterRecoversExpiredLeaseAcrossInstances(t *testing.T) {
	ctx := context.Background()
	service := testService(t)
	if _, err := service.CreateSource(ctx, "lease-source", "Lease Source", "", "test"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.September, 27, 10, 0, 0, 0, time.UTC)
	profiles := map[string]Quota{"test": {RequestsPerMinute: 10, MaxConcurrent: 1}}
	first := NewLimiter(service.db, profiles)
	second := NewLimiter(service.db, profiles)
	first.now = func() time.Time { return now }
	second.now = func() time.Time { return now }
	first.leaseDuration = 30 * time.Second
	second.leaseDuration = 30 * time.Second
	identity := Identity{SourceID: "lease-source", KeyID: "one", QuotaProfile: "test"}
	if _, err := first.Acquire(ctx, identity); err != nil {
		t.Fatal(err)
	}
	now = now.Add(31 * time.Second)
	release, err := second.Acquire(ctx, identity)
	if err != nil {
		t.Fatalf("expired lease was not recovered: %v", err)
	}
	release()
	var leases int64
	if err := service.db.Model(&db.IntegrationQuotaLease{}).Count(&leases).Error; err != nil {
		t.Fatal(err)
	}
	if leases != 0 {
		t.Fatalf("active quota leases = %d, want 0 after release", leases)
	}
}

func TestLimiterConcurrentInstancesGrantOneSharedSlot(t *testing.T) {
	ctx := context.Background()
	service := testService(t)
	if _, err := service.CreateSource(ctx, "concurrent-source", "Concurrent Source", "", "test"); err != nil {
		t.Fatal(err)
	}
	profiles := map[string]Quota{"test": {RequestsPerMinute: 10, MaxConcurrent: 1}}
	first := NewLimiter(service.db, profiles)
	second := NewLimiter(service.db, profiles)
	identity := Identity{SourceID: "concurrent-source", QuotaProfile: "test"}
	type result struct {
		release func()
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for _, limiter := range []*Limiter{first, second} {
		go func(limiter *Limiter) {
			<-start
			release, err := limiter.Acquire(ctx, identity)
			results <- result{release: release, err: err}
		}(limiter)
	}
	close(start)
	one, two := <-results, <-results
	successes, limited := 0, 0
	for _, outcome := range []result{one, two} {
		if outcome.err == nil {
			successes++
			outcome.release()
		} else if ErrorCode(outcome.err) == "rate_limited" {
			limited++
		} else {
			t.Fatalf("unexpected concurrent limiter error: %v", outcome.err)
		}
	}
	if successes != 1 || limited != 1 {
		t.Fatalf("concurrent limiter outcomes: successes=%d rate_limited=%d", successes, limited)
	}
}

func TestExecutionBindingIsExplicit(t *testing.T) {
	ctx := context.Background()
	service := testService(t)
	if _, err := service.ExecutionBinding(ctx, "WA", "reassign"); ErrorCode(err) != "forbidden" {
		t.Fatalf("missing binding error = %v, want forbidden", err)
	}
	binding := db.JiraExecutionBinding{
		ProjectRef: "wa", ActionClass: "ReAssign",
		ConnectorRef: "jira-primary", ExecutorRef: "jira-service", Status: SourceActive,
	}
	if err := service.UpsertExecutionBinding(ctx, binding); err != nil {
		t.Fatalf("upsert binding: %v", err)
	}
	loaded, err := service.ExecutionBinding(ctx, "WA", "reassign")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ProjectRef != "WA" || loaded.ActionClass != "reassign" {
		t.Fatalf("binding was not normalized: %+v", loaded)
	}
}

func TestErrorCodePreservesDomainErrors(t *testing.T) {
	if got := ErrorCode(fmt.Errorf("wrapped: %w", newError("stale_plan", "stale"))); got != "stale_plan" {
		t.Fatalf("wrapped error code = %q", got)
	}
	if got := ErrorCode(errors.New("plain")); got != "internal_error" {
		t.Fatalf("plain error code = %q", got)
	}
}
