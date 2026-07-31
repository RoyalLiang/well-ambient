package server

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"well-ambient/internal/db"
)

func TestQueryFilterValuesSupportsRepeatedAndCommaSeparatedValues(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/tasks?project=HIT&project=ns2,DG&project=all&project=hit", nil)
	got := queryFilterValues(request, "project")
	want := []string{"HIT", "ns2", "DG"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("queryFilterValues() = %#v, want %#v", got, want)
	}
}

func TestFilterTaskTelemetriesByQueryUsesUnionWithinEachFilter(t *testing.T) {
	tasks := []db.TaskTelemetry{
		{TaskID: "HIT-1", Assignee: "Alice"},
		{TaskID: "NS2-2", Assignee: "Bob"},
		{TaskID: "DG-3", Assignee: "Alice"},
		{TaskID: "HIT-4", Assignee: "Vendor"},
		{TaskID: "LEGACY-5", ProjectKey: "HIT", Assignee: "Alice"},
		{TaskID: "HIT-6", ProjectKey: "DG", Assignee: "Alice"},
	}

	filtered := filterTaskTelemetriesByQuery(tasks, []string{"hit", "NS2"}, []string{"alice", "Bob"})
	if len(filtered) != 3 || filtered[0].TaskID != "HIT-1" || filtered[1].TaskID != "NS2-2" || filtered[2].TaskID != "LEGACY-5" {
		t.Fatalf("filtered tasks = %#v, want HIT-1, NS2-2, and explicit HIT LEGACY-5", filtered)
	}

	if got := filterTaskTelemetriesByQuery(tasks, nil, nil); len(got) != len(tasks) {
		t.Fatalf("empty filters returned %d tasks, want %d", len(got), len(tasks))
	}
}
