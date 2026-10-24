package codereview

import (
	"context"
	"encoding/json"
	"time"
	"well-ambient/internal/db"
)

// ExportDocument enriches only the filename of legacy reports that did not
// freeze source_branch. It never changes their stored evidence or result.
func (s *Service) ExportDocument(ctx context.Context, run db.CodeReviewRun) (ReviewDocument, error) {
	var snapshot Snapshot
	if run.Kind == "mr" && json.Unmarshal([]byte(run.SnapshotJSON), &snapshot) == nil && snapshot.SourceBranch == "" {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		repo, repoErr := s.Repo(run.ProjectID)
		source := s.Config().GitLab.Source(repo, time.Now())
		if repoErr == nil && runSourceMatches(run, source) {
			if current, err := s.git().mr(ctx, run.ProjectID, run.Ref); err == nil && (run.HeadSHA == "" || mrVersion(current).Head == run.HeadSHA) && (run.BaseSHA == "" || mrVersion(current).Base == run.BaseSHA) {
				snapshot.SourceBranch = current.SourceBranch
				run.SnapshotJSON = encode(snapshot)
			}
		}
	}
	return BuildDocument(run)
}
