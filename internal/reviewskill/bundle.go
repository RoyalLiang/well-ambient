// Package reviewskill contains the user-provided, versioned merge-review bundle.
package reviewskill

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"strings"
)

//go:embed assets/SKILL.md assets/references/*.md assets/scripts/*
var Files embed.FS

func Prompt() string {
	parts := []string{}
	for _, name := range []string{"SKILL.md", "references/release_policy.md", "references/report_templates.md"} {
		data, err := Files.ReadFile("assets/" + name)
		if err != nil {
			panic(err)
		}
		parts = append(parts, "<merge_review_document path=\""+name+"\">\n"+string(data)+"\n</merge_review_document>")
	}
	return strings.Join(parts, "\n\n")
}
func Hash() string { sum := sha256.Sum256([]byte(Prompt())); return hex.EncodeToString(sum[:]) }
