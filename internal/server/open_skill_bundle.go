package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
	"well-ambient/internal/opennames"
)

const openSkillBundleName = "well-ambient"

func (s *Server) publishedSubskills() map[string]bool {
	enabled := map[string]bool{}
	for _, child := range officialSkills {
		enabled[child.name] = true
	}
	if s == nil {
		return enabled
	}
	s.configMutationMu.Lock()
	defer s.configMutationMu.Unlock()
	if s.config == nil {
		return enabled
	}
	for _, value := range s.config.OpenCapabilities.DisabledSkills {
		if value == openSkillBundleName {
			return map[string]bool{}
		}
		if child, ok := canonicalSubskill(value); ok {
			delete(enabled, child)
		}
	}
	return enabled
}

func canonicalSubskill(value string) (string, bool) {
	value = strings.TrimSpace(value)
	for _, skill := range officialSkills {
		if value == skill.name || value == openSkillBundleName+"/"+skill.name || strings.HasSuffix(value, "-"+skill.name) {
			return skill.name, true
		}
	}
	return "", false
}

func bundleCatalog(baseURL string) []openSkillSummary {
	baseURL = strings.TrimRight(baseURL, "/")
	root := baseURL + "/open/v1/skills/" + openSkillBundleName
	parent := openSkillSummary{Enabled: true, CanonicalName: openSkillBundleName, Name: openSkillBundleName, Version: "2.0.0", Title: "well-ambient", Description: "统一的需求背景、Jira 分析、受控看板决策与评审证据技能；按任务加载包内子技能。", ArchiveURL: root + "/archive", ManifestURL: root + "/manifest", DocURL: root + "/skill.md", Tools: []string{}, Children: []openSkillSummary{}}
	for _, skill := range officialSkills {
		tools := make([]string, 0, len(skill.tools))
		for _, tool := range skill.tools {
			tools = append(tools, opennames.Name(opennames.DefaultPrefix, tool))
		}
		parent.Children = append(parent.Children, openSkillSummary{Enabled: true, CanonicalName: skill.name, Name: openSkillBundleName + "/" + skill.name, Parent: openSkillBundleName, Version: "1", Title: skill.title, Description: skill.description, Tools: tools, ArchiveURL: parent.ArchiveURL, ManifestURL: parent.ManifestURL, DocURL: root + "/subskills/" + skill.name + "/skill.md"})
		parent.Tools = append(parent.Tools, tools...)
	}
	return []openSkillSummary{parent}
}

func (s *Server) bundleDocument(snapshot ...map[string]bool) ([]byte, error) {
	enabled := s.publishedSubskills()
	if len(snapshot) > 0 {
		enabled = snapshot[0]
	}
	raw, err := os.ReadFile(filepath.Join(resolveSkillsRoot(), openSkillBundleName, "SKILL.md"))
	if err != nil {
		return nil, err
	}
	lines := []string{}
	for _, line := range strings.Split(string(raw), "\n") {
		keep := true
		for _, child := range officialSkills {
			if strings.Contains(line, "../"+child.name+"/") && !enabled[child.name] {
				keep = false
			}
			line = strings.ReplaceAll(line, "../"+child.name+"/SKILL.md", "modules/"+child.name+"/WORKFLOW.md")
		}
		if keep {
			lines = append(lines, line)
		}
	}
	return s.renderOpenSkillContent([]byte(strings.Join(lines, "\n")), opennames.DefaultPrefix), nil
}

func (s *Server) bundleManifest(snapshot ...map[string]bool) ([]byte, error) {
	enabled := s.publishedSubskills()
	if len(snapshot) > 0 {
		enabled = snapshot[0]
	}
	raw, err := os.ReadFile(filepath.Join(resolveSkillsRoot(), openSkillBundleName, "capability-manifest.yaml"))
	if err != nil {
		return nil, err
	}
	var manifest map[string]any
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	tools := []string{}
	resources := []map[string]any{{"key": "router", "load_level": "L0", "content_ref": "SKILL.md", "content_kind": "markdown", "token_estimate": 700}}
	for _, child := range officialSkills {
		if enabled[child.name] {
			for _, tool := range child.tools {
				tools = append(tools, opennames.Name(opennames.DefaultPrefix, tool))
			}
			resources = append(resources, map[string]any{"key": child.name, "load_level": "L1", "content_ref": "modules/" + child.name + "/WORKFLOW.md", "content_kind": "markdown", "token_estimate": 1200})
		}
	}
	manifest["tools"] = tools
	manifest["resources"] = resources
	return yaml.Marshal(manifest)
}

func (s *Server) serveBundleDocument(w http.ResponseWriter, r *http.Request, manifest bool) {
	if _, ok := s.resolveOpenSkillName(r.PathValue("name"), opennames.DefaultPrefix); !ok {
		http.Error(w, "Invalid skill name", http.StatusBadRequest)
		return
	}
	var raw []byte
	var err error
	if manifest {
		raw, err = s.bundleManifest()
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	} else {
		raw, err = s.bundleDocument()
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	}
	if err != nil {
		http.Error(w, "Skill package source unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

func (s *Server) handleGetOpenSubskillDoc(w http.ResponseWriter, r *http.Request) {
	if r.PathValue("name") != openSkillBundleName {
		http.NotFound(w, r)
		return
	}
	name := r.PathValue("subskill")
	canonical, ok := canonicalSubskill(name)
	if !ok || name != canonical {
		http.NotFound(w, r)
		return
	}
	raw, err := os.ReadFile(filepath.Join(resolveSkillsRoot(), canonical, "SKILL.md"))
	if err != nil {
		http.Error(w, "Subskill source unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(s.renderOpenSkillContent(stripSkillFrontmatter(raw), opennames.DefaultPrefix))
}

func stripSkillFrontmatter(raw []byte) []byte {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if strings.HasPrefix(text, "---\n") {
		if end := strings.Index(text[4:], "\n---\n"); end >= 0 {
			return []byte(strings.TrimLeft(text[4+end+5:], "\n"))
		}
	}
	return []byte(text)
}

func (s *Server) serveBundleArchive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if _, ok := s.resolveOpenSkillName(r.PathValue("name"), opennames.DefaultPrefix); !ok {
		http.Error(w, "Invalid skill name", http.StatusBadRequest)
		return
	}
	enabled := s.publishedSubskills()
	if len(enabled) == 0 {
		http.Error(w, "No published subskills", http.StatusForbidden)
		return
	}
	// Resolve all source files before committing HTTP 200 so clients can safely
	// use curl -f, and reject non-regular files rather than following symlinks.
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(gz)
	write := func(name string, raw []byte) error {
		if err := archive.WriteHeader(&tar.Header{Name: openSkillBundleName + "/" + filepath.ToSlash(name), Mode: 0644, Size: int64(len(raw))}); err != nil {
			return err
		}
		_, err := archive.Write(raw)
		return err
	}
	doc, err := s.bundleDocument(enabled)
	if err == nil {
		err = write("SKILL.md", doc)
	}
	if err == nil {
		var manifest []byte
		manifest, err = s.bundleManifest(enabled)
		if err == nil {
			err = write("capability-manifest.yaml", manifest)
		}
	}
	for _, child := range officialSkills {
		if err != nil {
			break
		}
		if !enabled[child.name] {
			continue
		}
		root := filepath.Join(resolveSkillsRoot(), child.name)
		err = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("non-regular skill source")
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if rel == "SKILL.md" {
				raw = stripSkillFrontmatter(raw)
				rel = "WORKFLOW.md"
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".md", ".yaml", ".yml", ".json":
				raw = s.renderOpenSkillContent(raw, opennames.DefaultPrefix)
				if filepath.Base(path) == "capability-manifest.yaml" {
					raw = bytes.ReplaceAll(raw, []byte("SKILL.md"), []byte("WORKFLOW.md"))
				}
			}
			return write(filepath.Join("modules", child.name, rel), raw)
		})
	}
	if err == nil {
		err = archive.Close()
	}
	if err == nil {
		err = gz.Close()
	}
	if err != nil {
		http.Error(w, "Skill package source unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="well-ambient-2.0.0.tar.gz"`)
	_, _ = w.Write(buffer.Bytes())
}

func renderBundleTools(raw []byte) []byte {
	text := string(raw)
	for _, skill := range officialSkills {
		for _, tool := range skill.tools {
			pattern := regexp.MustCompile(`\b(?:well-ambient-|well-)?` + regexp.QuoteMeta(tool) + `\b`)
			text = pattern.ReplaceAllString(text, opennames.Name(opennames.DefaultPrefix, tool))
		}
	}
	return []byte(text)
}
