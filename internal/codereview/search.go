package codereview

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"well-ambient/internal/db"
)

type RunFilter struct {
	Project, Kind, Query string
	Limit, Offset        int
}

type RunPage struct {
	Items   []db.CodeReviewRun `json:"items"`
	Total   int64              `json:"total"`
	HasMore bool               `json:"has_more"`
}

var mrSearch = regexp.MustCompile(`(?i)^(?:mr\s*!?\s*|!)([0-9]+)$`)
var commitSearch = regexp.MustCompile(`(?i)^commit\s+([0-9a-f]{7,64})$`)

func (s *Service) Search(ctx context.Context, f RunFilter) (RunPage, error) {
	page := RunPage{Items: []db.CodeReviewRun{}}
	if f.Kind != "" && f.Kind != "mr" && f.Kind != "commit" {
		return page, errors.New("评审类型必须为 mr 或 commit")
	}
	q := strings.TrimSpace(f.Query)
	if utf8.RuneCountInString(q) > 200 || f.Offset < 0 {
		return page, errors.New("搜索不超过200字，分页位置不能为负数")
	}
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	ids := []string{}
	for _, repo := range s.Config().GitLab.Repos {
		if repo.ProjectID != "" {
			ids = append(ids, repo.ProjectID)
		}
	}
	if f.Project != "" {
		if _, err := s.Repo(f.Project); err != nil {
			return page, err
		}
		ids = []string{f.Project}
	}
	query := s.DB.WithContext(ctx).Model(&db.CodeReviewRun{}).Where("project_id IN ?", ids)
	// Keep history addressable by ID, while pagination shows the latest report
	// for each frozen repository and MR code version.
	if f.Kind != "commit" {
		hidden, err := s.duplicateMRRunIDs(ctx, ids)
		if err != nil {
			return page, err
		}
		for start := 0; start < len(hidden); start += 400 {
			end := start + 400
			if end > len(hidden) {
				end = len(hidden)
			}
			query = query.Where("id NOT IN ?", hidden[start:end])
		}
	}
	if f.Kind != "" {
		query = query.Where("kind = ?", f.Kind)
	}
	if mr := mrSearch.FindStringSubmatch(q); mr != nil {
		query = query.Where("kind = ? AND ref = ?", "mr", mr[1])
	} else if q != "" {
		if commit := commitSearch.FindStringSubmatch(q); commit != nil {
			q = commit[1]
			query = query.Where("kind = ?", "commit")
		}
		literal := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(q))
		pattern := "%" + literal + "%"
		query = query.Where("(LOWER(title) LIKE ? ESCAPE '!' OR LOWER(ref) LIKE ? ESCAPE '!' OR LOWER(repo) LIKE ? ESCAPE '!' OR LOWER(author) LIKE ? ESCAPE '!' OR LOWER(head_sha) LIKE ? ESCAPE '!' OR LOWER(base_sha) LIKE ? ESCAPE '!')", pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if err := query.Count(&page.Total).Error; err != nil {
		return page, err
	}
	err := query.Select("id", "repository_source_json", "project_id", "repo", "author", "kind", "ref", "head_sha", "base_sha", "title", "url", "status", "phase", "error", "model", "prompt_version", "skill_version_id", "skill_version", "skill_scope_type", "skill_scope_id", "skill_name", "skill_hash", "retry_of_id", "publish_status", "publish_error", "comment_id", "published_at", "created_at", "updated_at").Order("created_at DESC, id DESC").Limit(f.Limit).Offset(f.Offset).Find(&page.Items).Error
	page.HasMore = int64(f.Offset+len(page.Items)) < page.Total
	return page, err
}
