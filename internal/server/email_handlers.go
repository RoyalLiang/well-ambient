package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm/clause"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/config"
	"well-ambient/internal/confluence"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/mailreport"
)

var errEmailAlreadyClaimed = errors.New("该报告日期已经尝试发送，为避免重复邮件不会重发；请检查发送记录和邮箱")

type emailReportRequest struct {
	Date     string                       `json:"date,omitempty"`
	Settings *config.DailyJiraEmailConfig `json:"settings,omitempty"`
}

func (s *Server) setEmailConfig(cfg config.Config) {
	cfg.Jira.SyncUsers = append([]string(nil), cfg.Jira.SyncUsers...)
	cfg.DailyJiraEmail = cfg.DailyJiraEmail.Normalized()
	cfg.SMTP = cfg.SMTP.Normalized()
	s.emailConfigMu.Lock()
	s.emailConfigSnapshot = &cfg
	s.emailConfigMu.Unlock()
}
func (s *Server) currentEmailConfig() config.Config {
	s.emailConfigMu.RLock()
	snapshot := s.emailConfigSnapshot
	s.emailConfigMu.RUnlock()
	if snapshot != nil {
		return *snapshot
	}
	if s.config != nil {
		return *s.config
	}
	return config.Config{}
}
func decodeEmailRequest(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON request")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return fmt.Errorf("request must contain one JSON object")
	}
	return nil
}
func emailJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func emailError(w http.ResponseWriter, status int, err error) {
	emailJSON(w, status, map[string]any{"success": false, "message": err.Error()})
}
func (s *Server) handleGenerateEmailTemplate(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Requirements string `json:"requirements"`
	}
	if err := decodeEmailRequest(w, r, &request); err != nil {
		emailError(w, 400, err)
		return
	}
	request.Requirements = strings.TrimSpace(request.Requirements)
	if request.Requirements == "" || len(request.Requirements) > 12000 {
		emailError(w, 400, fmt.Errorf("requirements must contain 1 to 12000 bytes"))
		return
	}
	cfg := s.currentEmailConfig()
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	system := `You replace the complete Chinese daily Jira email template style, not just greetings. Return ONLY one JSON object with string fields style, subject, introduction, closing, html. style must be exactly "custom". html must contain the complete email body markup that replaces the whole template layout; use email-safe inline-styled HTML and do not include html, head, body, script, style, iframe, form, input, link, meta, object, embed, svg, or external CSS/JS. html must contain {{introduction}}, {{closing}}, {{date}}, {{timezone}}, {{yesterday}}, and {{unresolved}} exactly once each; it may also compose the report with {{owners}}, {{commits}}, {{charts}}, {{overview_cards}}, {{analysis}}, and {{warnings}}. Never invent report facts. Keep copy sharp, concise, professional, and de-AI-ified (no empty pleasantries, corporate fluff, or disclaimers). Introduction must be 1-2 factual sentences. Closing must be 1 concise actionable sentence. Subject <= 512 bytes, introduction <= 8000 bytes, closing <= 4000 bytes, html <= 131072 bytes.`
	current, err := json.Marshal(cfg.DailyJiraEmail.Normalized().Template)
	if err != nil {
		emailError(w, 500, fmt.Errorf("cannot encode current email template"))
		return
	}
	userPrompt := "Current template:\n" + string(current) + "\n\nRequirements:\n" + request.Requirements
	var raw string
	if s.emailLLM != nil {
		raw, err = s.emailLLM(ctx, system, userPrompt)
	} else {
		raw, err = queryServerLLMContext(ctx, &cfg, system, userPrompt)
	}
	if err != nil {
		emailError(w, 502, fmt.Errorf("模板生成失败，请检查 AI 引擎配置后重试"))
		return
	}
	if len(raw) > 192000 {
		emailError(w, 502, fmt.Errorf("generated template exceeds size limit"))
		return
	}
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") && strings.HasSuffix(raw, "```") {
		raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "```json"), "```"))
	}
	var result config.EmailTemplate
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		emailError(w, 502, fmt.Errorf("AI returned an invalid template"))
		return
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		emailError(w, 502, fmt.Errorf("AI returned trailing template data"))
		return
	}
	if strings.TrimSpace(result.Subject) == "" {
		emailError(w, 502, fmt.Errorf("AI returned an empty subject"))
		return
	}
	switch result.Style {
	case "custom":
		sanitized, err := sanitizeEmailTemplateHTML(result.HTML)
		if err != nil {
			emailError(w, 502, fmt.Errorf("AI returned unsafe email template html"))
			return
		}
		result.HTML = sanitized
	default:
		emailError(w, 502, fmt.Errorf("AI returned an invalid template style"))
		return
	}
	if err := config.ValidateEmailTemplate(result); err != nil {
		emailError(w, 502, err)
		return
	}
	result = result.Normalized()
	emailJSON(w, 200, map[string]any{"template": result, "candidates": emailTemplateCandidate(result)})
}
func (s *Server) handlePreviewEmail(w http.ResponseWriter, r *http.Request) {
	var request emailReportRequest
	if err := decodeEmailRequest(w, r, &request); err != nil {
		emailError(w, 400, err)
		return
	}
	cfg := s.currentEmailConfig()
	settings := cfg.DailyJiraEmail.Normalized()
	if request.Settings != nil {
		settings = request.Settings.Normalized()
		merged := cfg
		merged.DailyJiraEmail = settings
		mergeConfiguredSecrets(&merged, cfg)
		settings = merged.DailyJiraEmail
	}
	if err := config.ValidateDailyJiraEmail(settings); err != nil {
		emailError(w, 400, err)
		return
	}
	if _, _, _, err := emailDateBounds(request.Date, settings.Timezone, time.Now()); err != nil {
		emailError(w, 400, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	report, err := s.buildEmailReport(ctx, cfg, settings, request.Date, time.Now())
	if err != nil {
		emailError(w, 500, err)
		return
	}
	emailJSON(w, 200, report)
}
func (s *Server) handleSendEmail(w http.ResponseWriter, r *http.Request) {
	var request emailReportRequest
	if err := decodeEmailRequest(w, r, &request); err != nil {
		emailError(w, 400, err)
		return
	}
	// A send always uses saved settings. Preview drafts cannot change recipients or content.
	if request.Settings != nil {
		emailError(w, 400, fmt.Errorf("请先保存早报与模板配置再发送"))
		return
	}
	cfg := s.currentEmailConfig()
	settings := cfg.DailyJiraEmail.Normalized()
	if !cfg.SMTP.Enabled {
		emailError(w, 409, fmt.Errorf("SMTP 尚未启用"))
		return
	}
	if len(settings.Recipients) == 0 {
		emailError(w, 400, fmt.Errorf("请先保存至少一个早报收件人"))
		return
	}
	if err := config.ValidateSMTP(cfg.SMTP); err != nil {
		emailError(w, 400, err)
		return
	}
	deliverySettings := settings
	deliverySettings.Confluence.Enabled = false
	if err := config.ValidateDailyJiraEmail(deliverySettings); err != nil {
		emailError(w, 400, err)
		return
	}
	if _, _, _, err := emailDateBounds(request.Date, settings.Timezone, time.Now()); err != nil {
		emailError(w, 400, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	date, err := s.sendDailyEmail(ctx, cfg, settings, request.Date, "manual", time.Now())
	if err != nil {
		status := 502
		if errors.Is(err, errEmailAlreadyClaimed) {
			status = 409
		}
		emailError(w, status, err)
		return
	}
	message := "早报处理完成，请查看发送记录；已发送的邮件不会重复发送"
	emailJSON(w, 200, map[string]any{"success": true, "message": message, "date": date})
}
func (s *Server) handleGetEmailCandidateOwners(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := s.currentEmailConfig()
	ownersSet := map[string]bool{}

	for _, m := range s.configuredKPICoreMembers() {
		m = strings.TrimSpace(m)
		if m != "" && m != "-" && m != "未指派" && !strings.EqualFold(m, "unassigned") {
			ownersSet[m] = true
		}
	}

	for _, g := range cfg.DailyJiraEmail.ProjectGroups {
		for _, o := range g.Owners {
			o = strings.TrimSpace(o)
			if o != "" {
				ownersSet[o] = true
			}
		}
	}

	if db.DB != nil {
		var assignees []string
		_ = db.DB.WithContext(r.Context()).Model(&db.TaskTelemetry{}).
			Distinct("assignee").
			Where("TRIM(assignee) <> '' AND assignee <> '-' AND assignee <> '未指派'").
			Limit(1000).
			Pluck("assignee", &assignees).Error
		for _, a := range assignees {
			a = strings.TrimSpace(a)
			if a != "" && !strings.EqualFold(a, "unassigned") {
				ownersSet[a] = true
			}
		}

		var users []userdb.User
		_ = db.DB.WithContext(r.Context()).Select("name", "username").Limit(500).Find(&users).Error
		for _, u := range users {
			if strings.TrimSpace(u.Name) != "" {
				ownersSet[strings.TrimSpace(u.Name)] = true
			}
			if strings.TrimSpace(u.Username) != "" {
				ownersSet[strings.TrimSpace(u.Username)] = true
			}
		}
	}

	var list []string
	for o := range ownersSet {
		list = append(list, o)
	}
	sort.Strings(list)
	if list == nil {
		list = []string{}
	}
	emailJSON(w, 200, map[string]any{"owners": list})
}

func (s *Server) handleEmailRuns(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		emailError(w, 503, fmt.Errorf("database is unavailable"))
		return
	}
	var runs []db.DailyJiraEmailRun
	if err := db.DB.WithContext(r.Context()).Order("date DESC").Limit(14).Find(&runs).Error; err != nil {
		emailError(w, 500, fmt.Errorf("cannot load email delivery history"))
		return
	}
	if runs == nil {
		runs = []db.DailyJiraEmailRun{}
	}
	emailJSON(w, 200, runs)
}
func (s *Server) sendDailyEmail(ctx context.Context, cfg config.Config, settings config.DailyJiraEmailConfig, date, trigger string, now time.Time) (string, error) {
	settings = settings.Normalized()
	if !cfg.SMTP.Enabled {
		return "", fmt.Errorf("SMTP is disabled")
	}
	if err := config.ValidateSMTP(cfg.SMTP); err != nil {
		return "", err
	}
	if len(settings.Recipients) == 0 {
		return "", fmt.Errorf("email recipients are missing")
	}
	// Rendering is independent of the optional document destination. Validate it
	// in the sync stage, where failure must not prevent SMTP.
	reportSettings := settings
	reportSettings.Confluence.Enabled = false
	report, err := s.buildEmailReport(ctx, cfg, reportSettings, date, now)
	if err != nil {
		return "", err
	}
	status := "sending"
	if settings.Confluence.Enabled {
		status = "syncing_confluence"
	}
	recipients, _ := json.Marshal(settings.Recipients)
	syncOnly := false
	claim := db.DailyJiraEmailRun{RecipientsJSON: string(recipients), Date: report.Date, Timezone: report.Timezone, Trigger: trigger, Status: status, StartedAt: now.UTC()}
	result := db.DB.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "date"}}, DoNothing: true}).Create(&claim)
	if result.Error != nil {
		return report.Date, fmt.Errorf("cannot claim email delivery")
	}
	if result.RowsAffected == 0 {
		// Only a confirmed pre-SMTP failure is safe to retry. The conditional
		// update lets one concurrent request reclaim the date.
		retry := db.DB.WithContext(ctx).Model(&db.DailyJiraEmailRun{}).
			Where("date = ? AND status = ?", report.Date, "confluence_failed").
			Updates(map[string]any{"status": status, "trigger": trigger, "started_at": now.UTC(), "finished_at": nil, "confluence_url": ""})
		if retry.Error != nil {
			return report.Date, fmt.Errorf("cannot reclaim email delivery")
		}
		if retry.RowsAffected == 0 {
			// A confirmed sent email is never submitted to SMTP again.
			retrySync := db.DB.WithContext(ctx).Model(&db.DailyJiraEmailRun{}).Where("date = ? AND status = ? AND confluence_status = ?", report.Date, "sent", "failed").Update("confluence_status", "syncing")
			if retrySync.Error != nil {
				return report.Date, retrySync.Error
			}
			if retrySync.RowsAffected == 0 {
				return report.Date, errEmailAlreadyClaimed
			}
			syncOnly = true
		}
	}
	finalizeRun := func(state, pageURL string) error {
		finalize, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return db.DB.WithContext(finalize).Model(&db.DailyJiraEmailRun{}).Where("date = ?", report.Date).
			Updates(map[string]any{"status": state, "finished_at": time.Now().UTC(), "confluence_url": pageURL}).Error
	}
	var syncFailure error
	if settings.Confluence.Enabled {
		syncReport := s.emailConfluenceSync
		if syncReport == nil {
			syncReport = syncEmailReportToConfluence
		}
		syncCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
		pageURL := ""
		syncErr := config.ValidateConfluenceSync(settings.Confluence)
		if syncErr == nil {
			pageURL, syncErr = syncReport(syncCtx, settings.Confluence, &report)
		}
		cancel()
		if syncErr == nil && pageURL == "" {
			syncErr = fmt.Errorf("missing Confluence page URL")
		}
		syncState, reason := "synced", ""
		if syncErr != nil {
			syncState, reason = "failed", confluence.ErrorMessage(syncErr)
			syncFailure = fmt.Errorf("%s", reason)
			log.Printf("daily Jira Confluence sync failed date=%s sync_only=%t: %s", report.Date, syncOnly, reason)
		} else {
			report.ConfluenceURL = pageURL
		}
		finalCtx, finish := context.WithTimeout(context.Background(), 2*time.Second)
		update := db.DB.WithContext(finalCtx).Model(&db.DailyJiraEmailRun{}).Where("date = ?", report.Date).Updates(map[string]any{"confluence_status": syncState, "confluence_error": reason, "confluence_url": pageURL})
		finish()
		if update.Error != nil {
			return report.Date, fmt.Errorf("同步状态保存失败，保留发送占用以防重复")
		}
	} else if syncOnly {
		_ = db.DB.Model(&db.DailyJiraEmailRun{}).Where("date = ?", report.Date).Update("confluence_status", "failed").Error
		return report.Date, fmt.Errorf("邮件已发送；请启用 Confluence 后仅重试同步")
	}
	if syncOnly {
		if syncFailure != nil {
			return report.Date, fmt.Errorf("邮件此前已发送，本次仅重试 Confluence 同步失败：%s", syncFailure)
		}
		return report.Date, nil
	}
	if report.ConfluenceURL != "" {
		if err := renderEmailReport(&report); err != nil {
			_ = finalizeRun("confluence_failed", report.ConfluenceURL)
			return report.Date, fmt.Errorf("邮件生成失败，未提交 SMTP；可重试")
		}
	}
	if err := db.DB.WithContext(ctx).Model(&db.DailyJiraEmailRun{}).Where("date = ?", report.Date).Updates(map[string]any{"status": "sending", "recipients_json": string(recipients)}).Error; err != nil {
		return report.Date, err
	}
	sender := s.emailSender
	if sender == nil {
		sender = mailreport.Send
	}
	sendErr := sender(ctx, cfg.SMTP, settings.Recipients, report.Subject, report.Text, report.HTML)
	status = "sent"
	if sendErr != nil {
		status = "failed_or_unknown"
	}
	// Finalize even after cancellation; a failed finalization still retains the durable claim.
	updateErr := finalizeRun(status, report.ConfluenceURL)
	if sendErr != nil {
		return report.Date, sendErr
	}
	if updateErr != nil {
		return report.Date, fmt.Errorf("SMTP accepted email but delivery ledger finalization failed")
	}
	if syncFailure != nil {
		return report.Date, fmt.Errorf("邮件已发送，Confluence 同步失败；再次重试仅同步文档，不会重发邮件：%s", syncFailure)
	}
	return report.Date, nil
}
