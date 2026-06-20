package telemetry

import (
	"testing"
)

func TestExtractTaskID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"dev/task-101-xxx", "task-101"},
		{"feat(#task-101): xxx", "task-101"},
		{"TASK-102", "task-102"},
		{"task-099", "task-099"},
		{"dev/task-105-new-feature", "task-105"},
		{"feat(#TASK-202): init db", "task-202"},
		{"dev/proj-123-bugfix", "proj-123"},
		{"fix(#BUG-456): resolve crash", "bug-456"},
		{"main", ""},
		{"dev/feat-something", ""},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			actual := ExtractTaskID(tc.input)
			if actual != tc.expected {
				t.Errorf("ExtractTaskID(%q) = %q; want %q", tc.input, actual, tc.expected)
			}
		})
	}
}
