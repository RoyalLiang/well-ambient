package telemetry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	userdb "well-ambient/internal/db/user"
	"well-ambient/internal/mailreport"
)

const missingJiraReason = "MR 已关闭：本次 MR 包含的所有 commit message 均未包含 Jira 编号。请在相关提交说明中补充 Jira 编号后重新打开 MR。"

// Temporarily pause automatic closure after reports of incorrect MR closures.
// Keep the policy implementation for investigation; re-enable only after validation.
const mrAutoCloseEnabled = false

var jiraCommitKey = regexp.MustCompile(`\b[A-Z][A-Z0-9_]*-[1-9][0-9]*\b`)

type policyUser struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	PublicEmail string `json:"public_email"`
}
type policyMR struct {
	State     string       `json:"state"`
	SHA       string       `json:"sha"`
	UpdatedAt string       `json:"updated_at"`
	Title     string       `json:"title"`
	WebURL    string       `json:"web_url"`
	Author    policyUser   `json:"author"`
	Assignees []policyUser `json:"assignees"`
	Assignee  *policyUser  `json:"assignee"`
}
type mrPolicy struct {
	conn   *gorm.DB
	cfg    *config.Config
	client *http.Client
	send   func(context.Context, config.SMTPConfig, []string, string, string, string) error
}

func (p mrPolicy) request(ctx context.Context, method, path string, body any, out any) (http.Header, error) {
	base, err := url.Parse(strings.TrimRight(p.cfg.GitLab.BaseURL, "/"))
	if err != nil || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Scheme != "https" && !(base.Scheme == "http" && (base.Hostname() == "127.0.0.1" || base.Hostname() == "localhost"))) {
		return nil, errors.New("invalid GitLab API base URL")
	}
	var data []byte
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(base.String(), "/")+"/api/v4"+path, bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("invalid GitLab policy request")
	}
	req.Header.Set("PRIVATE-TOKEN", p.cfg.GitLab.APIToken)
	req.Header.Set("Content-Type", "application/json")
	client := *p.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("GitLab MR policy request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitLab MR policy HTTP %d", resp.StatusCode)
	}
	if out != nil {
		if err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
			return nil, errors.New("invalid GitLab MR policy response")
		}
	}
	return resp.Header, nil
}

func (p mrPolicy) recipients(ctx context.Context, mr policyMR) ([]string, error) {
	users := append([]policyUser{mr.Author}, mr.Assignees...)
	if mr.Assignee != nil {
		users = append(users, *mr.Assignee)
	}
	seenUser := map[int]bool{}
	seenEmail := map[string]bool{}
	var result []string
	for _, u := range users {
		if u.ID <= 0 {
			return nil, errors.New("MR notification recipient identity is missing")
		}
		if seenUser[u.ID] {
			continue
		}
		seenUser[u.ID] = true
		var full policyUser
		if _, err := p.request(ctx, http.MethodGet, "/users/"+strconv.Itoa(u.ID), nil, &full); err != nil {
			return nil, err
		}
		if full.ID != u.ID {
			return nil, errors.New("GitLab recipient identity does not match MR participant")
		}
		address := full.Email
		if address == "" {
			address = full.PublicEmail
		}
		// Only an exact account identifier may bridge to the local user directory.
		if address == "" && full.Username != "" {
			var matches []userdb.User
			if err := p.conn.Where("username = ?", full.Username).Limit(2).Find(&matches).Error; err != nil {
				return nil, errors.New("cannot resolve MR recipient email")
			}
			if len(matches) == 1 {
				address = matches[0].Email
			}
		}
		mailbox, err := config.Mailbox(address)
		if err != nil || mailbox == "" {
			return nil, errors.New("MR author or assignee has no resolvable email address")
		}
		key := strings.ToLower(mailbox)
		if !seenEmail[key] {
			seenEmail[key] = true
			result = append(result, mailbox)
		}
	}
	return result, nil
}

func (p mrPolicy) enforce(ctx context.Context, project string, iid int) (bool, error) {
	path := "/projects/" + url.PathEscape(project) + "/merge_requests/" + strconv.Itoa(iid)
	var mr policyMR
	if _, err := p.request(ctx, http.MethodGet, path, nil, &mr); err != nil {
		return false, err
	}
	if mr.State != "opened" {
		return false, nil
	}
	if mr.SHA == "" || mr.UpdatedAt == "" {
		return false, errors.New("MR head or version is missing")
	}
	total := 0
	hasKey := false
	for page := 1; page <= 100; page++ {
		var commits []struct {
			ID      string  `json:"id"`
			Message *string `json:"message"`
		}
		headers, err := p.request(ctx, http.MethodGet, path+"/commits?per_page=100&page="+strconv.Itoa(page), nil, &commits)
		if err != nil {
			return false, err
		}
		total += len(commits)
		for _, c := range commits {
			if c.ID == "" || c.Message == nil {
				return false, errors.New("incomplete MR commit data")
			}
			hasKey = hasKey || jiraCommitKey.MatchString(*c.Message)
		}
		next := headers.Get("X-Next-Page")
		if next != "" {
			n, e := strconv.Atoi(next)
			if e != nil || n != page+1 {
				return false, errors.New("invalid MR commit pagination")
			}
		}
		if next == "" && len(commits) < 100 {
			break
		}
		if page == 100 {
			return false, errors.New("MR commit pagination limit exceeded")
		}
	}
	if total == 0 {
		return false, errors.New("MR commit list is empty; no closure decision made")
	}
	if hasKey {
		return false, nil
	}
	// Read the current head again after inspecting all pages and before side effects.
	var current policyMR
	if _, err := p.request(ctx, http.MethodGet, path, nil, &current); err != nil {
		return false, err
	}
	if current.State != "opened" || current.SHA != mr.SHA || current.UpdatedAt != mr.UpdatedAt {
		return false, errors.New("MR changed during policy check; awaiting a fresh event")
	}
	sum := sha256.Sum256([]byte(project + ":" + strconv.Itoa(iid) + ":" + mr.SHA + ":" + mr.UpdatedAt))
	run := db.MRPolicyRun{Key: hex.EncodeToString(sum[:]), Project: project, MRIID: iid, HeadSHA: mr.SHA, Status: "closing", Reason: missingJiraReason}
	claim := p.conn.Clauses(clause.OnConflict{DoNothing: true}).Create(&run)
	if claim.Error != nil {
		return false, claim.Error
	}
	if claim.RowsAffected == 0 {
		return false, nil
	}
	finish := func(status string) error {
		return p.conn.Model(&db.MRPolicyRun{}).Where("key = ?", run.Key).Update("status", status).Error
	}
	var closed policyMR
	if _, err := p.request(ctx, http.MethodPut, path, map[string]string{"state_event": "close"}, &closed); err != nil {
		_ = finish("close_failed_or_unknown")
		return false, err
	}
	if closed.State != "closed" {
		_ = finish("close_failed_or_unknown")
		return false, errors.New("GitLab did not confirm MR closure")
	}
	if err := finish("closed"); err != nil {
		return true, err
	}
	// Keep the reason in GitLab as well as the durable local decision ledger.
	_, noteErr := p.request(ctx, http.MethodPost, path+"/notes", map[string]string{"body": missingJiraReason}, nil)
	recipients, err := p.recipients(ctx, mr)
	if err != nil {
		_ = finish("notification_blocked")
		return true, err
	}
	if !p.cfg.SMTP.Enabled {
		_ = finish("notification_blocked")
		return true, errors.New("MR closed but SMTP is disabled")
	}
	if err := finish("sending"); err != nil {
		return true, err
	}
	subject := fmt.Sprintf("MR !%d 已关闭：缺少 Jira 编号", iid)
	text := fmt.Sprintf("%s\n\nMR: %s\n%s\n", missingJiraReason, mr.Title, mr.WebURL)
	if err := p.send(ctx, p.cfg.SMTP, recipients, subject, text, "<p>"+strings.ReplaceAll(html.EscapeString(text), "\n", "<br>")+"</p>"); err != nil {
		_ = finish("mail_failed_or_unknown")
		return true, errors.New("MR closed; notification delivery failed or is unknown")
	}
	status := "sent"
	if noteErr != nil {
		status = "sent_note_failed"
	}
	return true, finish(status)
}

func enforceMRCommitPolicy(cfg *config.Config, payload MergeRequestHookPayload) (bool, error) {
	if !mrAutoCloseEnabled {
		return false, nil
	}
	if !cfg.GitLab.Enabled || payload.ObjectAttributes.State != "opened" || payload.ObjectAttributes.IID <= 0 {
		return false, nil
	}
	project := ""
	for _, repo := range cfg.GitLab.Repos {
		if (payload.Project.ID > 0 && repo.ProjectID == strconv.Itoa(payload.Project.ID)) || (payload.Project.ID == 0 && repo.Name == payload.Project.Name) {
			project = repo.ProjectID
			break
		}
	}
	if project == "" {
		return false, nil
	}
	if cfg.GitLab.APIToken == "" || db.DB == nil {
		return false, errors.New("MR policy requires GitLab API credentials and database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return (mrPolicy{conn: db.DB, cfg: cfg, client: &http.Client{Timeout: 15 * time.Second}, send: mailreport.Send}).enforce(ctx, project, payload.ObjectAttributes.IID)
}
