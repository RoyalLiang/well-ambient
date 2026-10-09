package server

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
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

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+"-1.0.0.tar.gz"))
	w.Header().Set("Cache-Control", "public, max-age=300")

	gw := gzip.NewWriter(w)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

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
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to pack skill: %v", err), http.StatusInternalServerError)
	}
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

func renderInstallScript(baseURL string) string {
	return fmt.Sprintf(`#!/usr/bin/env bash
# ==============================================================================
# well-ambient Open Capabilities & Skills Online Installer
# Downloads official skills and shows a secret-free local MCP configuration
# ==============================================================================
set -euo pipefail

BASE_URL=%s

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}=== well-ambient 开放能力与 Skill 在线安装器 ===${NC}"
echo -e "服务端地址: ${BASE_URL}"

# 1. 检查连通性
echo -ne "正在探测服务端健康状态... "
if curl -fsSL "${BASE_URL}/health" >/dev/null 2>&1; then
    echo -e "${GREEN}正常${NC}"
else
    echo -e "${YELLOW}警告: 服务端 /health 无法连接，请确认网络或服务端是否在线。${NC}"
fi

# 2. 准备 Skill 存储目标目录
SKILLS_DIR=""
if [ -d ".agents/skills" ]; then
    SKILLS_DIR=".agents/skills"
elif [ -d "$HOME/.codex/skills" ]; then
    SKILLS_DIR="$HOME/.codex/skills"
else
    mkdir -p "$HOME/.codex/skills"
    SKILLS_DIR="$HOME/.codex/skills"
fi

echo -e "微内核技能安装目标目录: ${BLUE}${SKILLS_DIR}${NC}"

# 3. 在线下载并解压官方 Skill 包
SKILLS=("jira-analysis" "board-decision" "code-review-reader")
INSTALLED_SKILLS=()
FAILED_SKILLS=()
for skill in "${SKILLS[@]}"; do
    echo -ne "正在下载并安装 Skill [${skill}]... "
    TMP_FILE=$(mktemp)
    if curl -fsSL "${BASE_URL}/open/v1/skills/${skill}/archive" -o "${TMP_FILE}" 2>/dev/null &&
       tar -xzf "${TMP_FILE}" -C "${SKILLS_DIR}"; then
        INSTALLED_SKILLS+=("${skill}")
        echo -e "${GREEN}完成${NC}"
    else
        FAILED_SKILLS+=("${skill}")
        echo -e "${RED}失败 (请检查网络或技能包)${NC}"
    fi
    rm -f "${TMP_FILE}"
done
if [ "${#FAILED_SKILLS[@]}" -gt 0 ]; then
    echo -e "已成功安装: ${INSTALLED_SKILLS[*]:-无}"
    echo -e "${RED}安装失败: ${FAILED_SKILLS[*]}${NC}"
    echo -e "请解决失败原因后重新运行安装命令。"
    exit 1
fi

# 4. 配置 MCP 客户端
MCP_URL="${BASE_URL}/mcp"

# A. Claude Desktop 配置检查
CLAUDE_CONFIG_DIR=""
if [[ "$OSTYPE" == "darwin"* ]]; then
    CLAUDE_CONFIG_DIR="$HOME/Library/Application Support/Claude"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    CLAUDE_CONFIG_DIR="$HOME/.config/Claude"
fi

if [ -n "${CLAUDE_CONFIG_DIR}" ] && [ -d "${CLAUDE_CONFIG_DIR}" ]; then
    CLAUDE_CONFIG_FILE="${CLAUDE_CONFIG_DIR}/claude_desktop_config.json"
    echo -e "检测到 Claude Desktop 安装环境: ${BLUE}${CLAUDE_CONFIG_FILE}${NC}"
fi

# B. Cursor 检查
if [ -d "$HOME/.cursor" ] || [ -d ".cursor" ]; then
    echo -e "检测到 Cursor 编辑器工作环境"
fi

echo ""
echo -e "${GREEN}✓ 技能安装完成！${NC}"
echo -e "目标目录: ${BLUE}${SKILLS_DIR}${NC}"
echo -e "已成功安装: ${INSTALLED_SKILLS[*]}"
echo ""
echo -e "${BLUE}=== 客户端接入配置指引 ===${NC}"
echo -e "请在客户端本地配置 Authorization，将 <YOUR_API_KEY> 替换为真实凭证。"
echo -e "不要将 API Key 放入下载地址、命令 URL 或客户端安装深链。"

echo ""
echo -e "1. 【Cursor / VS Code (Streamable HTTP 模式)】:"
echo -e "在 .cursor/mcp.json 或 settings 中添加:"
cat <<EOF
{
  "mcpServers": {
    "well-ambient": {
      "url": "${MCP_URL}",
      "headers": {
        "Authorization": "Bearer <YOUR_API_KEY>"
      }
    }
  }
}
EOF

echo ""
echo -e "2. 【本地凭证配置】:"
echo -e "在 Cursor 或其他 MCP 客户端保存上面的配置模板后，本地填写 Authorization。"
echo ""
echo -e "${GREEN}所有就绪！欢迎使用 well-ambient 开放能力。${NC}"
`, "'"+strings.ReplaceAll(baseURL, "'", "'\"'\"'")+"'")
}
