package reviewread

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRepositorySourceRequiresPublishedFields(t *testing.T) {
	module, conn, policy := testReviewModule(t)
	rows := seedReviews(t, conn)
	if err := conn.Model(&rows[0]).Updates(map[string]any{"repository_source_json": `{"instance_url":"https://fde.example.test","full_path":"project-code/a/service"}`}).Error; err != nil {
		t.Fatal(err)
	}
	for _, capability := range []string{"review_search", "review_get"} {
		for _, allow := range []bool{false, true} {
			if allow {
				policy.FieldRules[capability] = append(policy.FieldRules[capability], "project_id", "repository_source_json")
			}
			var value any
			var err error
			if capability == "review_search" {
				value, err = module.Search(context.Background(), policy, "test", SearchRequest{})
			} else {
				value, err = module.Get(context.Background(), policy, "test", GetRequest{RunID: rows[0].ID})
			}
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(value)
			if strings.Contains(string(encoded), "repository_source_json") != allow || strings.Contains(string(encoded), "project_id") != allow {
				t.Fatalf("source field policy mismatch: %s allow=%v", capability, allow)
			}
		}
	}
}
