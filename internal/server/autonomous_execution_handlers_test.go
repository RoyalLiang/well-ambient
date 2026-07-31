package server

import (
	"strings"
	"testing"
)

func TestParseGeneratedChangeSetAcceptsBoundedUpdatesAndCreates(t *testing.T) {
	raw := "```json\n" + `{
  "summary":"implement handler",
  "change_set":[
    {"action":"update","path":"internal/server/handler.go","content":"package server"},
    {"action":"create","path":"internal/server/handler_test.go","content":"package server"}
  ],
  "test_commands":["go test ./internal/server"]
}` + "\n```"
	result, err := parseGeneratedChangeSet(raw, []sourceFileSnapshot{{Path: "internal/server/handler.go", Content: "package server"}})
	if err != nil {
		t.Fatalf("parse generated change set: %v", err)
	}
	if len(result.ChangeSet) != 2 || len(result.TestCommands) != 1 {
		t.Fatalf("unexpected generated result: %+v", result)
	}
}

func TestParseGeneratedChangeSetRejectsDeleteAndUnsuppliedUpdate(t *testing.T) {
	raw := `{
  "summary":"unsafe",
  "change_set":[
    {"action":"delete","path":"internal/server/handler.go"},
    {"action":"update","path":"internal/server/unknown.go","content":"bad"}
  ],
  "test_commands":["go test ./internal/server"]
}`
	_, err := parseGeneratedChangeSet(raw, []sourceFileSnapshot{{Path: "internal/server/handler.go", Content: "package server"}})
	if err == nil || !strings.Contains(err.Error(), "deletions") || !strings.Contains(err.Error(), "supplied source files") {
		t.Fatalf("expected deletion and source-boundary errors, got %v", err)
	}
}
