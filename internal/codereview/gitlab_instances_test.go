package codereview

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestGitLabRequestsKeepInstanceCredentialsSeparated(t *testing.T) {
	seen := map[string]string{}
	client := &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		seen[r.URL.Host] = r.Header.Get("PRIVATE-TOKEN")
		if r.URL.Path != "/api/v4/projects/10/repository/commits" {
			t.Fatalf("remote path %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`[]`))}, nil
	})}
	g := GitLab{Client: client, Config: config.GitLabConfig{BaseURL: "https://devgit.example.test", APIToken: "dev-only", Repos: []config.RepoMapping{{ProjectID: "10"}, {ProjectID: "fde:10", InstanceID: "fde", RemoteProjectID: "10"}}, Connections: []config.GitLabConnection{{ID: "fde", Enabled: true, BaseURL: "https://fde.example.test", APIToken: "fde-only"}}}}
	for _, project := range []string{"10", "fde:10"} {
		if _, err := g.request(context.Background(), http.MethodGet, projectPath(project)+"/repository/commits", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if seen["devgit.example.test"] != "dev-only" || seen["fde.example.test"] != "fde-only" {
		t.Fatal("credentials crossed instances")
	}
	if _, err := g.request(context.Background(), http.MethodGet, projectPath("missing:10")+"/repository/commits", nil, nil); err == nil {
		t.Fatal("unknown instance was requested")
	}
	delete(seen, "fde.example.test")
	g.Config.Connections[0].Enabled = false
	if _, err := g.request(context.Background(), http.MethodGet, projectPath("fde:10")+"/repository/commits", nil, nil); err == nil || seen["fde.example.test"] != "" {
		t.Fatal("disabled instance sent credentials or fetched source")
	}
}

func TestPublicationIdentitySeparatesFrozenOrigins(t *testing.T) {
	a := db.CodeReviewRun{ProjectID: "fde:10", Kind: "commit", Ref: "same", HeadSHA: "same"}
	legacy := publicationKey(a)
	a.RepositorySourceJSON = (config.RepositorySource{RepositoryKey: "gitlab:https://a.example.test:10"}).JSON()
	b := a
	b.RepositorySourceJSON = (config.RepositorySource{RepositoryKey: "gitlab:https://b.example.test:10"}).JSON()
	if publicationKey(a) == publicationKey(b) || publicationKey(a) == legacy {
		t.Fatal("publication identities ignore frozen origin")
	}
}

func TestLegacyDefaultPublicationsRemainIdempotent(t *testing.T) {
	for _, state := range []string{"published", "unknown", "sending"} {
		t.Run(state, func(t *testing.T) {
			f := newFixture(t)
			f.cfg.GitLab.LegacyDedupeOrigin = f.cfg.GitLab.BaseURL
			f.policy(t, db.CodeReviewPolicy{SyncMRs: true})
			run := f.review(t, "mr")
			legacy := run
			legacy.RepositorySourceJSON = ""
			if publicationKey(run) != publicationKey(legacy) {
				t.Fatal("anchored default lost legacy key")
			}
			if err := f.s.DB.Create(&db.CodeReviewPublication{Key: publicationKey(legacy), RunID: run.ID, Status: state, CommentID: 321}).Error; err != nil {
				t.Fatal(err)
			}
			if err := f.s.Publish(context.Background(), run); err != nil {
				t.Fatal(err)
			}
			if f.posts != 0 {
				t.Fatal("legacy publication was sent twice")
			}
			f.cfg.GitLab.BaseURL = "https://new-origin.example.test"
			source := f.cfg.GitLab.Source(f.cfg.GitLab.Repos[0], run.CreatedAt)
			run.RepositorySourceJSON = source.JSON()
			if source.LegacyDedupe || publicationKey(run) == publicationKey(legacy) {
				t.Fatal("new origin inherited legacy claims")
			}
		})
	}
}

func TestLegacyQueuedReviewCannotReadChangedOrigin(t *testing.T) {
	f := newFixture(t)
	f.cfg.GitLab.LegacyDedupeOrigin = f.cfg.GitLab.BaseURL
	run, err := f.s.Enqueue(context.Background(), "10", "commit", head, "legacy-origin-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.DB.Model(&run).Update("repository_source_json", "").Error; err != nil {
		t.Fatal(err)
	}
	f.cfg.GitLab.BaseURL = "https://changed.example.test"
	requests := 0
	f.s.HTTP = &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) { requests++; return nil, io.EOF })}
	if _, err := f.s.ProcessOne(context.Background()); err == nil {
		t.Fatal("legacy run read another origin")
	}
	if requests != 0 || f.calls != 0 {
		t.Fatal("legacy run performed external work")
	}
}
