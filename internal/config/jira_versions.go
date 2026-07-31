package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	jiraProjectKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	jiraVersionIDPattern  = regexp.MustCompile(`^[0-9]+$`)
)

// JiraVersionReference is the normalized query identity parsed from a Jira release page URL.
type JiraVersionReference struct {
	ProjectKey   string
	VersionID    string
	CanonicalURL string
}

// ParseJiraVersionURL parses Jira release pages such as
// https://jira.example.com/projects/PROJ/versions/13622 without scraping page HTML.
func ParseJiraVersionURL(baseURL, rawURL string) (JiraVersionReference, error) {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return JiraVersionReference{}, fmt.Errorf("Jira 基础 URL 无效")
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return JiraVersionReference{}, fmt.Errorf("Jira 基础 URL 仅支持 HTTP 或 HTTPS")
	}

	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return JiraVersionReference{}, fmt.Errorf("Jira 版本链接不能为空")
	}
	candidate, err := url.Parse(rawURL)
	if err != nil {
		return JiraVersionReference{}, fmt.Errorf("Jira 版本链接无效")
	}
	if !candidate.IsAbs() {
		resolutionBase := *base
		if !strings.HasSuffix(resolutionBase.Path, "/") {
			resolutionBase.Path += "/"
			resolutionBase.RawPath = ""
		}
		candidate = resolutionBase.ResolveReference(candidate)
	}
	if candidate.Scheme != base.Scheme || !strings.EqualFold(candidate.Host, base.Host) {
		return JiraVersionReference{}, fmt.Errorf("Jira 版本链接必须属于已配置的 Jira 站点")
	}

	basePath := strings.TrimSuffix(base.EscapedPath(), "/")
	candidatePath := candidate.EscapedPath()
	relativePath := candidatePath
	if basePath != "" {
		if !strings.HasPrefix(candidatePath, basePath+"/") {
			return JiraVersionReference{}, fmt.Errorf("Jira 版本链接不在基础 URL 的路径下")
		}
		relativePath = strings.TrimPrefix(candidatePath, basePath)
	}

	segments := strings.Split(strings.Trim(relativePath, "/"), "/")
	if len(segments) != 4 || segments[0] != "projects" || segments[2] != "versions" {
		return JiraVersionReference{}, fmt.Errorf("链接格式应为 /projects/{项目号}/versions/{版本ID}")
	}
	projectKey, err := url.PathUnescape(segments[1])
	if err != nil || !jiraProjectKeyPattern.MatchString(projectKey) {
		return JiraVersionReference{}, fmt.Errorf("版本链接中的 Jira 项目号无效")
	}
	versionID, err := url.PathUnescape(segments[3])
	if err != nil || !jiraVersionIDPattern.MatchString(versionID) || strings.TrimLeft(versionID, "0") == "" {
		return JiraVersionReference{}, fmt.Errorf("版本链接中的 Jira 版本 ID 无效")
	}

	canonical := &url.URL{Scheme: candidate.Scheme, Host: candidate.Host, Path: candidate.Path}
	return JiraVersionReference{
		ProjectKey:   strings.ToUpper(projectKey),
		VersionID:    versionID,
		CanonicalURL: canonical.String(),
	}, nil
}

// NormalizeJiraVersionSources validates version mappings and writes their canonical form back to cfg.
func NormalizeJiraVersionSources(cfg *JiraConfig) error {
	if cfg == nil || len(cfg.VersionSources) == 0 {
		return nil
	}

	normalized := make([]JiraVersionSource, 0, len(cfg.VersionSources))
	seen := make(map[string]struct{}, len(cfg.VersionSources))
	for index, source := range cfg.VersionSources {
		source.ProjectKey = strings.ToUpper(strings.TrimSpace(source.ProjectKey))
		source.ProjectName = strings.TrimSpace(source.ProjectName)
		source.VersionURL = strings.TrimSpace(source.VersionURL)
		if source.ProjectKey == "" && source.ProjectName == "" && source.VersionURL == "" {
			continue
		}
		if source.ProjectName == "" {
			return fmt.Errorf("第 %d 个 Jira 版本来源缺少项目名称", index+1)
		}

		ref, err := ParseJiraVersionURL(cfg.BaseURL, source.VersionURL)
		if err != nil {
			return fmt.Errorf("第 %d 个 Jira 版本来源无效: %w", index+1, err)
		}
		if source.ProjectKey == "" {
			source.ProjectKey = ref.ProjectKey
		}
		if !strings.EqualFold(source.ProjectKey, ref.ProjectKey) {
			return fmt.Errorf("第 %d 个 Jira 版本来源的项目号与链接不一致", index+1)
		}

		identity := ref.ProjectKey + ":" + ref.VersionID
		if _, exists := seen[identity]; exists {
			return fmt.Errorf("第 %d 个 Jira 版本来源与前面的配置重复", index+1)
		}
		seen[identity] = struct{}{}
		source.ProjectKey = ref.ProjectKey
		source.VersionURL = ref.CanonicalURL
		normalized = append(normalized, source)
	}
	cfg.VersionSources = normalized
	return nil
}

// JiraVersionReferences returns only valid, unique version references from an effective config.
func JiraVersionReferences(cfg *JiraConfig) []JiraVersionReference {
	if cfg == nil {
		return nil
	}
	refs := make([]JiraVersionReference, 0, len(cfg.VersionSources))
	seen := make(map[string]struct{}, len(cfg.VersionSources))
	for _, source := range cfg.VersionSources {
		ref, err := ParseJiraVersionURL(cfg.BaseURL, source.VersionURL)
		if err != nil {
			continue
		}
		if key := strings.TrimSpace(source.ProjectKey); key != "" && !strings.EqualFold(key, ref.ProjectKey) {
			continue
		}
		identity := ref.ProjectKey + ":" + ref.VersionID
		if _, exists := seen[identity]; exists {
			continue
		}
		seen[identity] = struct{}{}
		refs = append(refs, ref)
	}
	return refs
}

// JiraProjectKeys returns the deduplicated project scope covered by ordinary and version sources.
func JiraProjectKeys(cfg *JiraConfig) []string {
	if cfg == nil {
		return nil
	}
	keys := make([]string, 0, len(cfg.SyncProjects)+len(cfg.VersionSources))
	seen := make(map[string]struct{}, cap(keys))
	appendKey := func(raw string) {
		key := strings.ToUpper(strings.TrimSpace(raw))
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for _, key := range cfg.SyncProjects {
		appendKey(key)
	}
	for _, ref := range JiraVersionReferences(cfg) {
		appendKey(ref.ProjectKey)
	}
	return keys
}
