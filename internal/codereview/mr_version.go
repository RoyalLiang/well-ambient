package codereview

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
)

const skippedMRVersionPhase = "已跳过：MR 提交已更新"

type mrCodeVersion struct{ Head, Base string }

func mrVersion(m mr) mrCodeVersion {
	head := m.DiffRefs.Head
	if head == "" {
		head = m.SHA
	}
	return mrCodeVersion{Head: head, Base: m.DiffRefs.Base}
}

// Automatic deliveries identify the frozen diff, rather than MR metadata updates.
func (s *Service) enqueueMRVersion(ctx context.Context, project, ref string, version mrCodeVersion) (db.CodeReviewRun, error) {
	version.Head = strings.ToLower(strings.TrimSpace(version.Head))
	version.Base = strings.ToLower(strings.TrimSpace(version.Base))
	if !shaPattern.MatchString(version.Head) || !shaPattern.MatchString(version.Base) {
		return db.CodeReviewRun{}, errors.New("MR 事件缺少可验证的基线或目标 SHA，未创建评审")
	}
	repo, err := s.Repo(project)
	if err != nil {
		return db.CodeReviewRun{}, err
	}
	source := s.Config().GitLab.Source(repo, time.Now())
	var existing []db.CodeReviewRun
	err = s.DB.WithContext(ctx).Where("project_id = ? AND kind = ? AND ref = ? AND head_sha = ? AND base_sha = ?", project, "mr", ref, version.Head, version.Base).Order("id DESC").Find(&existing).Error
	if err != nil {
		return db.CodeReviewRun{}, err
	}
	for _, run := range existing {
		if !runSourceMatches(run, source) {
			continue
		}
		if run.Status == "cancelled" && run.Phase == skippedMRVersionPhase {
			// A force-push can make a previously skipped diff current again. Only an
			// automatic skip is revived; explicit user cancellation remains respected.
			policy, err := s.Policy(project)
			if err != nil {
				return run, err
			}
			global, err := s.CommentSettings(ctx)
			if err != nil {
				return run, err
			}
			if !global.ReviewMRs || !*policy.ReviewMRs {
				return run, nil
			}
			err = s.DB.WithContext(ctx).Model(&db.CodeReviewRun{}).Where("id = ? AND status = ? AND phase = ?", run.ID, "cancelled", skippedMRVersionPhase).Updates(map[string]any{"status": "queued", "phase": "等待执行", "error": "", "publish_error": "", "publish_status": "waiting_review", "updated_at": time.Now()}).Error
			if err != nil {
				return run, err
			}
			return s.Get(run.ID)
		}
		return run, nil
	}
	return s.enqueue(ctx, project, "mr", ref, "version:"+version.Base+":"+version.Head, &version)
}
func runSourceMatches(run db.CodeReviewRun, source config.RepositorySource) bool {
	var frozen config.RepositorySource
	if json.Unmarshal([]byte(run.RepositorySourceJSON), &frozen) == nil && frozen.RepositoryKey != "" {
		return frozen.RepositoryKey == source.RepositoryKey
	}
	return strings.TrimSpace(run.RepositorySourceJSON) == "" && source.LegacyDedupe
}
func versionBoundMR(run db.CodeReviewRun) bool {
	if run.Kind != "mr" || !shaPattern.MatchString(run.HeadSHA) || !shaPattern.MatchString(run.BaseSHA) {
		return false
	}
	identity := run.ProjectID
	var source config.RepositorySource
	if json.Unmarshal([]byte(run.RepositorySourceJSON), &source) == nil && source.RepositoryKey != "" && !source.LegacyDedupe {
		identity = source.RepositoryKey
	}
	return run.Key == digest(identity+":mr:"+run.Ref+":version:"+run.BaseSHA+":"+run.HeadSHA)
}
