package config

import (
	"strings"
	"testing"
	"time"
)

func TestGitLabInstanceIdentityAndRouting(t *testing.T) {
	cfg := GitLabConfig{BaseURL: "https://devgit.example.test", APIToken: "default-pat", Repos: []RepoMapping{{Name: "service", Path: "core/service", ProjectID: "10"}, {Name: "service", Path: "project-code/a/service", ProjectID: "fde:10", InstanceID: "fde", RemoteProjectID: "10"}}, Connections: []GitLabConnection{{ID: "fde", Name: "FDE", Enabled: true, BaseURL: "https://fde.example.test", APIToken: "fde-pat"}}}
	if err := cfg.ValidateInstances(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, host, token, want string }{{"/projects/10/repository/commits?ref=main", "https://devgit.example.test", "default-pat", "/projects/10/repository/commits?ref=main"}, {"/projects/fde%3A10/merge_requests/4", "https://fde.example.test", "fde-pat", "/projects/10/merge_requests/4"}, {"/projects/fde%3A10?simple=true", "https://fde.example.test", "fde-pat", "/projects/10?simple=true"}} {
		connection, path, err := cfg.RouteAPIPath(tc.path)
		if err != nil || connection.BaseURL != tc.host || connection.APIToken != tc.token || path != tc.want {
			t.Fatalf("route %s failed: %s %v", tc.path, path, err)
		}
	}
	if _, _, err := cfg.RouteAPIPath("/projects/fde%3A999/commits"); err == nil {
		t.Fatal("unknown qualified repository fell back to default")
	}
	if _, err := cfg.Repository("service"); err == nil {
		t.Fatal("ambiguous short name resolved")
	}
	selected, err := cfg.Connection("")
	if err != nil || len(selected.Repos) != 1 || selected.Repos[0].ProjectID != "10" {
		t.Fatal("default connection leaked FDE repositories")
	}
	source := cfg.Source(cfg.Repos[1], time.Unix(1700000000, 0)).JSON()
	if !strings.Contains(source, "project-code/a/service") || !strings.Contains(source, "https://fde.example.test:10") || strings.Contains(source, "fde-pat") || strings.Contains(source, "default-pat") {
		t.Fatalf("source lacks safe frozen identity: %s", source)
	}
}
