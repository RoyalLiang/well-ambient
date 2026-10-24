package codereview

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"well-ambient/internal/db"
)

func TestContractDistinctFrozenSourcesStayVisible(t *testing.T) {
	f := newFixture(t)
	repo := f.cfg.GitLab.Repos[0]
	oldSource := f.cfg.GitLab.Source(repo, time.Now())
	f.cfg.GitLab.BaseURL = "https://new-origin.example.test"
	newSource := f.cfg.GitLab.Source(repo, time.Now())
	for i, source := range []string{oldSource.JSON(), newSource.JSON()} {
		run := db.CodeReviewRun{Key: string(rune('a' + i)), ProjectID: "10", Kind: "mr", Ref: "7", HeadSHA: head, BaseSHA: base, Status: "completed", RepositorySourceJSON: source}
		if err := f.s.DB.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.s.Search(context.Background(), RunFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 {
		t.Fatalf("different frozen repository keys collapsed: total=%d", page.Total)
	}
}
func TestContractLegacyExportDoesNotMixRepositorySources(t *testing.T) {
	f := newFixture(t)
	repo := f.cfg.GitLab.Repos[0]
	oldSource := f.cfg.GitLab.Source(repo, time.Now())
	f.cfg.GitLab.BaseURL = "https://new-origin.example.test"
	called := 0
	f.s.HTTP.Transport = transport(func(r *http.Request) (*http.Response, error) {
		called++
		payload, _ := json.Marshal(map[string]any{"source_branch": "different-origin-branch", "diff_refs": map[string]string{"head_sha": head, "base_sha": base}})
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
	})
	run := db.CodeReviewRun{ID: 77, ProjectID: "10", Repo: "dispatch", Kind: "mr", Ref: "7", HeadSHA: head, BaseSHA: base, Status: "completed", RepositorySourceJSON: oldSource.JSON(), SnapshotJSON: encode(Snapshot{RepositorySource: oldSource, Complete: true}), ReportJSON: encode(exampleReport())}
	document, err := f.s.ExportDocument(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	if called != 0 || strings.Contains(document.Filename, "different-origin-branch") {
		t.Fatalf("frozen legacy report used current other origin: calls=%d filename=%q", called, document.Filename)
	}
}

func TestReviewSearchKeepsMalformedAndUnknownSourcesDistinct(t *testing.T) {
	f := newFixture(t)
	for i, source := range []string{`{"repository_key":"same"`, `{"repository_key":"same"`, "", ""} {
		run := db.CodeReviewRun{Key: fmt.Sprintf("malformed-%d", i), ProjectID: "10", Kind: "mr", Ref: "7", HeadSHA: head, BaseSHA: base, RepositorySourceJSON: source}
		if err := f.s.DB.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
	page, err := f.s.Search(context.Background(), RunFilter{})
	if err != nil || page.Total != 4 {
		t.Fatalf("unknown sources collapsed: %+v %v", page, err)
	}
}
