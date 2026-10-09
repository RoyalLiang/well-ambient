package server

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/testutil/pgtest"
)

func TestDailyJiraPageEnrichmentsSQLDialects(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			previous := db.DB
			t.Cleanup(func() { db.DB = previous })
			if dialect == "postgres" {
				conn := pgtest.Open(t)
				if err := conn.AutoMigrate(&db.DecisionEvent{}, &db.DailyJiraDecision{}); err != nil {
					t.Fatal(err)
				}
				db.DB = conn
			} else {
				setupServerTestDB(t)
			}

			for _, statement := range []string{
				`CREATE INDEX IF NOT EXISTS idx_daily_jira_event_task_action_created ON decision_events (task_id, action, created_at DESC, id DESC)`,
				`CREATE INDEX IF NOT EXISTS idx_daily_jira_decision_task_created ON daily_jira_decisions (task_id, created_at DESC, id DESC)`,
			} {
				if err := db.DB.Exec(statement).Error; err != nil {
					t.Fatal(err)
				}
			}

			now := time.Date(2026, time.October, 9, 10, 0, 0, 0, time.UTC)
			wantEvents := make(map[string][]uint)
			wantDecisions := make(map[string]uint)
			for _, taskID := range []string{"PAGE-1", "PAGE-2", "OTHER-1"} {
				for i := 0; i < 12; i++ {
					// Equal timestamp pairs exercise the ID tie-breaker in both queries.
					createdAt := now.Add(time.Duration(i/2) * time.Minute)
					event := db.DecisionEvent{
						TaskID: taskID, Action: "daily_jira_follow_up", Actor: "PM",
						Reason: fmt.Sprintf("event %d", i), CreatedAt: createdAt,
					}
					if err := db.DB.Create(&event).Error; err != nil {
						t.Fatal(err)
					}
					wantEvents[taskID] = append(wantEvents[taskID], event.ID)
					decision := db.DailyJiraDecision{
						TaskID: taskID, Status: "follow_up", Actor: "PM",
						Note: fmt.Sprintf("decision %d", i), CreatedAt: createdAt,
					}
					if err := db.DB.Create(&decision).Error; err != nil {
						t.Fatal(err)
					}
					wantDecisions[taskID] = decision.ID
				}
			}
			for _, action := range []string{"dailyXjira_follow_up", "daily_jiraXfollow_up", "DAILY_JIRA_follow_up", "daily_jira", "override_assignee"} {
				event := db.DecisionEvent{TaskID: "PAGE-1", Action: action, CreatedAt: now.Add(time.Hour)}
				if err := db.DB.Create(&event).Error; err != nil {
					t.Fatal(err)
				}
			}

			events, decisions, err := loadDailyJiraPageEnrichments([]string{"PAGE-1", "PAGE-2", "MISSING-1"})
			if err != nil {
				t.Fatalf("load Daily Jira page enrichment on %s: %v", dialect, err)
			}
			if len(events) != 16 || len(decisions) != 2 {
				t.Fatalf("enrichment returned %d events and %d decisions, want 16 and 2", len(events), len(decisions))
			}
			gotEvents := make(map[string][]uint)
			for i, event := range events {
				if event.TaskID != "PAGE-1" && event.TaskID != "PAGE-2" {
					t.Fatalf("event outside requested page: %+v", event)
				}
				if event.Action != "daily_jira_follow_up" {
					t.Fatalf("non-Daily-Jira action included: %+v", event)
				}
				if i > 0 {
					previous := events[i-1]
					if previous.CreatedAt.Before(event.CreatedAt) || (previous.CreatedAt.Equal(event.CreatedAt) && previous.ID < event.ID) {
						t.Fatalf("events are not in descending timestamp/ID order: %+v before %+v", previous, event)
					}
				}
				gotEvents[event.TaskID] = append(gotEvents[event.TaskID], event.ID)
			}
			for _, taskID := range []string{"PAGE-1", "PAGE-2"} {
				var want []uint
				for i := 11; i >= 4; i-- {
					want = append(want, wantEvents[taskID][i])
				}
				if !reflect.DeepEqual(gotEvents[taskID], want) {
					t.Fatalf("%s event IDs = %v, want latest eight %v", taskID, gotEvents[taskID], want)
				}
			}
			for _, decision := range decisions {
				if decision.TaskID != "PAGE-1" && decision.TaskID != "PAGE-2" {
					t.Fatalf("decision outside requested page: %+v", decision)
				}
				if decision.ID != wantDecisions[decision.TaskID] {
					t.Fatalf("%s decision ID = %d, want latest %d", decision.TaskID, decision.ID, wantDecisions[decision.TaskID])
				}
				delete(wantDecisions, decision.TaskID)
			}

			for _, taskIDs := range [][]string{nil, {}, {"MISSING-1"}} {
				events, decisions, err := loadDailyJiraPageEnrichments(taskIDs)
				if err != nil || len(events) != 0 || len(decisions) != 0 {
					t.Fatalf("empty enrichment for %v: events=%v decisions=%v err=%v", taskIDs, events, decisions, err)
				}
			}
		})
	}
}
