package config

import "testing"

func TestPerformanceBrainConfigNormalizedUsesThreeMonthDefaults(t *testing.T) {
	actual := (PerformanceBrainConfig{}).Normalized()
	if actual.Enabled {
		t.Fatal("existing deployments must remain disabled until explicitly configured")
	}
	if actual.IntervalMinutes != 60 || actual.AssessmentWindowDays != 90 || actual.RetentionDays != 90 {
		t.Fatalf("unexpected timing defaults: %+v", actual)
	}
	if actual.FormulaVersion != "v6.0" || actual.PublicationMode != "shadow" || actual.EvidenceCoverageGate != 0.70 || actual.MinimumSamples != 5 || actual.MinimumExposureDays != 30 {
		t.Fatalf("unexpected scoring defaults: %+v", actual)
	}
	if !*actual.DemandMetricsEnabled || !*actual.BugMetricsEnabled || !*actual.CodeMetricsEnabled || !*actual.JiraHistoryEnabled || !*actual.GitDedupeEnabled {
		t.Fatalf("v6 feature defaults must be enabled: %+v", actual)
	}
	if actual.BusyRetryAttempts != 3 || actual.BusyRetryDelayMS != 200 {
		t.Fatalf("unexpected busy retry defaults: %+v", actual)
	}
}

func TestPerformanceBrainConfigNormalizedKeepsOperationalOverridesAndLocksScorecard(t *testing.T) {
	actual := (PerformanceBrainConfig{
		Enabled: true, IntervalMinutes: 15, AssessmentWindowDays: 60, RetentionDays: 120,
		FormulaVersion: "personnel-v2", PublicationMode: "formal", EvidenceCoverageGate: 0.90, MinimumSamples: 8, MinimumExposureDays: 45,
		BusyRetryAttempts: 5, BusyRetryDelayMS: 350,
	}).Normalized()
	if !actual.Enabled || actual.IntervalMinutes != 15 || actual.AssessmentWindowDays != 60 || actual.RetentionDays != 120 ||
		actual.FormulaVersion != "v6.0" || actual.PublicationMode != "formal" || actual.EvidenceCoverageGate != 0.70 || actual.MinimumSamples != 5 || actual.MinimumExposureDays != 30 ||
		actual.BusyRetryAttempts != 5 || actual.BusyRetryDelayMS != 350 {
		t.Fatalf("operational overrides or scorecard contract changed: %+v", actual)
	}
}
