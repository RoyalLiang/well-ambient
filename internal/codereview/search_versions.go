package codereview

import (
	"context"
	"encoding/json"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

// Only duplicate-version metadata is read here. Source JSON is decoded in Go
// so malformed legacy records remain distinct on every supported database.
func (s *Service) duplicateMRRunIDs(ctx context.Context, projects []string) ([]uint, error) {
	groups := s.DB.WithContext(ctx).Table("code_review_runs").Select("project_id, ref, head_sha, COALESCE(base_sha, '')").Where("project_id IN ? AND kind = ? AND COALESCE(head_sha, '') <> ''", projects, "mr").Group("project_id, ref, head_sha, COALESCE(base_sha, '')").Having("COUNT(*) > 1")
	rows, err := s.DB.WithContext(ctx).Model(&db.CodeReviewRun{}).Select("id", "project_id", "ref", "head_sha", "base_sha", "repository_source_json").Where("kind = ? AND (project_id, ref, head_sha, COALESCE(base_sha, '')) IN (?)", "mr", groups).Order("id DESC").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hidden := []uint{}
	seen := map[string]bool{}
	for rows.Next() {
		var run db.CodeReviewRun
		if err = s.DB.ScanRows(rows, &run); err != nil {
			return nil, err
		}
		var source config.RepositorySource
		if json.Unmarshal([]byte(run.RepositorySourceJSON), &source) != nil || source.RepositoryKey == "" {
			continue
		}
		key := encode([]string{run.ProjectID, run.Ref, run.HeadSHA, run.BaseSHA, source.RepositoryKey})
		if seen[key] {
			hidden = append(hidden, run.ID)
		} else {
			seen[key] = true
		}
	}
	return hidden, rows.Err()
}
