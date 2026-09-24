package config

import "testing"

func TestEmailSettingsDefaultsAndValidation(t *testing.T) {
	defaults := DailyJiraEmailConfig{}.Normalized()
	if defaults.Enabled || defaults.IncludeCommits || defaults.SendTime != "09:00" || defaults.Timezone != "Asia/Shanghai" {
		t.Fatalf("unsafe defaults: %+v", defaults)
	}
	if defaults.Template.Subject == "" || defaults.Template.Introduction == "" || defaults.Template.Closing == "" {
		t.Fatal("default sending template must be complete")
	}
	custom := DailyJiraEmailConfig{Template: EmailTemplate{Subject: "Custom"}}.Normalized()
	if custom.Template.Introduction != "" || custom.Template.Closing != "" {
		t.Fatal("do not overwrite intentionally blank custom text")
	}
	if err := ValidateEmailTemplate(DefaultEmailTemplate()); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"+1:00", "-0:30", "24:00", "09:60", "9:00"} {
		c := defaults
		c.SendTime = value
		if ValidateDailyJiraEmail(c) == nil {
			t.Errorf("accepted time %q", value)
		}
	}
	for _, value := range []string{"invalid", "a@example.test\r\nBcc:x@example.test"} {
		c := defaults
		c.Recipients = []string{value}
		if ValidateDailyJiraEmail(c) == nil {
			t.Errorf("accepted recipient %q", value)
		}
	}
	defaults.Timezone = "Invalid/Zone"
	if ValidateDailyJiraEmail(defaults) == nil {
		t.Fatal("accepted invalid timezone")
	}
	if ValidateEmailTemplate(EmailTemplate{Subject: "{{password}}"}) == nil {
		t.Fatal("accepted unknown variable")
	}
	if ValidateMailSettings(Config{DailyJiraEmail: DailyJiraEmailConfig{Enabled: true, Recipients: []string{"a@example.test"}}}) == nil {
		t.Fatal("enabled daily without SMTP")
	}

	// Test ProjectGroups normalization and validation
	pgCfg := DailyJiraEmailConfig{
		ProjectGroups: []DailyJiraProjectGroup{
			{Name: " 盐田组 ", Owners: []string{" 白凌云 ", "梁志远", ""}, Projects: []string{" YT ", "盐田"}},
			{Name: "   ", Owners: []string{}, Projects: []string{}}, // should be pruned
		},
	}.Normalized()
	if len(pgCfg.ProjectGroups) != 1 {
		t.Fatalf("expected 1 normalized project group, got %d", len(pgCfg.ProjectGroups))
	}
	if pgCfg.ProjectGroups[0].Name != "盐田组" || len(pgCfg.ProjectGroups[0].Owners) != 2 || len(pgCfg.ProjectGroups[0].Projects) != 2 {
		t.Fatalf("unexpected group normalization: %+v", pgCfg.ProjectGroups[0])
	}
	if err := ValidateDailyJiraEmail(pgCfg); err != nil {
		t.Fatalf("valid project groups failed validation: %v", err)
	}
}
