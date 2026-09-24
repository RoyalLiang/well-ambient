package config

import "testing"

func TestEmailTemplateStylesDefaultAndAllowlist(t *testing.T) {
	if got := DefaultEmailTemplate().Style; got != "brief" {
		t.Fatalf("default style=%q", got)
	}
	legacy := DailyJiraEmailConfig{Template: EmailTemplate{Subject: "已有主题", Introduction: "已有文案"}}.Normalized()
	if legacy.Template.Style != "brief" || legacy.Template.Subject != "已有主题" || legacy.Template.Introduction != "已有文案" || legacy.Template.Closing != "" {
		t.Fatalf("legacy template changed: %+v", legacy.Template)
	}
	for _, style := range []string{"", "brief", "focus", "ledger", "hyperframe"} {
		if err := ValidateEmailTemplate(EmailTemplate{Style: style}); err != nil {
			t.Errorf("valid style %q: %v", style, err)
		}
	}
	custom := EmailTemplate{Style: "custom", Subject: "主题", Introduction: "开场", Closing: "结尾", HTML: "<div>{{introduction}}</div><div>{{closing}}</div><span>{{date}}</span><span>{{timezone}}</span><div>{{yesterday}}</div><div>{{unresolved}}</div>"}
	if err := ValidateEmailTemplate(custom); err != nil {
		t.Fatalf("valid custom template: %v", err)
	}
	for _, style := range []string{"BRIEF", "<script>", "brief\" onload=\"evil()", "focus; background:url(https://example.test)"} {
		if err := ValidateEmailTemplate(EmailTemplate{Style: style}); err == nil {
			t.Errorf("unsafe style accepted: %q", style)
		}
	}
	if err := ValidateEmailTemplate(EmailTemplate{Style: "custom", Subject: "主题", Introduction: "开场", Closing: "结尾"}); err == nil {
		t.Error("custom template without html accepted")
	}
	for _, missing := range []string{"{{yesterday}}", "{{unresolved}}"} {
		html := "<div>{{introduction}}</div><div>{{closing}}</div><span>{{date}}</span><span>{{timezone}}</span>"
		if missing == "{{yesterday}}" {
			html += "<div>{{unresolved}}</div>"
		} else {
			html += "<div>{{yesterday}}</div>"
		}
		if err := ValidateEmailTemplate(EmailTemplate{Style: "custom", Subject: "主题", Introduction: "开场", Closing: "结尾", HTML: html}); err == nil {
			t.Errorf("custom template without %s accepted", missing)
		}
	}
}
