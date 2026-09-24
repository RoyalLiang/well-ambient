package server

import (
	"net/url"
	"sort"
	"strings"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

type emailCommitDetail struct {
	ID      string `json:"id"`
	URL     string `json:"url,omitempty"`
	Author  string `json:"author"`
	Message string `json:"message"`
}

type emailCommitRow struct {
	Repository string              `json:"repository"`
	Branch     string              `json:"branch"`
	Count      int                 `json:"count"`
	Commits    []emailCommitDetail `json:"commits"`
}

// Branch rows describe observed pushes. A SHA seen on two branches occurs in
// both rows, but the report total remains unique by repository and SHA.
func buildEmailCommitRows(cfg config.Config, commits []db.GitCommitLog) []emailCommitRow {
	rows := []emailCommitRow{}
	indices := map[string]int{}
	seen := map[string]bool{}
	for _, c := range commits {
		branch := strings.TrimPrefix(strings.TrimSpace(c.Branch), "refs/heads/")
		if branch == "" {
			branch = "未记录分支"
		}
		key := c.Repo + "\x00" + branch
		unique := key + "\x00" + c.CommitID
		if seen[unique] {
			continue
		}
		seen[unique] = true
		i, ok := indices[key]
		if !ok {
			i = len(rows)
			indices[key] = i
			rows = append(rows, emailCommitRow{Repository: c.Repo, Branch: branch})
		}
		rows[i].Commits = append(rows[i].Commits, emailCommitDetail{ID: c.CommitID, URL: emailCommitURL(cfg, c.Repo, c.CommitID), Author: c.Author, Message: c.Message})
		rows[i].Count++
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Repository == rows[j].Repository {
			return rows[i].Branch < rows[j].Branch
		}
		return rows[i].Repository < rows[j].Repository
	})
	return rows
}

func emailCommitURL(cfg config.Config, repo, sha string) string {
	base, err := url.Parse(strings.TrimSpace(cfg.GitLab.BaseURL))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || strings.Contains(cfg.GitLab.BaseURL, "#") {
		return ""
	}
	paths := map[string]bool{}
	for _, mapping := range cfg.GitLab.Repos {
		if repo == mapping.Name || repo == mapping.Path {
			path := strings.Trim(strings.TrimSuffix(mapping.Path, ".git"), "/")
			if path != "" {
				paths[path] = true
			}
		}
	}
	// A repository name alone is not sufficient to guess a GitLab namespace.
	if len(paths) != 1 {
		return ""
	}
	var path string
	for value := range paths {
		path = value
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, ":?#\\") {
			return ""
		}
	}
	if sha == "" || strings.ContainsAny(sha, "/?#\\") {
		return ""
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + path + "/-/commit/" + sha
	base.RawPath, base.RawQuery, base.Fragment = "", "", ""
	return base.String()
}
