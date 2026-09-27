package agentruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
	"well-ambient/internal/openmcp"
)

func TestOpenCapabilityPackagesAreDiscoverableAndValid(t *testing.T) {
	root := filepath.Join("..", "..")
	skills := []struct {
		dir  string
		name string
		id   string
	}{
		{".agents/skills/jira-analysis", "jira-analysis", "well-ambient.jira-analysis"},
		{".agents/skills/board-decision", "board-decision", "well-ambient.board-decision"},
		{".agents/skills/code-review-reader", "code-review-reader", "well-ambient.code-review-reader"},
	}
	for _, skill := range skills {
		t.Run(skill.name, func(t *testing.T) {
			directory := filepath.Join(root, skill.dir)
			skillData, err := os.ReadFile(filepath.Join(directory, "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			var frontmatter struct {
				Name        string `yaml:"name"`
				Description string `yaml:"description"`
			}
			var document string
			if _, err := splitSkillFrontmatter(string(skillData), &frontmatter, &document); err != nil {
				t.Fatal(err)
			}
			if frontmatter.Name != skill.name || frontmatter.Description == "" || document == "" {
				t.Fatalf("invalid skill frontmatter: %+v", frontmatter)
			}
			manifestData, err := os.ReadFile(filepath.Join(directory, "capability-manifest.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			manifest, err := ParseManifest(manifestData)
			if err != nil {
				t.Fatal(err)
			}
			if manifest.ID != skill.id || manifest.Kind != KindSkill || manifest.Digest == "" {
				t.Fatalf("invalid manifest: %+v", manifest)
			}
			assertManifestResourcesExist(t, directory, manifest.Resources)
		})
	}

	mcpDirectory := filepath.Join(root, "capabilities/open/mcp/open-capabilities-mcp")
	manifestData, err := os.ReadFile(filepath.Join(mcpDirectory, "capability-manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := ParseManifest(manifestData)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "well-ambient.open-capabilities-mcp" || manifest.Kind != KindMCP {
		t.Fatalf("invalid MCP manifest: %+v", manifest)
	}
	if len(manifest.Tools) != len(openmcp.ToolNames) {
		t.Fatalf("MCP manifest tools = %d, want %d", len(manifest.Tools), len(openmcp.ToolNames))
	}
	for index, name := range openmcp.ToolNames {
		if manifest.Tools[index] != name {
			t.Fatalf("MCP tool[%d] = %q, want %q", index, manifest.Tools[index], name)
		}
	}
	assertManifestResourcesExist(t, mcpDirectory, manifest.Resources)
}

func TestOpenCapabilityContractsParse(t *testing.T) {
	root := filepath.Join("..", "..", "capabilities", "open", "contracts")
	for _, filename := range []string{"tool-catalog.json", "errors.schema.json"} {
		data, err := os.ReadFile(filepath.Join(root, filename))
		if err != nil {
			t.Fatal(err)
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatalf("%s: %v", filename, err)
		}
	}
	openAPI, err := os.ReadFile(filepath.Join(root, "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(openAPI, &document); err != nil {
		t.Fatal(err)
	}
	if document["openapi"] != "3.1.0" {
		t.Fatalf("OpenAPI version = %#v", document["openapi"])
	}
}

func splitSkillFrontmatter(source string, frontmatter any, document *string) (string, error) {
	const delimiter = "---\n"
	if len(source) < len(delimiter) || source[:len(delimiter)] != delimiter {
		return "", os.ErrInvalid
	}
	rest := source[len(delimiter):]
	index := -1
	for offset := 0; offset+len(delimiter) <= len(rest); offset++ {
		if rest[offset:offset+len(delimiter)] == delimiter {
			index = offset
			break
		}
	}
	if index < 0 {
		return "", os.ErrInvalid
	}
	header := rest[:index]
	if err := yaml.Unmarshal([]byte(header), frontmatter); err != nil {
		return "", err
	}
	*document = rest[index+len(delimiter):]
	return header, nil
}

func assertManifestResourcesExist(t *testing.T, directory string, resources []ResourceDef) {
	t.Helper()
	for _, resource := range resources {
		if resource.ContentRef == "" {
			continue
		}
		if _, err := os.Stat(filepath.Clean(filepath.Join(directory, resource.ContentRef))); err != nil {
			t.Fatalf("resource %s (%s): %v", resource.Key, resource.ContentRef, err)
		}
	}
}
