package packaging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeReviewRuntimeBundleSurvivesDockerContext(t *testing.T) {
	root := projectRoot(t)
	matcher := contextMatcher(t, root)
	for _, path := range []string{
		"review_skill_assets.go",
		".agents/skills/merge-review/SKILL.md",
		".agents/skills/merge-review/references/review_framework.md",
		".agents/skills/merge-review/references/report_templates.md",
	} {
		assertIncluded(t, root, matcher, path)
	}
	data, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	for _, instruction := range []string{
		"COPY review_skill_assets.go ./",
		"COPY .agents/skills/merge-review/ ./.agents/skills/merge-review/",
		"COPY --chown=1000:1000 .agents/skills/merge-review/ .agents/skills/merge-review/",
	} {
		if !strings.Contains(string(data), instruction) {
			t.Errorf("missing runtime skill build input: %s", instruction)
		}
	}
}
