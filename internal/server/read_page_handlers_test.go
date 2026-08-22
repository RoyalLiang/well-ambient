package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"
)

func TestContextFactsUseGenerationBoundKeysetPages(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	base := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	rows := make([]db.ContextFact, 0, 125)
	for index := 0; index < 125; index++ {
		rows = append(rows, db.ContextFact{
			Type: "architecture", Scope: "global", Status: "active",
			Summary: fmt.Sprintf("fact-%03d", index), Content: "bounded",
			UpdatedAt: base.Add(time.Duration(index) * time.Second),
		})
	}
	if err := db.DB.CreateInBatches(&rows, 25).Error; err != nil {
		t.Fatalf("seed context facts: %v", err)
	}

	first := getContextFactPage(t, server, "limit=50&status=active")
	if len(first.Facts) != 50 || !first.Page.HasMore || first.Page.NextCursor == "" {
		t.Fatalf("first page = %d facts, %+v", len(first.Facts), first.Page)
	}
	second := getContextFactPage(t, server, "limit=50&status=active&cursor="+url.QueryEscape(first.Page.NextCursor))
	if len(second.Facts) != 50 || first.Facts[len(first.Facts)-1].ID == second.Facts[0].ID {
		t.Fatalf("second page = %d facts; first/second boundary IDs = %d/%d", len(second.Facts), first.Facts[len(first.Facts)-1].ID, second.Facts[0].ID)
	}

	if err := db.DB.Model(&db.ContextFact{}).Where("id = ?", rows[0].ID).Update("summary", "changed").Error; err != nil {
		t.Fatalf("mutate context fact: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/context/facts?limit=50&status=active&cursor="+url.QueryEscape(first.Page.NextCursor), nil)
	recorder := httptest.NewRecorder()
	server.handleListContextFacts(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("stale cursor status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestExecutionRunsUseBoundedPagesAndBatchBoundedActions(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	base := time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC)
	runs := make([]db.ExecutionRun, 0, 125)
	for index := 0; index < 125; index++ {
		runs = append(runs, db.ExecutionRun{
			RunKey: fmt.Sprintf("bounded-run-%03d", index), DemandID: "FZ-2257",
			Status: "pending", CreatedAt: base.Add(time.Duration(index) * time.Second), UpdatedAt: base,
		})
	}
	if err := db.DB.CreateInBatches(&runs, 25).Error; err != nil {
		t.Fatalf("seed execution runs: %v", err)
	}
	actions := make([]db.ExecutionAction, 0, 25)
	for index := 0; index < 25; index++ {
		actions = append(actions, db.ExecutionAction{
			ExecutionRunID: runs[len(runs)-1].ID,
			ActionKey:      fmt.Sprintf("bounded-action-%03d", index),
			Action:         "step", Status: "completed", CreatedAt: base.Add(time.Duration(index) * time.Second),
		})
	}
	if err := db.DB.CreateInBatches(&actions, 25).Error; err != nil {
		t.Fatalf("seed execution actions: %v", err)
	}

	first := getExecutionRunPage(t, server, "limit=50&demand_id=FZ-2257")
	if len(first.Items) != 50 || !first.Page.HasMore || first.Page.NextCursor == "" {
		t.Fatalf("first page = %d runs, %+v", len(first.Items), first.Page)
	}
	if got := len(first.Items[0].Actions); got != maxExecutionActionsPerRun {
		t.Fatalf("nested action count = %d, want %d", got, maxExecutionActionsPerRun)
	}
	second := getExecutionRunPage(t, server, "limit=50&demand_id=FZ-2257&cursor="+url.QueryEscape(first.Page.NextCursor))
	if len(second.Items) != 50 || first.Items[len(first.Items)-1].ID == second.Items[0].ID {
		t.Fatalf("second page = %d runs; boundary IDs = %d/%d", len(second.Items), first.Items[len(first.Items)-1].ID, second.Items[0].ID)
	}

	if err := db.DB.Create(&db.ExecutionAction{
		ExecutionRunID: runs[0].ID, ActionKey: "generation-change", Action: "step", Status: "completed", CreatedAt: base.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("mutate execution dataset: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/execution/runs?limit=50&demand_id=FZ-2257&cursor="+url.QueryEscape(first.Page.NextCursor), nil)
	recorder := httptest.NewRecorder()
	server.handleListExecutionRuns(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("stale execution cursor status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestContextCorpusListsUseBoundedPagesAndProjectedCounts(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	base := time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC)
	documents := make([]db.ContextDocument, 0, 125)
	for index := 0; index < 125; index++ {
		documents = append(documents, db.ContextDocument{
			Title: fmt.Sprintf("document-%03d", index), Status: "active", IngestionStatus: "completed",
			ContentHash: fmt.Sprintf("hash-%03d", index), Content: strings.Repeat("large-body", 100),
			CreatedAt: base.Add(time.Duration(index) * time.Second), UpdatedAt: base.Add(time.Duration(index) * time.Second),
		})
	}
	if err := db.DB.CreateInBatches(&documents, 25).Error; err != nil {
		t.Fatalf("seed context documents: %v", err)
	}
	candidates := make([]db.CorpusCandidate, 0, len(documents)*2)
	for index, document := range documents {
		createdAt := base.Add(time.Duration(index) * time.Second)
		candidates = append(candidates,
			db.CorpusCandidate{ContextDocumentID: document.ID, Title: "pending", Status: "pending", CreatedAt: createdAt, UpdatedAt: createdAt},
			db.CorpusCandidate{ContextDocumentID: document.ID, Title: "published", Status: "accepted", CreatedAt: createdAt.Add(time.Millisecond), UpdatedAt: createdAt},
		)
	}
	if err := db.DB.CreateInBatches(&candidates, 50).Error; err != nil {
		t.Fatalf("seed corpus candidates: %v", err)
	}

	documentPage := getContextDocumentPage(t, server, "limit=50&status=all")
	if len(documentPage.Items) != 50 || !documentPage.Page.HasMore || documentPage.Page.NextCursor == "" {
		t.Fatalf("document page = %d items, %+v", len(documentPage.Items), documentPage.Page)
	}
	if documentPage.Items[0].Content != "" || documentPage.Items[0].CandidateCount != 2 || documentPage.Items[0].PendingCount != 1 || documentPage.Items[0].PublishedCount != 1 {
		t.Fatalf("document projection = %+v", documentPage.Items[0])
	}
	candidatePage := getCorpusCandidatePage(t, server, "limit=50&status=all")
	if len(candidatePage.Items) != 50 || !candidatePage.Page.HasMore || candidatePage.Page.NextCursor == "" {
		t.Fatalf("candidate page = %d items, %+v", len(candidatePage.Items), candidatePage.Page)
	}
	if candidatePage.Items[0].SourceDocument == nil || candidatePage.Items[0].SourceDocument.ID == 0 {
		t.Fatalf("candidate document projection missing: %+v", candidatePage.Items[0])
	}
	filteredCandidates := getCorpusCandidatePage(t, server, fmt.Sprintf("limit=50&status=all&document_id=%d", documents[0].ID))
	if len(filteredCandidates.Items) != 2 {
		t.Fatalf("document-scoped candidate page = %d items, want 2", len(filteredCandidates.Items))
	}

	extra := make([]db.CorpusCandidate, 0, 125)
	for index := 0; index < 125; index++ {
		extra = append(extra, db.CorpusCandidate{
			ContextDocumentID: documents[0].ID, Title: fmt.Sprintf("detail-%03d", index), Status: "pending",
			CreatedAt: base.Add(time.Duration(index+500) * time.Second), UpdatedAt: base,
		})
	}
	if err := db.DB.CreateInBatches(&extra, 25).Error; err != nil {
		t.Fatalf("seed detail candidates: %v", err)
	}
	detailRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/context/documents/%d", documents[0].ID), nil)
	detailRequest.SetPathValue("id", fmt.Sprintf("%d", documents[0].ID))
	detailRecorder := httptest.NewRecorder()
	server.handleGetContextDocument(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("context document detail status = %d, body = %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	var detail struct {
		Candidates    []db.CorpusCandidate `json:"candidates"`
		CandidatePage struct {
			Limit                int    `json:"limit"`
			HasMore              bool   `json:"has_more"`
			ContinuationEndpoint string `json:"continuation_endpoint"`
		} `json:"candidate_page"`
	}
	if err := json.NewDecoder(detailRecorder.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Candidates) != 100 || !detail.CandidatePage.HasMore || !strings.Contains(detail.CandidatePage.ContinuationEndpoint, "document_id=") {
		t.Fatalf("bounded context detail = %d candidates, %+v", len(detail.Candidates), detail.CandidatePage)
	}
}

func TestReleasesUseBoundedPagesAndPageLocalAggregates(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	base := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	releases := make([]db.ReleaseVersion, 0, 125)
	for index := 0; index < 125; index++ {
		releaseDate := base.AddDate(0, 0, index)
		releases = append(releases, db.ReleaseVersion{
			ProjectKey: "FZ", Source: "local", ExternalID: fmt.Sprintf("bounded-release-%03d", index),
			Name: fmt.Sprintf("Release %03d", index), Status: "planned", ReleaseDate: &releaseDate,
			CreatedAt: base, UpdatedAt: base,
		})
	}
	if err := db.DB.CreateInBatches(&releases, 25).Error; err != nil {
		t.Fatalf("seed releases: %v", err)
	}
	links := []db.WorkItemReleaseLink{
		{WorkItemID: "FZ-1", ReleaseVersionID: releases[0].ID, Relation: "target_fix", IsPrimary: true, Active: true, Source: "local"},
		{WorkItemID: "FZ-2", ReleaseVersionID: releases[0].ID, Relation: "target_fix", IsPrimary: true, Active: true, Source: "local"},
	}
	if err := db.DB.Create(&links).Error; err != nil {
		t.Fatalf("seed release links: %v", err)
	}

	first := getReleasePage(t, server, "limit=50&project_key=FZ")
	if len(first.Items) != 50 || !first.Page.HasMore || first.Page.NextCursor == "" {
		t.Fatalf("release page = %d items, %+v", len(first.Items), first.Page)
	}
	if first.Items[0].JiraIssueCount != 2 {
		t.Fatalf("first release issue count = %d", first.Items[0].JiraIssueCount)
	}
	second := getReleasePage(t, server, "limit=50&project_key=FZ&cursor="+url.QueryEscape(first.Page.NextCursor))
	if len(second.Items) != 50 || first.Items[len(first.Items)-1].Release.ID == second.Items[0].Release.ID {
		t.Fatalf("second release page = %d items; boundary IDs = %d/%d", len(second.Items), first.Items[len(first.Items)-1].Release.ID, second.Items[0].Release.ID)
	}
	projectFirst := getProjectReleasePage(t, server, "FZ", "limit=50")
	if len(projectFirst.Items) != 50 || !projectFirst.Page.HasMore || projectFirst.Page.NextCursor == "" {
		t.Fatalf("project release page = %d items, %+v", len(projectFirst.Items), projectFirst.Page)
	}
	projectSecond := getProjectReleasePage(t, server, "FZ", "limit=50&cursor="+url.QueryEscape(projectFirst.Page.NextCursor))
	if len(projectSecond.Items) != 50 || projectFirst.Items[49].ID == projectSecond.Items[0].ID {
		t.Fatalf("project release second page = %d items", len(projectSecond.Items))
	}
}

func TestReleaseJiraIssuesUseGenerationBoundPages(t *testing.T) {
	setupServerTestDB(t)
	server := NewServer(&config.Config{}, "")
	base := time.Date(2026, 8, 21, 16, 0, 0, 0, time.UTC)
	release := db.ReleaseVersion{ProjectKey: "FZ", Source: "local", ExternalID: "jira-page", Name: "Jira Page", Status: "planned", CreatedAt: base, UpdatedAt: base}
	if err := db.DB.Create(&release).Error; err != nil {
		t.Fatal(err)
	}
	tasks := make([]db.TaskTelemetry, 0, 125)
	links := make([]db.WorkItemReleaseLink, 0, 125)
	for index := 0; index < 125; index++ {
		taskID := fmt.Sprintf("FZ-%04d", index)
		tasks = append(tasks, db.TaskTelemetry{
			TaskID: taskID, ExternalKey: taskID, ProjectKey: "FZ", Source: "jira", IssueType: "bug",
			Title: fmt.Sprintf("bounded issue %03d", index), Status: "progress", LastUpdate: base.Add(time.Duration(index) * time.Second),
		})
		links = append(links, db.WorkItemReleaseLink{
			WorkItemID: taskID, ReleaseVersionID: release.ID, Relation: deliveryplanning.ReleaseTargetFix,
			IsPrimary: true, Active: true, Source: "local",
		})
	}
	if err := db.DB.CreateInBatches(&tasks, 25).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.DB.CreateInBatches(&links, 25).Error; err != nil {
		t.Fatal(err)
	}
	first := getReleaseJiraIssuePage(t, server, release.ID, "scope=linked&limit=50")
	if len(first.Items) != 50 || !first.Page.HasMore || first.Page.NextCursor == "" {
		t.Fatalf("release Jira first page = %d items, %+v", len(first.Items), first.Page)
	}
	second := getReleaseJiraIssuePage(t, server, release.ID, "scope=linked&limit=50&cursor="+url.QueryEscape(first.Page.NextCursor))
	if len(second.Items) != 50 || first.Items[49].WorkItemID == second.Items[0].WorkItemID {
		t.Fatalf("release Jira second page = %d items", len(second.Items))
	}
	if err := db.DB.Model(&db.TaskTelemetry{}).Where("task_id = ?", tasks[0].TaskID).Update("title", "generation changed").Error; err != nil {
		t.Fatal(err)
	}
	request := releaseJiraIssueRequest(release.ID, "scope=linked&limit=50&cursor="+url.QueryEscape(first.Page.NextCursor))
	recorder := httptest.NewRecorder()
	server.handleListReleaseJiraIssues(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("stale release Jira cursor status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

type contextFactPageResponse struct {
	Facts []db.ContextFact `json:"facts"`
	Page  readPageMeta     `json:"page"`
}

type executionRunPageResponse struct {
	Items []executionRunDTO `json:"items"`
	Page  readPageMeta      `json:"page"`
}

type contextDocumentPageResponse struct {
	Items []contextDocumentDTO `json:"items"`
	Page  readPageMeta         `json:"page"`
}

type corpusCandidatePageResponse struct {
	Items []corpusCandidateListItem `json:"items"`
	Page  readPageMeta              `json:"page"`
}

type releasePageResponse struct {
	Items []releasePlanItem `json:"items"`
	Page  readPageMeta      `json:"page"`
}

type projectReleasePageResponse struct {
	Items []db.ReleaseVersion `json:"items"`
	Page  readPageMeta        `json:"page"`
}

type releaseJiraIssuePageResponse struct {
	Items []releaseJiraIssueItem `json:"items"`
	Page  readPageMeta           `json:"page"`
}

func getContextFactPage(t *testing.T, server *Server, query string) contextFactPageResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/context/facts?"+query, nil)
	recorder := httptest.NewRecorder()
	server.handleListContextFacts(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("context facts status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response contextFactPageResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode context facts: %v", err)
	}
	return response
}

func getExecutionRunPage(t *testing.T, server *Server, query string) executionRunPageResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/execution/runs?"+query, nil)
	recorder := httptest.NewRecorder()
	server.handleListExecutionRuns(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("execution runs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response executionRunPageResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode execution runs: %v", err)
	}
	return response
}

func getContextDocumentPage(t *testing.T, server *Server, query string) contextDocumentPageResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/context/documents?"+query, nil)
	recorder := httptest.NewRecorder()
	server.handleListContextDocuments(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("context documents status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response contextDocumentPageResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode context documents: %v", err)
	}
	return response
}

func getCorpusCandidatePage(t *testing.T, server *Server, query string) corpusCandidatePageResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/corpus-candidates?"+query, nil)
	recorder := httptest.NewRecorder()
	server.handleListCorpusCandidates(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("corpus candidates status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response corpusCandidatePageResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode corpus candidates: %v", err)
	}
	return response
}

func getReleasePage(t *testing.T, server *Server, query string) releasePageResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/releases?"+query, nil)
	recorder := httptest.NewRecorder()
	server.handleListReleases(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("releases status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response releasePageResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode releases: %v", err)
	}
	return response
}

func getProjectReleasePage(t *testing.T, server *Server, projectKey, query string) projectReleasePageResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectKey+"/releases?"+query, nil)
	request.SetPathValue("project_key", projectKey)
	recorder := httptest.NewRecorder()
	server.handleListProjectReleases(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("project releases status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response projectReleasePageResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode project releases: %v", err)
	}
	return response
}

func releaseJiraIssueRequest(releaseID uint, query string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/releases/%d/jira-issues?%s", releaseID, query), nil)
	request.SetPathValue("id", fmt.Sprintf("%d", releaseID))
	return request
}

func getReleaseJiraIssuePage(t *testing.T, server *Server, releaseID uint, query string) releaseJiraIssuePageResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	server.handleListReleaseJiraIssues(recorder, releaseJiraIssueRequest(releaseID, query))
	if recorder.Code != http.StatusOK {
		t.Fatalf("release Jira issues status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response releaseJiraIssuePageResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode release Jira issues: %v", err)
	}
	return response
}
