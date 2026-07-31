package server

import (
	"net/http/httptest"
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
