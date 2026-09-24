package codereview

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm/clause"
	"net/http"
	"strconv"
	"strings"
	"time"
	"well-ambient/internal/db"
)

func publicationLeaseToken() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (s *Service) Publish(ctx context.Context, run db.CodeReviewRun) error {
	if _, err := s.Repo(run.ProjectID); err != nil {
		return s.status(run, "blocked")
	}
	if !s.Config().GitLab.Enabled || run.Status != "completed" || !shaPattern.MatchString(run.HeadSHA) {
		return s.status(run, "blocked")
	}
	allowed, err := s.policyAllows(run)
	if err != nil {
		return err
	}
	if !allowed {
		return s.status(run, "off")
	}
	g := s.git()
	if run.Kind == "mr" {
		m, err := g.mr(ctx, run.ProjectID, run.Ref)
		if err != nil {
			return err
		}
		if m.State != "opened" || m.DiffRefs.Head != run.HeadSHA || m.DiffRefs.Base != run.BaseSHA {
			return s.status(run, "stale")
		}
	}
	var report Report
	if err = json.Unmarshal([]byte(run.ReportJSON), &report); err != nil {
		return s.status(run, "blocked")
	}
	key := publicationKey(run)
	marker := "<!-- well-ambient-code-review:" + key + " -->"
	path := projectPath(run.ProjectID)
	if run.Kind == "mr" {
		path += "/merge_requests/" + run.Ref + "/notes"
	} else {
		path += "/repository/commits/" + run.HeadSHA + "/comments"
	}
	// Reconcile before claiming or retrying. A timeout is never permission for a second POST.
	commentID, found, err := g.findComment(ctx, path, marker)
	if err != nil {
		return err
	}
	if found {
		return s.published(run, key, commentID)
	}
	leaseToken, err := publicationLeaseToken()
	if err != nil {
		return err
	}
	claim := db.CodeReviewPublication{Key: key, RunID: run.ID, Status: "claimed", LeaseToken: leaseToken}
	res := s.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var existing db.CodeReviewPublication
		if err = s.DB.First(&existing, "key = ?", key).Error; err != nil {
			return err
		}
		switch existing.Status {
		case "published":
			return s.published(run, key, existing.CommentID)
		case "unknown":
			return s.status(run, "unknown")
		case "sending":
			_ = s.DB.Model(&db.CodeReviewPublication{}).
				Where("key = ? AND status = ? AND lease_token = ?", key, "sending", existing.LeaseToken).
				Update("status", "unknown").Error
			return s.status(run, "unknown")
		case "claimed", "publishing":
			leaseCutoff := time.Now().Add(-2 * time.Minute)
			if !existing.UpdatedAt.Before(leaseCutoff) {
				return nil
			}
			leaseToken, err = publicationLeaseToken()
			if err != nil {
				return err
			}
			reclaim := s.DB.Model(&db.CodeReviewPublication{}).
				Where("key = ? AND status IN ? AND updated_at = ?", key, []string{"claimed", "publishing"}, existing.UpdatedAt).
				Updates(map[string]any{"run_id": run.ID, "status": "claimed", "lease_token": leaseToken, "updated_at": time.Now()})
			if reclaim.Error != nil {
				return reclaim.Error
			}
			if reclaim.RowsAffected == 0 {
				return nil
			}
		default:
			// Another publisher owns the claim. It will either publish or expose
			// an unknown result; do not regress the run projection here.
			return nil
		}
	}
	if err = s.status(run, "publishing"); err != nil {
		return s.releasePublicationClaim(run, key, leaseToken, err)
	}
	unlockPolicy := LockPolicy(run.ProjectID)
	defer unlockPolicy()
	// Check current policy immediately before the external write, including a switch turned off mid-run.
	allowed, err = s.policyAllows(run)
	if err != nil {
		return s.releasePublicationClaim(run, key, leaseToken, err)
	}
	if !allowed {
		_ = s.DB.Where("key = ? AND status IN ? AND lease_token = ?", key, []string{"claimed", "publishing"}, leaseToken).Delete(&db.CodeReviewPublication{}).Error
		return s.status(run, "off")
	}
	if run.Kind == "mr" {
		m, e := g.mr(ctx, run.ProjectID, run.Ref)
		if e != nil {
			return s.releasePublicationClaim(run, key, leaseToken, e)
		}
		if m.State != "opened" || m.DiffRefs.Head != run.HeadSHA || m.DiffRefs.Base != run.BaseSHA {
			_ = s.DB.Where("key = ? AND lease_token = ?", key, leaseToken).Delete(&db.CodeReviewPublication{}).Error
			return s.status(run, "stale")
		}
	}
	fence := s.DB.Model(&db.CodeReviewPublication{}).
		Where("key = ? AND status IN ? AND lease_token = ?", key, []string{"claimed", "publishing"}, leaseToken).
		Updates(map[string]any{"status": "sending", "updated_at": time.Now()})
	if fence.Error != nil {
		return fence.Error
	}
	if fence.RowsAffected == 0 {
		return nil
	}
	body := commentBody(run, report) + "\n\n" + marker
	payload := map[string]string{"body": body}
	if run.Kind == "commit" {
		payload = map[string]string{"note": body}
	}
	var result struct {
		ID int `json:"id"`
	}
	_, err = g.request(ctx, http.MethodPost, path, payload, &result)
	if err != nil {
		var httpErr *gitLabHTTPError
		if errors.As(err, &httpErr) {
			return s.releasePublicationClaim(run, key, leaseToken, err)
		}
		_ = s.DB.Model(&db.CodeReviewPublication{}).
			Where("key = ? AND status = ? AND lease_token = ?", key, "sending", leaseToken).
			Update("status", "unknown").Error
		_ = s.status(run, "unknown")
		return errors.New("评论发送结果未知；请核对远端评论，系统不会自动重复发送")
	}
	return s.published(run, key, result.ID)
}
func (s *Service) releasePublicationClaim(run db.CodeReviewRun, key, leaseToken string, cause error) error {
	result := s.DB.Where(
		"key = ? AND status IN ? AND lease_token = ?",
		key,
		[]string{"claimed", "publishing", "sending"},
		leaseToken,
	).Delete(&db.CodeReviewPublication{})
	if result.Error == nil && result.RowsAffected > 0 {
		_ = s.status(run, "sync_failed")
	}
	return cause
}
func (s *Service) published(run db.CodeReviewRun, key string, id int) error {
	p := db.CodeReviewPublication{Key: key, RunID: run.ID, Status: "published", CommentID: id}
	if err := s.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"status", "comment_id", "updated_at"})}).Create(&p).Error; err != nil {
		return err
	}
	return s.DB.Model(&db.CodeReviewRun{}).Where("id = ?", run.ID).Updates(map[string]any{"publish_status": "published", "publish_error": "", "comment_id": id, "published_at": time.Now()}).Error
}
func (g GitLab) findComment(ctx context.Context, path, marker string) (int, bool, error) {
	for page := 1; page <= 20; page++ {
		var comments []struct {
			ID   int    `json:"id"`
			Body string `json:"body"`
			Note string `json:"note"`
		}
		h, err := g.request(ctx, http.MethodGet, path+"?per_page=100&page="+strconv.Itoa(page), nil, &comments)
		if err != nil {
			return 0, false, err
		}
		for _, c := range comments {
			if strings.Contains(c.Body, marker) || strings.Contains(c.Note, marker) {
				return c.ID, true, nil
			}
		}
		next := h.Get("X-Next-Page")
		if next == "" {
			if len(comments) >= 100 {
				return 0, false, errors.New("评论列表分页不完整，暂不发送")
			}
			return 0, false, nil
		}
		if next != strconv.Itoa(page+1) {
			return 0, false, errors.New("评论分页异常")
		}
	}
	return 0, false, errors.New("评论过多，无法确认是否重复")
}
func commentBody(run db.CodeReviewRun, r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Code Review · %s\n\n代码版本：`%s`\n\n%s\n\n%s\n", r.Scenario, run.HeadSHA, r.Summary, r.Validation)
	if len(r.Findings) == 0 {
		b.WriteString("\n在本次已读取范围内未发现具备源码证据的问题；不等同于安全或合并批准。\n")
	}
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "\n#### [%s] %s\n`%s:%d` · %s\n\n- 影响：%s\n- 建议：%s\n- 验证方法：%s\n", f.Severity, f.Title, f.File, f.Line, f.Dimension, f.Impact, f.Suggestion, f.Verification)
		if len(f.KnowledgeIDs) > 0 {
			fmt.Fprintf(&b, "- 系统知识引用：%v\n", f.KnowledgeIDs)
		}
	}
	for _, a := range r.Assessments {
		fmt.Fprintf(&b, "\n**%s**：%s\n", a.Dimension, a.Analysis)
	}
	if len(r.Questions) > 0 {
		b.WriteString("\n**待确认与覆盖限制**\n")
		for _, q := range r.Questions {
			b.WriteString("- " + q + "\n")
		}
	}
	b.WriteString("\n此评论由 well-ambient 自动评审生成，不会关闭或合并 MR。")
	// Prevent untrusted model/source text from issuing broad GitLab mentions.
	return strings.ReplaceAll(b.String(), "@", "＠")
}
