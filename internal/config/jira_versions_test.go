package config

import "testing"

func TestParseJiraVersionURL(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		versionURL string
		projectKey string
		versionID  string
		canonical  string
		wantError  bool
	}{
		{
			name:       "absolute release page",
			baseURL:    "https://jira.westwell-lab.com",
			versionURL: "https://jira.westwell-lab.com/projects/PRJ25024/versions/13622",
			projectKey: "PRJ25024",
			versionID:  "13622",
			canonical:  "https://jira.westwell-lab.com/projects/PRJ25024/versions/13622",
		},
		{
			name:       "relative page with context path and query",
			baseURL:    "https://jira.example.com/jira",
			versionURL: "projects/team_2/versions/42?selectedIssue=TEAM_2-7#issues",
			projectKey: "TEAM_2",
			versionID:  "42",
			canonical:  "https://jira.example.com/jira/projects/team_2/versions/42",
		},
		{
			name:       "wrong site",
			baseURL:    "https://jira.example.com",
			versionURL: "https://attacker.example/projects/PROJ/versions/42",
			wantError:  true,
		},
		{
			name:       "wrong route",
			baseURL:    "https://jira.example.com",
			versionURL: "https://jira.example.com/browse/PROJ-42",
			wantError:  true,
		},
		{
			name:       "invalid version id",
			baseURL:    "https://jira.example.com",
			versionURL: "https://jira.example.com/projects/PROJ/versions/latest",
			wantError:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ref, err := ParseJiraVersionURL(test.baseURL, test.versionURL)
			if test.wantError {
				if err == nil {
					t.Fatalf("ParseJiraVersionURL() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseJiraVersionURL() error = %v", err)
			}
			if ref.ProjectKey != test.projectKey || ref.VersionID != test.versionID || ref.CanonicalURL != test.canonical {
				t.Fatalf("ParseJiraVersionURL() = %#v, want project=%q version=%q canonical=%q", ref, test.projectKey, test.versionID, test.canonical)
			}
		})
	}
}

func TestNormalizeJiraVersionSources(t *testing.T) {
	cfg := JiraConfig{
		BaseURL: "https://jira.example.com/",
		VersionSources: []JiraVersionSource{
			{ProjectName: "Release Train", VersionURL: "https://jira.example.com/projects/proj/versions/13622?tab=issues"},
			{},
		},
	}
	if err := NormalizeJiraVersionSources(&cfg); err != nil {
		t.Fatalf("NormalizeJiraVersionSources() error = %v", err)
	}
	if len(cfg.VersionSources) != 1 {
		t.Fatalf("len(VersionSources) = %d, want 1", len(cfg.VersionSources))
	}
	source := cfg.VersionSources[0]
	if source.ProjectKey != "PROJ" || source.ProjectName != "Release Train" || source.VersionURL != "https://jira.example.com/projects/proj/versions/13622" {
		t.Fatalf("normalized source = %#v", source)
	}
}

func TestNormalizeJiraVersionSourcesRejectsMismatchAndDuplicates(t *testing.T) {
	t.Run("project mismatch", func(t *testing.T) {
		cfg := JiraConfig{
			BaseURL: "https://jira.example.com",
			VersionSources: []JiraVersionSource{{
				ProjectKey:  "OTHER",
				ProjectName: "Other",
				VersionURL:  "https://jira.example.com/projects/PROJ/versions/1",
			}},
		}
		if err := NormalizeJiraVersionSources(&cfg); err == nil {
			t.Fatal("NormalizeJiraVersionSources() error = nil, want project mismatch")
		}
	})

	t.Run("duplicate release", func(t *testing.T) {
		cfg := JiraConfig{
			BaseURL: "https://jira.example.com",
			VersionSources: []JiraVersionSource{
				{ProjectKey: "PROJ", ProjectName: "Project", VersionURL: "https://jira.example.com/projects/PROJ/versions/1"},
				{ProjectKey: "PROJ", ProjectName: "Project", VersionURL: "https://jira.example.com/projects/PROJ/versions/1?tab=issues"},
			},
		}
		if err := NormalizeJiraVersionSources(&cfg); err == nil {
			t.Fatal("NormalizeJiraVersionSources() error = nil, want duplicate release")
		}
	})
}

func TestJiraProjectKeysIncludesVersionSources(t *testing.T) {
	cfg := JiraConfig{
		BaseURL:      "https://jira.example.com",
		SyncProjects: []string{"OPS", "ops"},
		VersionSources: []JiraVersionSource{{
			ProjectKey:  "PRJ25024",
			ProjectName: "Release Train",
			VersionURL:  "https://jira.example.com/projects/PRJ25024/versions/13622",
		}},
	}
	keys := JiraProjectKeys(&cfg)
	if len(keys) != 2 || keys[0] != "OPS" || keys[1] != "PRJ25024" {
		t.Fatalf("JiraProjectKeys() = %#v, want [OPS PRJ25024]", keys)
	}
}
