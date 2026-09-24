package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
	"well-ambient/internal/config"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

const maxEmailTemplateCandidates = 30

var errEmailTemplateGalleryFull = errors.New("模板库最多保存 30 条自建候选，请先删除不再需要的候选")

type emailTemplateCandidateInput struct {
	Name     string               `json:"name"`
	Template config.EmailTemplate `json:"template"`
}
type emailTemplateCandidateDTO struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Builtin     bool                 `json:"builtin"`
	Template    config.EmailTemplate `json:"template"`
	PreviewHTML string               `json:"preview_html"`
	CreatedAt   *time.Time           `json:"created_at,omitempty"`
}

// One model-written template becomes one complete candidate. For custom style
// the model supplies a full email body; the server sanitizes it before render.
func emailTemplateCandidate(t config.EmailTemplate) []emailTemplateCandidateInput {
	return []emailTemplateCandidateInput{{Name: "Agent 自定义整版模板", Template: t.Normalized()}}
}

// Gallery previews contain no runtime facts, credentials or personal data.
// Their fixed demonstration data makes styles comparable across reloads.
func emailTemplateDemo(t config.EmailTemplate) (emailReport, error) {
	t = t.Normalized()
	if err := config.ValidateEmailTemplate(t); err != nil {
		return emailReport{}, err
	}
	stamp := time.Date(2026, 1, 14, 9, 0, 0, 0, time.FixedZone("CST", 8*3600))
	items := []emailIssue{
		{TaskID: "DEMO-101", Title: "示例：确认接口验收条件", Assignee: "示例负责人甲", Status: "progress", IssueType: "任务", Category: "FMS", CreatedAt: stamp, UpdatedAt: stamp},
		{TaskID: "DEMO-102", Title: "示例：完善发布检查清单", Assignee: "示例负责人乙", Status: "review", IssueType: "缺陷", Category: "GPP", CreatedAt: stamp.AddDate(0, 0, -1), UpdatedAt: stamp},
		{TaskID: "DEMO-103", Title: "示例：完成数据校验", Assignee: "示例负责人甲", Status: "done", IssueType: "任务", Category: "FMS", CreatedAt: stamp, UpdatedAt: stamp},
	}
	expand := func(text string) string {
		return strings.NewReplacer("{{date}}", "2026-01-15", "{{timezone}}", "Asia/Shanghai").Replace(text)
	}
	report := emailReport{
		Style: t.Style, Date: "2026-01-15", Timezone: "Asia/Shanghai", Subject: expand(t.Subject), Introduction: expand(t.Introduction), Closing: expand(t.Closing),
		Semantics:        "示例数据，仅用于比较模板样式；报告日期 2026-01-15（时区 Asia/Shanghai）。",
		Warnings:         []string{"示例数据 · 此预览不会查询实际事项或发送邮件。"},
		YesterdayUpdated: items, RecentUnresolved: append([]emailIssue(nil), items[:2]...),
		CoreMembers:   []emailCoreMember{{Name: "示例负责人甲", YesterdayCount: 2, UnresolvedCount: 1, CommitCount: 4}, {Name: "示例负责人乙", YesterdayCount: 1, UnresolvedCount: 1, CommitCount: 2}},
		Commits:       &emailCommitSummary{Count: 6, Repositories: 2, Authors: map[string]int{"示例负责人甲": 4, "示例负责人乙": 2}, Analysis: "示例：昨日采集 6 次提交，涉及 2 个代码仓库。"},
		ActivityChart: &emailChart{Title: "昨日 Commit / MR", Description: "示例数据：Commit 按仓库与 SHA、MR 按仓库与 IID 去重。", Total: 9, Bars: []emailChartBar{{Label: "Commit", Count: 6}, {Label: "MR", Count: 3}}},
		TrendPoints:   []int{4, 6, 5, 8, 7, 5, len(items)},
	}
	// Exercise the same project-group layout as configured reports without
	// reading the active configuration or exposing real projects and owners.
	groups := []config.DailyJiraProjectGroup{{
		Name: "示例交付组", Projects: []string{"DEMO"},
		Owners: []string{"示例负责人甲", "示例负责人乙"},
	}}
	report.YesterdayGroups, report.YesterdayUngrouped = groupIssues(report.YesterdayUpdated, groups)
	report.UnresolvedGroups, report.UnresolvedUngrouped = groupIssues(report.RecentUnresolved, groups)
	if t.Style == "custom" {
		sanitized, err := sanitizeEmailTemplateHTML(t.HTML)
		if err != nil {
			return emailReport{}, err
		}
		report.CustomHTML = template.HTML(sanitized)
	}
	if err := renderEmailReport(&report); err != nil {
		return emailReport{}, err
	}
	return report, nil
}

func emailTemplateDTO(id, name string, builtin bool, t config.EmailTemplate, createdAt *time.Time) (emailTemplateCandidateDTO, error) {
	t = t.Normalized()
	demo, err := emailTemplateDemo(t)
	if err != nil {
		return emailTemplateCandidateDTO{}, err
	}
	return emailTemplateCandidateDTO{ID: id, Name: name, Builtin: builtin, Template: t, PreviewHTML: demo.HTML, CreatedAt: createdAt}, nil
}

func (s *Server) handleListEmailTemplates(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		emailError(w, 503, fmt.Errorf("database is unavailable"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	var rows []db.EmailTemplateCandidate
	if err := db.DB.WithContext(ctx).Order("created_at DESC").Order("id DESC").Limit(maxEmailTemplateCandidates).Find(&rows).Error; err != nil {
		emailError(w, 500, fmt.Errorf("cannot load email template gallery"))
		return
	}
	builtins := []struct{ style, name string }{
		{"hyperframe", "HyperFrame 全景看板"},
		{"brief", "晨间简报"},
		{"focus", "行动聚焦"},
		{"ledger", "明细台账"},
	}
	result := make([]emailTemplateCandidateDTO, 0, len(builtins)+len(rows))
	for _, entry := range builtins {
		tmpl := config.DefaultEmailTemplate()
		tmpl.Style = entry.style
		dto, err := emailTemplateDTO("builtin-"+entry.style, entry.name, true, tmpl, nil)
		if err != nil {
			emailError(w, 500, fmt.Errorf("cannot render built-in email template"))
			return
		}
		result = append(result, dto)
	}
	for _, row := range rows {
		var t config.EmailTemplate
		if err := json.Unmarshal([]byte(row.TemplateJSON), &t); err != nil {
			emailError(w, 500, fmt.Errorf("stored email template is invalid"))
			return
		}
		created := row.CreatedAt
		dto, err := emailTemplateDTO(strconv.FormatUint(uint64(row.ID), 10), row.Name, false, t, &created)
		if err != nil {
			emailError(w, 500, fmt.Errorf("stored email template cannot be rendered"))
			return
		}
		result = append(result, dto)
	}
	emailJSON(w, 200, map[string]any{"templates": result})
}

// appendEmailTemplates serializes count+insert across processes, not merely
// HTTP goroutines. PostgreSQL's table lock and SQLite's write reservation cover
// empty galleries too; a row lock alone would not protect that case.
func appendEmailTemplates(ctx context.Context, conn *gorm.DB, rows []db.EmailTemplateCandidate) error {
	return conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("LOCK TABLE email_template_candidates IN EXCLUSIVE MODE").Error; err != nil {
				return err
			}
		} else {
			if err := tx.Exec("UPDATE email_template_candidates SET id = id WHERE 1 = 0").Error; err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Model(&db.EmailTemplateCandidate{}).Count(&count).Error; err != nil {
			return err
		}
		if count+int64(len(rows)) > maxEmailTemplateCandidates {
			return errEmailTemplateGalleryFull
		}
		return tx.Create(&rows).Error
	})
}

func (s *Server) handleCreateEmailTemplates(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Templates []emailTemplateCandidateInput `json:"templates"`
	}
	if err := decodeEmailRequest(w, r, &request); err != nil {
		emailError(w, 400, err)
		return
	}
	if len(request.Templates) < 1 || len(request.Templates) > 3 {
		emailError(w, 400, fmt.Errorf("每次请保存 1 至 3 条模板候选"))
		return
	}
	if db.DB == nil {
		emailError(w, 503, fmt.Errorf("database is unavailable"))
		return
	}
	rows := make([]db.EmailTemplateCandidate, 0, len(request.Templates))
	result := make([]emailTemplateCandidateDTO, 0, len(request.Templates))
	now := time.Now().UTC()
	for _, entry := range request.Templates {
		name := strings.TrimSpace(entry.Name)
		if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			emailError(w, 400, fmt.Errorf("模板名称须为 1 至 80 个字符且不包含控制字符"))
			return
		}
		if entry.Template.Style == "custom" {
			sanitized, err := sanitizeEmailTemplateHTML(entry.Template.HTML)
			if err != nil {
				emailError(w, 400, fmt.Errorf("自定义模板 HTML 不安全或无法解析"))
				return
			}
			entry.Template.HTML = sanitized
		}
		t := entry.Template.Normalized()
		if err := config.ValidateEmailTemplate(t); err != nil {
			emailError(w, 400, err)
			return
		}
		dto, err := emailTemplateDTO("", name, false, t, &now)
		if err != nil {
			emailError(w, 400, err)
			return
		}
		encoded, err := json.Marshal(t)
		if err != nil {
			emailError(w, 400, fmt.Errorf("invalid email template"))
			return
		}
		rows = append(rows, db.EmailTemplateCandidate{Name: name, TemplateJSON: string(encoded), CreatedAt: now})
		result = append(result, dto)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := appendEmailTemplates(ctx, db.DB, rows); err != nil {
		if errors.Is(err, errEmailTemplateGalleryFull) {
			emailError(w, 409, err)
		} else {
			emailError(w, 500, fmt.Errorf("cannot save email template gallery"))
		}
		return
	}
	for i := range rows {
		result[i].ID = strconv.FormatUint(uint64(rows[i].ID), 10)
	}
	emailJSON(w, http.StatusCreated, map[string]any{"templates": result})
}

func (s *Server) handleDeleteEmailTemplate(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("id")
	if strings.HasPrefix(raw, "builtin-") {
		emailError(w, 403, fmt.Errorf("内置模板不可删除"))
		return
	}
	id, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != raw {
		emailError(w, 400, fmt.Errorf("invalid email template id"))
		return
	}
	if db.DB == nil {
		emailError(w, 503, fmt.Errorf("database is unavailable"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result := db.DB.WithContext(ctx).Delete(&db.EmailTemplateCandidate{}, uint(id))
	if result.Error != nil {
		emailError(w, 500, fmt.Errorf("cannot delete email template"))
		return
	}
	if result.RowsAffected == 0 {
		emailError(w, 404, fmt.Errorf("email template not found"))
		return
	}
	emailJSON(w, 200, map[string]bool{"success": true})
}
