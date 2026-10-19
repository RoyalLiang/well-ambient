package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"well-ambient/internal/openaccess"
	"well-ambient/internal/opennames"
)

type openSkillSummary struct {
	Enabled       bool               `json:"enabled"`
	CanonicalName string             `json:"canonical_name"`
	Name          string             `json:"name"`
	Version       string             `json:"version"`
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	Tools         []string           `json:"tools"`
	ArchiveURL    string             `json:"archive_url"`
	ManifestURL   string             `json:"manifest_url"`
	DocURL        string             `json:"doc_url"`
	Parent        string             `json:"parent,omitempty"`
	Children      []openSkillSummary `json:"children,omitempty"`
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
	{
		name:        "runtime-knowledge",
		title:       "需求背景与系统知识",
		description: "从定时更新的授权知识快照获取需求背景、能力边界和场景证据，支持预算压缩与来源展开",
		tools:       []string{"knowledge_describe", "knowledge_get_context", "knowledge_get_evidence"},
	},
}

func getOfficialSkillsCatalog(baseURL string, prefixes ...string) []openSkillSummary {
	return bundleCatalog(baseURL)
}

func (s *Server) handleGetOpenSkillsList(w http.ResponseWriter, r *http.Request) {
	baseURL := s.resolveRequestBaseURL(r)
	catalog := s.openSkillsCatalog(baseURL)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(catalog)
}

func (s *Server) handleGetOpenSkillInfo(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if _,ok:=s.resolveOpenSkillName(name,opennames.DefaultPrefix);!ok{http.Error(w,"Skill not found",http.StatusNotFound);return}
	baseURL := s.resolveRequestBaseURL(r)
	catalog := s.openSkillsCatalog(baseURL)
	for _, skill := range catalog {
		if skill.Name == openSkillBundleName {
			if name!=openSkillBundleName{w.Header().Set("Deprecation","true");w.Header().Set("Content-Location","/open/v1/skills/well-ambient")}
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
	s.serveBundleDocument(w, r, false)
}

func (s *Server) handleGetOpenSkillManifest(w http.ResponseWriter, r *http.Request) {
	s.serveBundleDocument(w, r, true)
}

func (s *Server) handleGetOpenSkillArchive(w http.ResponseWriter, r *http.Request) {
	s.serveBundleArchive(w, r)
}

func (s *Server) handleGetOpenInstallScript(w http.ResponseWriter, r *http.Request) {
	if r.URL.RawQuery != "" {
		writeOpenError(w, "", openaccess.NewError("invalid_request", "installer URLs must not contain query parameters or credentials; configure Authorization locally"))
		return
	}
	baseURL := s.resolveRequestBaseURL(r)
	catalog := s.openSkillsCatalog(baseURL)
	names := make([]string, 0, len(catalog))
	for _, skill := range catalog {
		if skill.Enabled {
			names = append(names, skill.Name)
		}
	}
	// Keep the namespace and enabled list from the same configuration snapshot.
	prefix := strings.TrimSuffix(catalog[0].Name, "-"+catalog[0].CanonicalName)
	script := renderInstallScriptForSkills(baseURL, prefix, names)

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

//go:embed open_skills_install.sh
var openSkillsInstallScript string

//go:embed open_skills_configure_codex.py
var openSkillsCodexSetup string

func renderInstallScript(baseURL string, prefixes ...string) string {
	return renderInstallScriptForSkills(baseURL, opennames.DefaultPrefix, []string{openSkillBundleName})
}

func renderInstallScriptForSkills(baseURL, prefix string, names []string) string {
	quotedURL := "'" + strings.ReplaceAll(baseURL, "'", "'\"'\"'") + "'"
	script := strings.NewReplacer("__WELL_AMBIENT_NAME_PREFIX__", prefix, "__WELL_AMBIENT_BASE_URL__", quotedURL, "__WELL_AMBIENT_ENABLED_SKILLS__", strings.Join(names, " ")).Replace(openSkillsInstallScript)
	return strings.Replace(script, "__WELL_AMBIENT_CODEX_SETUP__", openSkillsCodexSetup, 1)
}

func (s *Server) openSkillEnabled(name string) bool {
	enabled := s.publishedSubskills()
	if name == openSkillBundleName {
		return len(enabled) > 0
	}
	canonical, ok := canonicalSubskill(name)
	return ok && enabled[canonical]
}

func (s *Server) openSkillsCatalog(baseURL string) []openSkillSummary {
	catalog := bundleCatalog(baseURL)
	enabled := s.publishedSubskills()
	catalog[0].Enabled = len(enabled) > 0
	catalog[0].Tools = []string{}
	for i := range catalog[0].Children {
		child := &catalog[0].Children[i]
		child.Enabled = enabled[child.CanonicalName]
		if child.Enabled {
			catalog[0].Tools = append(catalog[0].Tools, child.Tools...)
		}
	}
	return catalog
}

func (s *Server) openNamePrefix() string { return opennames.DefaultPrefix }

func (s *Server) resolveOpenSkillName(value string, prefix string) (string, bool) {
	if strings.TrimSpace(value) == openSkillBundleName {
		return openSkillBundleName, true
	}
	if _, ok := canonicalSubskill(value); ok {
		return openSkillBundleName, true
	}
	return "", false
}

func (s *Server) renderOpenSkillContent(content []byte, prefix string) []byte {
	return renderBundleTools(content)
}
