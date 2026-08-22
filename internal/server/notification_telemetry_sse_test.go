package server

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestSendTelemetryUpdatedEventIncludesCanonicalTaskID(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/notifications/sse", nil)

	if ok := sendTelemetryUpdatedEvent(recorder, request, recorder, "FZ-2247"); !ok {
		t.Fatal("sendTelemetryUpdatedEvent returned false")
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "event: telemetry-updated\n") {
		t.Fatalf("missing telemetry event name: %q", body)
	}
	if !strings.Contains(body, `"task_id":"FZ-2247"`) {
		t.Fatalf("missing canonical task id: %q", body)
	}
}

func TestTelemetryBroadcastRetainsAndDeduplicatesBurstBeyondClientBuffer(t *testing.T) {
	updates := make(chan string, 1)
	registerTelemetryClient(updates)
	t.Cleanup(func() { unregisterTelemetryClient(updates) })

	BroadcastTelemetryUpdated("DL-4309")
	BroadcastTelemetryUpdated("DL-4310")
	BroadcastTelemetryUpdated("DL-4310")
	BroadcastTelemetryUpdated("DL-4311")

	if got := <-updates; got != "DL-4309" {
		t.Fatalf("buffered telemetry update = %q, want DL-4309", got)
	}
	if got := takePendingTelemetryUpdates(updates); !reflect.DeepEqual(got, []string{"DL-4310", "DL-4311"}) {
		t.Fatalf("pending telemetry updates = %v, want deduplicated burst", got)
	}
}
