package telemetry

import (
	"regexp"
	"strings"
)

var taskIDRegex = regexp.MustCompile(`(?i)([a-z]+)-\d+`)

// ExtractTaskID extracts a task ID (e.g., "task-101") from a string,
// such as a branch name or commit message. Returns empty string if not found.
func ExtractTaskID(input string) string {
	match := taskIDRegex.FindString(input)
	if match == "" {
		return ""
	}
	return strings.ToLower(match)
}
