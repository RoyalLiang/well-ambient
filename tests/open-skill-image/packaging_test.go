package packaging_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"
	"gopkg.in/yaml.v3"
)

var officialSkills = []string{"jira-analysis", "board-decision", "code-review-reader"}

func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source location is unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func contextMatcher(t *testing.T, root string) *patternmatcher.PatternMatcher {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".dockerignore"))
	if err != nil {
		t.Fatal(err)
	}
	patterns, err := ignorefile.ReadAll(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	matcher, err := patternmatcher.New(patterns)
	if err != nil {
		t.Fatal(err)
	}
	return matcher
}

func assertIncluded(t *testing.T, root string, matcher *patternmatcher.PatternMatcher, relative string) {
	t.Helper()
	excluded, err := matcher.MatchesOrParentMatches(filepath.ToSlash(relative))
	if err != nil {
		t.Fatal(err)
	}
	if excluded {
		t.Errorf("public package resource is absent from Docker build context: %s", relative)
	}
	if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
		t.Errorf("public package resource is absent on disk: %s: %v", relative, err)
	}
}

func TestOfficialSkillsImageContainsCompletePublicPackages(t *testing.T) {
	root := projectRoot(t)
	matcher := contextMatcher(t, root)
	dockerfile, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	stage := regexp.MustCompile(`(?ms)^FROM [^\n]+ AS server\n(.*?)(?:^FROM |\z)`).FindSubmatch(dockerfile)
	if len(stage) != 2 {
		t.Fatal("final server stage is unavailable")
	}
	server := string(stage[1])
	if regexp.MustCompile(`(?m)^WORKDIR\s+`).MatchString(server) {
		t.Error("server image must preserve its base working directory for existing relative configuration and data paths")
	}
	for _, name := range officialSkills {
		directory := filepath.ToSlash(filepath.Join(".agents", "skills", name))
		copyPattern := `(?m)^COPY ` + regexp.QuoteMeta(directory) + `/? ` + regexp.QuoteMeta(directory) + `/?\s*$`
		if !regexp.MustCompile(copyPattern).MatchString(server) {
			t.Errorf("server stage does not copy public package %s", name)
		}
		manifestPath := filepath.Join(directory, "capability-manifest.yaml")
		manifest, err := os.ReadFile(filepath.Join(root, manifestPath))
		if err != nil {
			t.Fatal(err)
		}
		var metadata struct {
			Resources []struct {
				ContentRef string `yaml:"content_ref"`
			} `yaml:"resources"`
		}
		if err := yaml.Unmarshal(manifest, &metadata); err != nil {
			t.Fatal(err)
		}
		for _, resource := range metadata.Resources {
			ref := filepath.Clean(resource.ContentRef)
			if filepath.IsAbs(ref) || ref == ".." || strings.HasPrefix(ref, ".."+string(filepath.Separator)) {
				t.Fatalf("resource escapes public package: %s", ref)
			}
			assertIncluded(t, root, matcher, filepath.Join(directory, ref))
		}
		if err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			assertIncluded(t, root, matcher, relative)
			if entry.Type()&os.ModeSymlink != 0 {
				t.Errorf("public package contains a symlink: %s", relative)
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(data), "-----BEGIN PRIVATE KEY-----") {
				t.Errorf("public package contains private key material: %s", relative)
			}
			if strings.HasSuffix(path, ".md") {
				for _, match := range regexp.MustCompile(`\]\(([^)#]+)(?:#[^)]*)?\)`).FindAllStringSubmatch(string(data), -1) {
					if strings.Contains(match[1], "://") || strings.HasPrefix(match[1], "#") {
						continue
					}
					target := filepath.Clean(filepath.Join(filepath.Dir(path), match[1]))
					ref, err := filepath.Rel(root, target)
					if err != nil {
						return err
					}
					if !strings.HasPrefix(filepath.ToSlash(ref), directory+"/") {
						t.Errorf("documentation dependency escapes package: %s", ref)
					}
					assertIncluded(t, root, matcher, ref)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if regexp.MustCompile(`(?m)^COPY (?:\./)?\.agents/?\s`).MatchString(server) {
		t.Error("server stage must not copy private project agent context wholesale")
	}
}

func TestDockerContextExcludesPrivateAgentAndRuntimeFiles(t *testing.T) {
	matcher := contextMatcher(t, projectRoot(t))
	privatePaths := []string{
		".agents/boot.yaml", ".agents/state.json", ".agents/skills/impeccable/SKILL.md",
		".agent-runtime/state-lite.json", ".env", ".env.production", "config.yaml",
		"deploy/runtime/config.yaml", "deploy/.env.production", "well-ambient.db",
	}
	for _, name := range officialSkills {
		for _, private := range []string{".env", "credentials.json", "runtime.db", ".agent-runtime/state.json", "agents/private-key.pem", "references/private-note.md", "examples/live-response.json"} {
			privatePaths = append(privatePaths, filepath.ToSlash(filepath.Join(".agents", "skills", name, private)))
		}
	}
	for _, path := range privatePaths {
		excluded, err := matcher.MatchesOrParentMatches(path)
		if err != nil {
			t.Fatal(err)
		}
		if !excluded {
			t.Errorf("private path may enter Docker build context: %s", path)
		}
	}
}
