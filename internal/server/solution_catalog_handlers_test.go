package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/solutioncatalog"
	"well-ambient/internal/solutions"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSolutionCatalogHandlersApplyProjectScopeBeforeListAndDetail(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open("file:solution_catalog_handler_scope?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.AutoMigrate(
		&db.TaskTelemetry{}, &db.UserProjectPreference{},
		&db.SolutionAsset{}, &db.SolutionRevision{}, &db.SolutionSourceRef{}, &db.SolutionPolishJob{},
		&db.SolutionPromptTemplate{}, &db.SolutionJiraOutbox{}, &db.SolutionCatalogEntry{},
		&db.SolutionCatalogSearchToken{}, &db.SolutionCatalogSyncJob{}, &db.SolutionComparison{},
		&db.SolutionStandardizationProposal{}, &db.SolutionStandard{}, &db.SolutionStandardRevision{},
	); err != nil {
		t.Fatal(err)
	}
	db.DB = conn
	now := time.Date(2026, 8, 12, 15, 0, 0, 0, time.UTC)
	solutionModule := solutions.New(conn, solutions.WithClock(func() time.Time { return now }))
	catalogModule := solutioncatalog.New(conn, solutioncatalog.Settings{RecallThreshold: 0.10}, solutioncatalog.WithClock(func() time.Time { return now }))
	server := NewServer(&config.Config{}, "")
	server.solutions = solutionModule
	server.solutionCatalog = catalogModule

	for _, fixture := range []struct {
		demandID, projectKey, title string
	}{{"WA-501", "WA", "统一登录治理"}, {"DG-502", "DG", "统一登录现场治理"}} {
		if err := conn.Create(&db.TaskTelemetry{
			TaskID: fixture.demandID, ProjectKey: fixture.projectKey, Title: fixture.title,
			IssueType: "requirement", Status: "progress", TaskCreatedAt: now.Add(-time.Hour), LastUpdate: now,
		}).Error; err != nil {
			t.Fatal(err)
		}
		workspace, err := solutionModule.EnsureDraft(context.Background(), solutions.EnsureDraftCommand{
			DemandID: fixture.demandID, Title: fixture.title, Markdown: "# " + fixture.title + "\n\n统一登录与权限回退。", Actor: "owner",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := solutionModule.Publish(context.Background(), solutions.PublishCommand{
			DemandID: fixture.demandID, ExpectedRevision: workspace.Asset.Revision, Actor: "owner",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := catalogModule.ProcessPending(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceUserProjectPreferences(conn, "alice@example.com", []string{"WA"}); err != nil {
		t.Fatal(err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/solution-catalog?q=统一登录", nil)
	listRequest.Header.Set("x-authenticated-user-id", "alice@example.com")
	listRecorder := httptest.NewRecorder()
	server.handleListSolutionCatalog(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var list solutioncatalog.CatalogList
	if err := json.NewDecoder(listRecorder.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].ProjectKey != "WA" {
		t.Fatalf("scoped catalog list = %+v", list)
	}

	var hidden db.SolutionCatalogEntry
	if err := conn.Where("project_key = ?", "DG").First(&hidden).Error; err != nil {
		t.Fatal(err)
	}
	detailRequest := httptest.NewRequest(http.MethodGet, "/api/solution-catalog/"+strconv.FormatUint(uint64(hidden.ID), 10), nil)
	detailRequest.SetPathValue("id", strconv.FormatUint(uint64(hidden.ID), 10))
	detailRequest.Header.Set("x-authenticated-user-id", "alice@example.com")
	detailRecorder := httptest.NewRecorder()
	server.handleGetSolutionCatalogEntry(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusNotFound {
		t.Fatalf("hidden detail status=%d body=%s", detailRecorder.Code, detailRecorder.Body.String())
	}

	projectRequest := httptest.NewRequest(http.MethodGet, "/api/solution-catalog/projects", nil)
	projectRequest.Header.Set("x-authenticated-user-id", "alice@example.com")
	projectRecorder := httptest.NewRecorder()
	server.handleListSolutionCatalogProjects(projectRecorder, projectRequest)
	if projectRecorder.Code != http.StatusOK || string(projectRecorder.Body.Bytes()) == "" {
		t.Fatalf("project response status=%d body=%s", projectRecorder.Code, projectRecorder.Body.String())
	}
	var projects struct {
		Items []solutioncatalog.CatalogProject `json:"items"`
	}
	if err := json.NewDecoder(projectRecorder.Body).Decode(&projects); err != nil {
		t.Fatal(err)
	}
	if len(projects.Items) != 1 || projects.Items[0].ProjectKey != "WA" {
		t.Fatalf("scoped projects = %+v", projects.Items)
	}
}

func TestSolutionComparisonWorkerBindsConfiguredPromptVersions(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open("file:solution_catalog_worker_prompt?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.AutoMigrate(
		&db.TaskTelemetry{}, &db.SolutionAsset{}, &db.SolutionRevision{}, &db.SolutionPromptTemplate{},
		&db.SolutionCatalogEntry{}, &db.SolutionCatalogSearchToken{}, &db.SolutionCatalogSyncJob{},
		&db.SolutionComparison{}, &db.SolutionStandardizationProposal{}, &db.SolutionStandard{}, &db.SolutionStandardRevision{},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	entries := make([]db.SolutionCatalogEntry, 0, 2)
	for index, fixture := range []struct{ demandID, projectKey string }{{"WA-701", "WA"}, {"DG-702", "DG"}} {
		if err := conn.Create(&db.TaskTelemetry{TaskID: fixture.demandID, ProjectKey: fixture.projectKey, Title: "统一重试治理", Description: "任务失败后幂等重试", TaskCreatedAt: now, LastUpdate: now}).Error; err != nil {
			t.Fatal(err)
		}
		asset := db.SolutionAsset{DemandID: fixture.demandID, Revision: 2, Sequence: 1, CreatedBy: "owner", CreatedAt: now, UpdatedAt: now}
		if err := conn.Create(&asset).Error; err != nil {
			t.Fatal(err)
		}
		markdown := "# 重试方案\n\n幂等、告警与回退。"
		hash := sha256.Sum256([]byte(markdown))
		revision := db.SolutionRevision{SolutionAssetID: asset.ID, Version: 1, Kind: "human_draft", Status: "published", Title: "重试方案", Content: []byte(markdown), ContentEncoding: "identity", ContentHash: hex.EncodeToString(hash[:]), ContentBytes: len(markdown), StoredBytes: len(markdown), AuthoredBy: "owner", CreatedAt: now}
		if err := conn.Create(&revision).Error; err != nil {
			t.Fatal(err)
		}
		entry := db.SolutionCatalogEntry{SolutionAssetID: asset.ID, PublishedRevisionID: revision.ID, DemandID: fixture.demandID, ProjectKey: fixture.projectKey, DemandTitle: "统一重试治理", SolutionTitle: "重试方案", ContentHash: revision.ContentHash, ContentBytes: revision.ContentBytes, StoredBytes: revision.StoredBytes, PublishedAt: now.Add(time.Duration(index) * time.Minute), SyncedAt: now}
		if err := conn.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	requirementPrompt := db.SolutionPromptTemplate{Purpose: "solution_compare_requirement", ScopeType: "global", Version: 3, Status: "active", Name: "需求对比", SystemPrompt: "REQ_PROMPT", CreatedBy: "root", CreatedAt: now}
	compatibilityPrompt := db.SolutionPromptTemplate{Purpose: "solution_compare_compatibility", ScopeType: "global", Version: 4, Status: "active", Name: "方案对比", SystemPrompt: "COMPAT_PROMPT", CreatedBy: "root", CreatedAt: now}
	if err := conn.Create(&requirementPrompt).Error; err != nil {
		t.Fatal(err)
	}
	if err := conn.Create(&compatibilityPrompt).Error; err != nil {
		t.Fatal(err)
	}
	comparison := db.SolutionComparison{PairKey: "1:2", LeftEntryID: entries[0].ID, RightEntryID: entries[1].ID, LeftRevisionID: entries[0].PublishedRevisionID, RightRevisionID: entries[1].PublishedRevisionID, RecallScore: 0.9, Status: solutioncatalog.ComparisonQueued, Stage: solutioncatalog.ComparisonStageRoundOne, CreatedAt: now, UpdatedAt: now}
	if err := conn.Create(&comparison).Error; err != nil {
		t.Fatal(err)
	}
	catalogModule := solutioncatalog.New(conn, solutioncatalog.Settings{}, solutioncatalog.WithClock(func() time.Time { return now }))
	server := &Server{solutions: solutions.New(conn), solutionCatalog: catalogModule}
	server.solutionLLM = func(_ context.Context, systemPrompt, _ string) (string, error) {
		switch systemPrompt {
		case "REQ_PROMPT":
			return `{"equivalent":true,"score":0.93,"reason":"目标一致"}`, nil
		case "COMPAT_PROMPT":
			return `{"compatible":true,"standardizable":true,"score":0.88,"summary":"公共核心稳定","proposal_title":"标准重试方案","proposal_markdown":"# 标准重试方案\\n\\n幂等、告警与回退。"}`, nil
		default:
			t.Fatalf("unexpected configured prompt: %q", systemPrompt)
			return "", nil
		}
	}
	for stage := 0; stage < 2; stage++ {
		processed, err := server.processOneSolutionComparison(context.Background())
		if err != nil || !processed {
			t.Fatalf("comparison stage %d processed=%v err=%v", stage+1, processed, err)
		}
	}
	if err := conn.First(&comparison, comparison.ID).Error; err != nil {
		t.Fatal(err)
	}
	if comparison.RoundOnePromptVersion != "prompt:"+strconv.FormatUint(uint64(requirementPrompt.ID), 10) || comparison.RoundTwoPromptVersion != "prompt:"+strconv.FormatUint(uint64(compatibilityPrompt.ID), 10) {
		t.Fatalf("comparison prompt bindings = round1:%q round2:%q", comparison.RoundOnePromptVersion, comparison.RoundTwoPromptVersion)
	}
	if comparison.Status != solutioncatalog.ComparisonNeedsReview {
		t.Fatalf("comparison status = %q, want review", comparison.Status)
	}
}
