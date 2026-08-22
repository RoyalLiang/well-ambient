package performance

import (
	"testing"

	"well-ambient/internal/db"
)

func TestV6MetricBoundariesMatchAssessmentContract(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		ratio float64
		want  float64
	}{
		{name: "D01 below two point boundary", code: metricDemandCompletion, ratio: 0.49, want: 1},
		{name: "D01 two points", code: metricDemandCompletion, ratio: 0.50, want: 2},
		{name: "D01 three points", code: metricDemandCompletion, ratio: 0.70, want: 3},
		{name: "D01 four points", code: metricDemandCompletion, ratio: 0.85, want: 4},
		{name: "D01 five points", code: metricDemandCompletion, ratio: 0.95, want: 5},
		{name: "D02 two points", code: metricDemandOnTime, ratio: 0.55, want: 2},
		{name: "D02 five points", code: metricDemandOnTime, ratio: 0.97, want: 5},
		{name: "B01 five points", code: metricDefectDensity, ratio: 0.15, want: 5},
		{name: "B01 four points", code: metricDefectDensity, ratio: 0.35, want: 4},
		{name: "B01 one point", code: metricDefectDensity, ratio: 0.91, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rule, ok := metricRuleFor(test.code)
			if !ok {
				t.Fatalf("missing rule %s", test.code)
			}
			if got := scoreForRatio(rule, test.ratio); got != test.want {
				t.Fatalf("scoreForRatio(%s, %.2f) = %.0f, want %.0f", test.code, test.ratio, got, test.want)
			}
		})
	}
}

func TestV6RulesWeightsFactorsAndRiskPenaltiesAreFrozen(t *testing.T) {
	if defaultFormulaVersion != "v6.0" || defaultCoverageGate != 0.70 || defaultMinimumSamples != 5 || defaultMinimumExposureDays != 30 {
		t.Fatalf("v6 global defaults drifted: formula=%s coverage=%.2f samples=%d exposure=%d", defaultFormulaVersion, defaultCoverageGate, defaultMinimumSamples, defaultMinimumExposureDays)
	}
	wantMinimumSamples := map[string]int{
		metricDemandCompletion: 5,
		metricDemandOnTime:     3,
		metricDefectDensity:    5,
	}
	weight := 0.0
	for code, want := range wantMinimumSamples {
		rule, ok := metricRuleFor(code)
		if !ok || rule.MinimumSamples != want || !rule.Core {
			t.Fatalf("%s rule = %+v, want minimum %d and core", code, rule, want)
		}
		weight += rule.Weight
	}
	if len(v6MetricRules) != 3 || round2(weight) != 1 {
		t.Fatalf("v6 formal metrics=%d weight=%.2f, want 3 and 1", len(v6MetricRules), weight)
	}
	if demandLevelFactor("P0") != 1.4 || projectWeightFactor("P1") != 1.15 {
		t.Fatal("delivery coefficients drifted")
	}
	factor := deliveryFactor(db.TaskTelemetry{TaskID: "WA-1", EstimateDays: 2, Priority: "P1", Difficulty: "High"}, projectFactors{Priority: "P1"})
	if factor.DeliveryWeight != 2.76 || factor.ComplexityFactor != 0 || factor.StageFactor != 0 || factor.RoleFactor != 0 {
		t.Fatalf("v6 delivery factor still contains virtual multipliers: %+v", factor)
	}
	if duplicateChangePenalty(0.03) != 0 || duplicateChangePenalty(0.10) != 1 || duplicateChangePenalty(0.20) != 3 || duplicateChangePenalty(0.35) != 5 || duplicateChangePenalty(0.36) != 6 {
		t.Fatal("duplicate-change risk thresholds drifted")
	}
	if commitDensityPenalty(8) != 0 || commitDensityPenalty(10) != 1 || commitDensityPenalty(12) != 2 || commitDensityPenalty(14) != 3 || commitDensityPenalty(14.01) != 4 {
		t.Fatal("commit-density risk thresholds drifted")
	}
	if loss, ok := bugLoss("S1", "生产", 1); !ok || loss != 12 {
		t.Fatalf("S1 production confirmed loss = %.2f, %v; want 12, true", loss, ok)
	}
}
