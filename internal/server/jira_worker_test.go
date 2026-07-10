package server

import (
	"testing"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

func TestMapJiraStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"To Do", "backlog"},
		{"backlog", "backlog"},
		{"Open", "backlog"},
		{"reopened", "backlog"},
		{"NEW", "backlog"},
		{"todo", "backlog"},
		{"In Progress", "progress"},
		{"progress", "progress"},
		{"active", "progress"},
		{"doing", "progress"},
		{"In Review", "review"},
		{"review", "review"},
		{"under review", "review"},
		{"qa", "review"},
		{"testing", "review"},
		{"Done", "done"},
		{"closed", "done"},
		{"resolved", "done"},
		{"completed", "done"},
		{"unknown status", "backlog"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			actual := mapJiraStatus(tc.input)
			if actual != tc.expected {
				t.Errorf("mapJiraStatus(%q) = %q; want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestMapJiraIssueType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Task", "demand"},
		{"任务", "demand"},
		{"Story", "demand"},
		{"需求", "demand"},
		{"Feature", "demand"},
		{"Epic", "demand"},
		{"Bug", "bug"},
		{"缺陷", "bug"},
		{"故障", "bug"},
		{"Defect", "bug"},
		{"unknown", "demand"},
		{"", "demand"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			actual := mapJiraIssueType(tc.input)
			if actual != tc.expected {
				t.Errorf("mapJiraIssueType(%q) = %q; want %q", tc.input, actual, tc.expected)
			}
		})
	}
}

func TestBuildJQL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.JiraConfig
		expected string
	}{
		{
			name:     "empty config",
			cfg:      config.JiraConfig{},
			expected: "",
		},
		{
			name: "only projects",
			cfg: config.JiraConfig{
				SyncProjects: []string{"PROJ", "TEST"},
			},
			expected: `project in ("PROJ", "TEST")`,
		},
		{
			name: "only users",
			cfg: config.JiraConfig{
				SyncUsers: []string{"eddie@company.com", "dev@company.com"},
			},
			expected: `assignee in ("eddie@company.com", "dev@company.com")`,
		},
		{
			name: "only statuses",
			cfg: config.JiraConfig{
				SyncStatuses: []string{"To Do", "In Progress"},
			},
			expected: `status in ("To Do", "In Progress")`,
		},
		{
			name: "all filters combined",
			cfg: config.JiraConfig{
				SyncProjects: []string{"PROJ"},
				SyncUsers:    []string{"eddie@company.com"},
				SyncStatuses: []string{"In Progress"},
			},
			expected: `project in ("PROJ") AND assignee in ("eddie@company.com") AND status in ("In Progress")`,
		},
		{
			name: "custom JQL overrides other filters",
			cfg: config.JiraConfig{
				SyncProjects: []string{"PROJ"},
				SyncUsers:    []string{"eddie@company.com"},
				SyncStatuses: []string{"In Progress"},
				CustomJQL:    "project = MYPROJ AND type = Bug",
			},
			expected: "project = MYPROJ AND type = Bug",
		},
		{
			name: "whitespace trimming",
			cfg: config.JiraConfig{
				SyncProjects: []string{" PROJ  ", ""},
				SyncUsers:    []string{" eddie@company.com "},
			},
			expected: `project in ("PROJ") AND assignee in ("eddie@company.com")`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := buildJQL(&tc.cfg)
			if actual != tc.expected {
				t.Errorf("buildJQL(...) = %q; want %q", actual, tc.expected)
			}
		})
	}
}

func TestParseJiraTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // YYYY-MM-DD HH:MM:SS format of UTC time
	}{
		{
			name:     "RFC3339 Zulu",
			input:    "2026-06-12T04:27:03Z",
			expected: "2026-06-12 04:27:03",
		},
		{
			name:     "Jira standard with offset",
			input:    "2026-06-12T12:27:03.000+0800",
			expected: "2026-06-12 04:27:03", // UTC equivalent
		},
		{
			name:     "empty string defaults to now",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed := parseJiraTime(tc.input)
			if tc.input == "" {
				if parsed.IsZero() {
					t.Errorf("expected now for empty time, got zero time")
				}
				return
			}
			utcStr := parsed.UTC().Format("2006-01-02 15:04:05")
			if utcStr != tc.expected {
				t.Errorf("parseJiraTime(%q) = %q (UTC); want %q (UTC)", tc.input, utcStr, tc.expected)
			}
		})
	}
}

func TestShouldPreserveLocalAssignee(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		task           db.TaskTelemetry
		incoming       string
		incomingStatus string
		want           bool
	}{
		{
			name: "recent decision reassignment",
			task: db.TaskTelemetry{
				Assignee:     "Bob",
				LastUpdate:   now.Add(-2 * time.Hour),
				DecisionLogs: "[2026-06-23 10:00:00] PM 调停干预：将指派人从 [Alice] 转派给 [Bob]。",
			},
			incoming:       "Alice",
			incomingStatus: "progress",
			want:           true,
		},
		{
			name: "completed Jira status ends local protection",
			task: db.TaskTelemetry{
				Assignee:     "Bob",
				LastUpdate:   now.Add(-2 * time.Hour),
				DecisionLogs: "[2026-06-23 10:00:00] PM 调停干预：将指派人从 [Alice] 转派给 [Bob]。",
			},
			incoming:       "Alice",
			incomingStatus: "done",
			want:           false,
		},
		{
			name: "stale decision can be refreshed from Jira",
			task: db.TaskTelemetry{
				Assignee:     "Bob",
				LastUpdate:   now.Add(-25 * time.Hour),
				DecisionLogs: "[2026-06-22 10:00:00] PM 调停干预：将指派人从 [Alice] 转派给 [Bob]。",
			},
			incoming:       "Alice",
			incomingStatus: "progress",
			want:           false,
		},
		{
			name: "no decision log follows Jira",
			task: db.TaskTelemetry{
				Assignee:   "Bob",
				LastUpdate: now.Add(-2 * time.Hour),
			},
			incoming:       "Alice",
			incomingStatus: "progress",
			want:           false,
		},
		{
			name: "same assignee is not protected",
			task: db.TaskTelemetry{
				Assignee:     "Bob",
				LastUpdate:   now.Add(-2 * time.Hour),
				DecisionLogs: "调整需求负责人：从 [Alice] 转派给 [Bob]。",
			},
			incoming:       "Bob",
			incomingStatus: "progress",
			want:           false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldPreserveLocalAssignee(tc.task, tc.incoming, tc.incomingStatus, now)
			if got != tc.want {
				t.Fatalf("shouldPreserveLocalAssignee() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldIncludeInJiraKeepAlive(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		task db.TaskTelemetry
		want bool
	}{
		{
			name: "active task is always kept alive",
			task: db.TaskTelemetry{
				TaskID:     "FZ-2220",
				Status:     "progress",
				LastUpdate: now.Add(-45 * 24 * time.Hour),
			},
			want: true,
		},
		{
			name: "recent completed issue is kept alive for owner corrections",
			task: db.TaskTelemetry{
				TaskID:     "FZ-2220",
				Status:     "done",
				LastUpdate: now.Add(-6 * 24 * time.Hour),
			},
			want: true,
		},
		{
			name: "old completed issue exits keep alive",
			task: db.TaskTelemetry{
				TaskID:     "FZ-2220",
				Status:     "done",
				LastUpdate: now.Add(-30 * 24 * time.Hour),
			},
			want: false,
		},
		{
			name: "completed issue without update time exits keep alive",
			task: db.TaskTelemetry{
				TaskID: "FZ-2220",
				Status: "done",
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldIncludeInJiraKeepAlive(tc.task, now)
			if got != tc.want {
				t.Fatalf("shouldIncludeInJiraKeepAlive() = %v, want %v", got, tc.want)
			}
		})
	}
}
