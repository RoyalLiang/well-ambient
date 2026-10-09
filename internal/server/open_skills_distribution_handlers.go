package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"well-ambient/internal/openaccess"
)

type openSkillSummary struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
	ArchiveURL  string   `json:"archive_url"`
	ManifestURL string   `json:"manifest_url"`
	DocURL      string   `json:"doc_url"`
}

var officialSkills = []struct {
	name        string
	title       string
	description string
	tools       []string
}{
	{
		name:        "jira-analysis",
		title:       "Jira 领域分析与度量",
		description: "基于发布投影与统计口径执行 Jira 深度检索、全量多维聚合与历史审计",
		tools: []string{
			"jira_describe_schema",
			"jira_search_issues",
			"jira_get_issue",
			"jira_aggregate_issues",
		},
	},
	{
		name:        "board-decision",
		title:       "看板排期决策",
		description: "准备并执行受控单事项 Jira 转派或改期计划，保障幂等、租约互斥与远端写后确认",
		tools: []string{
			"decision_get_context",
			"decision_prepare",
			"decision_execute",
			"decision_get_operation",
		},
	},
	{
		name:        "code-review-reader",
		title:       "代码评审读取",
		description: "按 GitLab Project ID 授权检索并读取过滤内部 Prompt、策略与堆栈后的代码评审报告",
		tools: []string{
			"review_search",
			"review_get",
		},
	},
}

func getOfficialSkillsCatalog(baseURL string) []openSkillSummary {
	baseURL = strings.TrimRight(baseURL, "/")
	var list []openSkillSummary
	for _, s := range officialSkills {
		list = append(list, openSkillSummary{
			Name:        s.name,
			Version:     "1.0.0",
			Title:       s.title,
			Description: s.description,
			Tools:       s.tools,
			ArchiveURL:  fmt.Sprintf("%s/open/v1/skills/%s/archive", baseURL, s.name),
			ManifestURL: fmt.Sprintf("%s/open/v1/skills/%s/manifest", baseURL, s.name),
			DocURL:      fmt.Sprintf("%s/open/v1/skills/%s/skill.md", baseURL, s.name),
		})
	}
	return list
}

func (s *Server) handleGetOpenSkillsList(w http.ResponseWriter, r *http.Request) {
	baseURL := s.resolveRequestBaseURL(r)
	catalog := getOfficialSkillsCatalog(baseURL)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(catalog)
}

func (s *Server) handleGetOpenSkillInfo(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	baseURL := s.resolveRequestBaseURL(r)
	catalog := getOfficialSkillsCatalog(baseURL)
	for _, skill := range catalog {
		if skill.Name == name {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(skill)
			return
		}
	}
	http.Error(w, "Skill not found", http.StatusNotFound)
}

func resolveSkillsRoot() string {
	candidates := []string{
		filepath.Join(".agents", "skills"),
		filepath.Join("..", ".agents", "skills"),
		filepath.Join("..", "..", ".agents", "skills"),
	}
	for _, candidate := range candidates {
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return candidate
		}
	}
	return filepath.Join(".agents", "skills")
}

func (s *Server) handleGetOpenSkillDoc(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if !isValidSkillName(name) {
		http.Error(w, "Invalid skill name", http.StatusBadRequest)
		return
	}
	filePath := filepath.Join(resolveSkillsRoot(), name, "SKILL.md")
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Skill documentation not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(content)
}

func (s *Server) handleGetOpenSkillManifest(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if !isValidSkillName(name) {
		http.Error(w, "Invalid skill name", http.StatusBadRequest)
		return
	}
	filePath := filepath.Join(resolveSkillsRoot(), name, "capability-manifest.yaml")
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Skill manifest not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(content)
}

func (s *Server) handleGetOpenSkillArchive(w http.ResponseWriter, r *http.Request) {
	// This stable URL must not retain archive bytes from a previous image.
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	name := strings.TrimSpace(r.PathValue("name"))
	if !isValidSkillName(name) {
		http.Error(w, "Invalid skill name", http.StatusBadRequest)
		return
	}
	skillDir := filepath.Join(resolveSkillsRoot(), name)
	if fi, err := os.Stat(skillDir); err != nil || !fi.IsDir() {
		http.Error(w, "Skill directory not found", http.StatusNotFound)
		return
	}

	// Finish packing before committing HTTP 200. Writing an error to a live
	// gzip stream corrupts the archive and prevents curl -f detecting failure.
	var archive bytes.Buffer
	gw := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gw)

	err := filepath.Walk(skillDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(skillDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		header.Name = filepath.Join(name, rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tw, file)
		return err
	})
	if err == nil {
		err = tw.Close()
	}
	if err == nil {
		err = gw.Close()
	}
	if err != nil {
		log.Printf("Failed to pack skill %q: %v", name, err)
		http.Error(w, "Failed to pack skill", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+"-1.0.0.tar.gz"))
	w.Header().Set("Content-Length", fmt.Sprint(archive.Len()))
	_, _ = w.Write(archive.Bytes())
}

func (s *Server) handleGetOpenInstallScript(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		writeOpenError(w, "", openaccess.NewError("invalid_request", "installer URLs must not contain query parameters or credentials; configure Authorization locally"))
		return
	}
	baseURL := s.resolveRequestBaseURL(r)
	script := renderInstallScript(baseURL)

	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	_, _ = w.Write([]byte(script))
}

func (s *Server) resolveRequestBaseURL(r *http.Request) string {
	if s != nil && s.config != nil && strings.TrimSpace(s.config.Server.PublicURL) != "" {
		return strings.TrimRight(strings.TrimSpace(s.config.Server.PublicURL), "/")
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		port := 8080
		if s != nil && s.config != nil && s.config.Server.Port > 0 {
			port = s.config.Server.Port
		}
		host = fmt.Sprintf("localhost:%d", port)
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

func isValidSkillName(name string) bool {
	for _, s := range officialSkills {
		if s.name == name {
			return true
		}
	}
	return false
}

//go:embed open_skills_install.sh
var openSkillsInstallScript string

//go:embed open_skills_configure_codex.py
var openSkillsCodexSetup string

func renderInstallScript(baseURL string) string {
	quotedURL := "'" + strings.ReplaceAll(baseURL, "'", "'\"'\"'") + "'"
	script := strings.Replace(openSkillsInstallScript, "__WELL_AMBIENT_BASE_URL__", quotedURL, 1)
	return strings.Replace(script, "__WELL_AMBIENT_CODEX_SETUP__", openSkillsCodexSetup, 1)
}
