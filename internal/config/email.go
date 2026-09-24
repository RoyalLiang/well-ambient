package config

import (
	"fmt"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

type SMTPConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	From     string `yaml:"from" json:"from"`
	TLSMode  string `yaml:"tls_mode" json:"tls_mode"`
}
type EmailTemplate struct {
	Style        string `yaml:"style" json:"style"`
	Subject      string `yaml:"subject" json:"subject"`
	Introduction string `yaml:"introduction" json:"introduction"`
	Closing      string `yaml:"closing" json:"closing"`
	HTML         string `yaml:"html,omitempty" json:"html,omitempty"`
}
type DailyJiraProjectGroup struct {
	Name     string   `yaml:"name" json:"name"`
	Owners   []string `yaml:"owners" json:"owners"`
	Projects []string `yaml:"projects" json:"projects"`
}
type DailyJiraEmailConfig struct {
	Enabled        bool                    `yaml:"enabled" json:"enabled"`
	Recipients     []string                `yaml:"recipients" json:"recipients"`
	Timezone       string                  `yaml:"timezone" json:"timezone"`
	SendTime       string                  `yaml:"send_time" json:"send_time"`
	IncludeCommits bool                    `yaml:"include_commits" json:"include_commits"`
	Template       EmailTemplate           `yaml:"template" json:"template"`
	ProjectGroups  []DailyJiraProjectGroup `yaml:"project_groups,omitempty" json:"project_groups"`
	Confluence     ConfluenceSyncConfig    `yaml:"confluence" json:"confluence"`
}

func (c SMTPConfig) Normalized() SMTPConfig {
	c.Host = strings.TrimSpace(c.Host)
	c.From = strings.TrimSpace(c.From)
	c.TLSMode = strings.ToLower(strings.TrimSpace(c.TLSMode))
	if c.TLSMode == "" {
		c.TLSMode = "starttls"
	}
	if c.Port == 0 {
		if c.TLSMode == "tls" {
			c.Port = 465
		} else if c.TLSMode == "none" {
			c.Port = 25
		} else {
			c.Port = 587
		}
	}
	return c
}

// DefaultEmailTemplate is ready to send without an AI request. The renderer adds
// the fact tables, owner summaries and email-compatible charts between these texts.
func DefaultEmailTemplate() EmailTemplate {
	return EmailTemplate{
		Style:        "brief",
		Subject:      "研发每日早报 · {{date}}",
		Introduction: "各位早上好，以下为 {{date}} 研发早报（{{timezone}}）：昨日更新与待跟进事项已汇总如下。",
		Closing:      "请相关负责人推进待办事项，并及时更新 Jira 状态。",
	}
}

func (c DailyJiraEmailConfig) Normalized() DailyJiraEmailConfig {
	c.Confluence = c.Confluence.Normalized()
	if c.Timezone == "" {
		c.Timezone = "Asia/Shanghai"
	}
	if c.SendTime == "" {
		c.SendTime = "09:00"
	}
	c.Recipients = append([]string{}, c.Recipients...)
	c.Template = c.Template.Normalized()
	if c.ProjectGroups != nil {
		groups := make([]DailyJiraProjectGroup, 0, len(c.ProjectGroups))
		for _, g := range c.ProjectGroups {
			name := strings.TrimSpace(g.Name)
			owners := make([]string, 0, len(g.Owners))
			for _, o := range g.Owners {
				o = strings.TrimSpace(o)
				if o != "" {
					owners = append(owners, o)
				}
			}
			projects := make([]string, 0, len(g.Projects))
			for _, p := range g.Projects {
				p = strings.TrimSpace(p)
				if p != "" {
					projects = append(projects, p)
				}
			}
			if name != "" || len(owners) > 0 || len(projects) > 0 {
				groups = append(groups, DailyJiraProjectGroup{
					Name:     name,
					Owners:   owners,
					Projects: projects,
				})
			}
		}
		c.ProjectGroups = groups
	} else {
		c.ProjectGroups = []DailyJiraProjectGroup{}
	}
	return c
}

// Normalized preserves legacy text-only templates and assigns the original layout.
func (t EmailTemplate) Normalized() EmailTemplate {
	if t == (EmailTemplate{}) {
		return DefaultEmailTemplate()
	}
	if t.Style == "" {
		t.Style = "brief"
	}
	if t.Subject == "" {
		t.Subject = DefaultEmailTemplate().Subject
	}
	return t
}

func Mailbox(value string) (string, error) {
	if strings.ContainsAny(value, "\r\n\x00") {
		return "", fmt.Errorf("email address contains forbidden characters")
	}
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || !strings.Contains(value, "@") {
		return "", fmt.Errorf("a single bare email address is required")
	}
	return value, nil
}
func ValidateSMTP(c SMTPConfig) error {
	c = c.Normalized()
	if c.Host == "" || strings.ContainsAny(c.Host, "\r\n\x00 /\\") || strings.Contains(c.Host, ":") && net.ParseIP(c.Host) == nil {
		return fmt.Errorf("SMTP host must be a hostname or IP without port")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("SMTP port must be between 1 and 65535")
	}
	if _, err := Mailbox(c.From); err != nil {
		return fmt.Errorf("SMTP from: %w", err)
	}
	switch c.TLSMode {
	case "starttls", "tls":
	case "none":
		ip := net.ParseIP(c.Host)
		if c.Host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return fmt.Errorf("plaintext SMTP is permitted only on loopback")
		}
		if c.Username != "" || c.Password != "" {
			return fmt.Errorf("SMTP authentication requires TLS")
		}
	default:
		return fmt.Errorf("SMTP tls_mode must be starttls, tls, or none")
	}
	if strings.ContainsAny(c.Username, "\r\n\x00") {
		return fmt.Errorf("invalid SMTP username")
	}
	if (c.Username == "") != (c.Password == "") {
		return fmt.Errorf("SMTP username and password must be supplied together")
	}
	return nil
}
func ValidateEmailTemplate(t EmailTemplate) error {
	switch t.Style {
	case "", "brief", "focus", "ledger", "hyperframe":
		if strings.TrimSpace(t.HTML) != "" {
			return fmt.Errorf("email template html is reserved for custom style")
		}
	case "custom":
		if strings.TrimSpace(t.HTML) == "" {
			return fmt.Errorf("custom email template requires html")
		}
		if len(t.HTML) > 131072 {
			return fmt.Errorf("custom email template html is too long")
		}
		for _, placeholder := range []string{"{{date}}", "{{timezone}}", "{{introduction}}", "{{closing}}", "{{yesterday}}", "{{unresolved}}"} {
			if strings.Count(t.HTML, placeholder) != 1 {
				return fmt.Errorf("custom email template must contain %s exactly once", placeholder)
			}
		}
		replacer := strings.NewReplacer(
			"{{date}}", "", "{{timezone}}", "", "{{introduction}}", "", "{{closing}}", "",
			"{{yesterday}}", "", "{{unresolved}}", "", "{{owners}}", "", "{{commits}}", "",
			"{{charts}}", "", "{{overview_cards}}", "", "{{analysis}}", "", "{{warnings}}", "",
		)
		rest := replacer.Replace(t.HTML)
		if strings.Contains(rest, "{{") || strings.Contains(rest, "}}") {
			return fmt.Errorf("custom email template supports only date, timezone, introduction, closing, yesterday, unresolved, owners, commits, charts, overview_cards, analysis, and warnings variables")
		}
	default:
		return fmt.Errorf("email template style must be brief, focus, ledger, hyperframe, or custom")
	}
	if strings.ContainsAny(t.Subject, "\r\n\x00") {
		return fmt.Errorf("email subject must be a single line")
	}
	if len(t.Subject) > 512 || len(t.Introduction) > 8000 || len(t.Closing) > 4000 {
		return fmt.Errorf("email template is too long")
	}
	for _, text := range []string{t.Subject, t.Introduction, t.Closing} {
		rest := strings.NewReplacer("{{date}}", "", "{{timezone}}", "").Replace(text)
		if strings.Contains(rest, "{{") || strings.Contains(rest, "}}") {
			return fmt.Errorf("template supports only {{date}} and {{timezone}} variables")
		}
	}
	return nil
}

// ValidateEmailTemplateStyle checks only the renderer style token. Renderers
// already receive expanded custom HTML, so they must not re-run full template
// validation against an empty source template.
func ValidateEmailTemplateStyle(style string) error {
	switch style {
	case "", "brief", "focus", "ledger", "hyperframe", "custom":
		return nil
	default:
		return fmt.Errorf("email template style must be brief, focus, ledger, hyperframe, or custom")
	}
}
func ValidateDailyJiraEmail(c DailyJiraEmailConfig) error {
	c = c.Normalized()
	if err := ValidateConfluenceSync(c.Confluence); err != nil {
		return err
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("invalid email timezone")
	}
	parts := strings.Split(c.SendTime, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return fmt.Errorf("send_time must be HH:MM")
	}
	hour, eh := strconv.Atoi(parts[0])
	minute, em := strconv.Atoi(parts[1])
	if eh != nil || em != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return fmt.Errorf("send_time must be HH:MM")
	}
	if eh == nil && em == nil && fmt.Sprintf("%02d:%02d", hour, minute) != c.SendTime {
		return fmt.Errorf("send_time must be HH:MM")
	}
	if len(c.Recipients) > 100 {
		return fmt.Errorf("at most 100 email recipients are allowed")
	}
	if c.Enabled && len(c.Recipients) == 0 {
		return fmt.Errorf("daily email requires recipients")
	}
	for _, recipient := range c.Recipients {
		if _, err := Mailbox(recipient); err != nil {
			return err
		}
	}
	if len(c.ProjectGroups) > 100 {
		return fmt.Errorf("at most 100 project groups are allowed")
	}
	for _, g := range c.ProjectGroups {
		if strings.TrimSpace(g.Name) == "" {
			return fmt.Errorf("project group name is required")
		}
		if len(g.Owners) == 0 && len(g.Projects) == 0 {
			return fmt.Errorf("project group requires at least one owner or project")
		}
		if len(g.Name) > 256 {
			return fmt.Errorf("project group name is too long")
		}
		if len(g.Owners) > 50 {
			return fmt.Errorf("at most 50 owners per project group are allowed")
		}
		for _, o := range g.Owners {
			if len(o) > 128 {
				return fmt.Errorf("project group owner name is too long")
			}
		}
		if len(g.Projects) > 50 {
			return fmt.Errorf("at most 50 projects per group are allowed")
		}
		for _, p := range g.Projects {
			if len(p) > 128 {
				return fmt.Errorf("project name is too long")
			}
		}
	}
	return ValidateEmailTemplate(c.Template)
}
func ValidateMailSettings(c Config) error {
	if c.SMTP.Enabled {
		if err := ValidateSMTP(c.SMTP); err != nil {
			return err
		}
	}
	if err := ValidateDailyJiraEmail(c.DailyJiraEmail); err != nil {
		return err
	}
	if c.DailyJiraEmail.Enabled && !c.SMTP.Enabled {
		return fmt.Errorf("daily email requires enabled SMTP")
	}
	return nil
}
