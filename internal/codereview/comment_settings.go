package codereview

import (
	"context"
	"errors"
	"well-ambient/internal/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// An empty project ID cannot identify a configured repository. The existing
// policy table therefore holds these global gates without a schema migration.
const globalCommentPolicyProjectID = ""
const SystemCorpusScope = "system"

type CommentSettings struct {
	ReviewCommits bool `json:"review_commits"`
	ReviewMRs     bool `json:"review_mrs"`
	SyncCommits   bool `json:"sync_commits"`
	SyncMRs       bool `json:"sync_mrs"`
}

func SystemCorpusPolicy(p db.CodeReviewPolicy) db.CodeReviewPolicy {
	p.AutoReview = true
	if p.ReviewCommits == nil {
		value := true
		p.ReviewCommits = &value
	}
	if p.ReviewMRs == nil {
		value := true
		p.ReviewMRs = &value
	}
	p.Domain = "general"
	p.Scenario = "general"
	p.KnowledgeScope = SystemCorpusScope
	return p
}

func (s *Service) CommentSettings(ctx context.Context) (CommentSettings, error) {
	p := db.CodeReviewPolicy{ProjectID: globalCommentPolicyProjectID, SyncCommits: true, SyncMRs: true}
	err := s.DB.WithContext(ctx).Where("project_id = ?", globalCommentPolicyProjectID).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	p = SystemCorpusPolicy(p)
	return CommentSettings{SyncCommits: p.SyncCommits, SyncMRs: p.SyncMRs, ReviewCommits: *p.ReviewCommits, ReviewMRs: *p.ReviewMRs}, err
}

func (s *Service) PatchCommentSettings(ctx context.Context, commits, mrs *bool, reviews ...*bool) (CommentSettings, error) {
	unlock := LockPolicy(globalCommentPolicyProjectID)
	defer unlock()
	settings, err := s.CommentSettings(ctx)
	if err != nil {
		return settings, err
	}
	if commits != nil {
		settings.SyncCommits = *commits
	}
	if mrs != nil {
		settings.SyncMRs = *mrs
	}
	if len(reviews) > 0 && reviews[0] != nil {
		settings.ReviewCommits = *reviews[0]
	}
	if len(reviews) > 1 && reviews[1] != nil {
		settings.ReviewMRs = *reviews[1]
	}
	p := SystemCorpusPolicy(db.CodeReviewPolicy{
		ReviewCommits: &settings.ReviewCommits, ReviewMRs: &settings.ReviewMRs,
		ProjectID: globalCommentPolicyProjectID, SyncCommits: settings.SyncCommits, SyncMRs: settings.SyncMRs,
	})
	// Updating only supplied columns also preserves simultaneous partial edits
	// from another server process; omitted gates must never be overwritten.
	columns := []string{s.DB.NamingStrategy.ColumnName("", "UpdatedAt")}
	if len(reviews) > 0 && reviews[0] != nil {
		columns = append(columns, s.DB.NamingStrategy.ColumnName("", "ReviewCommits"))
	}
	if len(reviews) > 1 && reviews[1] != nil {
		columns = append(columns, db.CodeReviewMRsColumn)
	}
	if commits != nil {
		columns = append(columns, s.DB.NamingStrategy.ColumnName("", "SyncCommits"))
	}
	if mrs != nil {
		columns = append(columns, s.DB.NamingStrategy.ColumnName("", "SyncMRs"))
	}
	err = s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "project_id"}},
		DoUpdates: clause.AssignmentColumns(columns),
	}).Create(&p).Error
	if err != nil {
		return settings, err
	}
	return s.CommentSettings(ctx)
}

func (s *Service) commentsAllowed(ctx context.Context, kind string, p db.CodeReviewPolicy) (bool, error) {
	settings, err := s.CommentSettings(ctx)
	if err != nil {
		return false, err
	}
	return (kind == "commit" && settings.SyncCommits && p.SyncCommits) ||
		(kind == "mr" && settings.SyncMRs && p.SyncMRs), nil
}
