package skillassets

import (
	"embed"
	"strings"
)

// Keep the runtime strategy and its report contract on the same canonical
// sources that are distributed as the merge-review skill.
//
//go:embed .agents/skills/merge-review/SKILL.md .agents/skills/merge-review/references/review_framework.md .agents/skills/merge-review/references/report_templates.md
var reviewFiles embed.FS

func MergeReviewBundle() string {
	var content strings.Builder
	for _, path := range []string{
		".agents/skills/merge-review/SKILL.md",
		".agents/skills/merge-review/references/review_framework.md",
		".agents/skills/merge-review/references/report_templates.md",
	} {
		data, err := reviewFiles.ReadFile(path)
		if err != nil {
			panic(err) // All three exact paths are verified by go:embed at build time.
		}
		content.WriteString("\n## " + path + "\n")
		content.Write(data)
	}
	return content.String()
}
