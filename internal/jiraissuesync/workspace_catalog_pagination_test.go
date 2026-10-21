package jiraissuesync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestWorkspaceDiscoverLargeGroupCatalog(t *testing.T) {
	for _, total := range []int{101, 1201, 5000, 5001} {
		t.Run(strconv.Itoa(total), func(t *testing.T) {
			f := workspaceTest(t)
			pages := 0
			jiraReads := 0
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/v4/groups/ops/projects":
					if r.Header.Get("PRIVATE-TOKEN") != "dedicated-gl" || r.URL.Query().Get("per_page") != "100" {
						t.Error("invalid repository request")
					}
					page, _ := strconv.Atoi(r.URL.Query().Get("page"))
					pages++
					first := (page - 1) * 100
					last := min(first+100, total)
					items := []Repo{}
					for i := first; i < last; i++ {
						items = append(items, Repo{ID: int64(i + 1), Path: fmt.Sprintf("ops/repository-%05d", i+1), DefaultBranch: "main", IssuesEnabled: true})
					}
					if last < total {
						w.Header().Set("X-Next-Page", strconv.Itoa(page+1))
					}
					_ = json.NewEncoder(w).Encode(items)
				case "/rest/api/2/project":
					if r.Header.Get("Authorization") != "Bearer dedicated-jira" {
						t.Error("independent Jira credential missing")
					}
					jiraReads++
					_ = json.NewEncoder(w).Encode([]Project{{Key: "NS2", Name: "Nansha"}, {Key: "HIT", Name: "HIT"}})
				default:
					http.NotFound(w, r)
				}
			}))
			defer upstream.Close()
			f.s.Client = upstream.Client()
			view, err := f.s.View(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			config := view.Settings
			config.GroupURL = upstream.URL + "/ops"
			config.JiraURL = upstream.URL
			config.GitLabToken = "dedicated-gl"
			config.JiraToken = "dedicated-jira"
			view, err = f.s.Save(context.Background(), config)
			if err != nil {
				t.Fatal(err)
			}
			run := f.run(t, view.Settings.Version, "discover")
			view, err = f.s.View(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if total <= 5000 {
				if run.Status != "succeeded" {
					t.Fatalf("catalog failed for %d repositories: %s", total, run.Message)
				}
				if len(view.Catalog.Repositories) != total || len(view.Catalog.Projects) != 2 || jiraReads != 1 {
					t.Fatalf("incomplete catalog: repos=%d projects=%d Jira reads=%d", len(view.Catalog.Repositories), len(view.Catalog.Projects), jiraReads)
				}
			} else {
				if run.Status != "failed" || !strings.Contains(run.Message, "5000") {
					t.Fatalf("expected explicit bounded catalog failure, got %+v", run)
				}
				if len(view.Catalog.Repositories) != 0 || len(view.Catalog.Projects) != 0 {
					t.Fatal("partial directory was accepted")
				}
			}
			if pages > 51 {
				t.Fatalf("pagination was not bounded: %d", pages)
			}
		})
	}
}

func TestGitLabDirectoryRejectsBrokenPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Next-Page", "3")
		_ = json.NewEncoder(w).Encode([]Repo{{ID: 1, Path: "ops/one"}})
	}))
	defer server.Close()
	remote, err := newRemote(WorkspaceConfig{GroupURL: server.URL + "/ops", JiraURL: server.URL}, credentials{}, server.Client(), nil)
	if err != nil {
		t.Fatal(err)
	}
	items, err := glList[Repo](context.Background(), remote, "/groups/ops/projects", 5000)
	if err == nil || items != nil || !strings.Contains(err.Error(), "不连续") {
		t.Fatalf("broken pagination accepted: %v %v", items, err)
	}
}
