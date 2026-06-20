package server

import "testing"

func TestParseDeconstructResponseContentBackfillsAnalysisForLegacyOutput(t *testing.T) {
	raw := `{
		"mappedRepos": ["backend-core"],
		"tasks": [
			{
				"id": "task-108",
				"repo": "backend-core",
				"title": "实现需求验收标准持久化",
				"assignee": "Eddie",
				"priority": "High",
				"complexity": "Medium"
			}
		]
	}`

	result, cleaned, err := parseDeconstructResponseContent(raw)
	if err != nil {
		t.Fatalf("parseDeconstructResponseContent() error = %v, cleaned = %s", err, cleaned)
	}

	if len(result.Tasks) != 1 {
		t.Fatalf("Tasks length = %d, want 1", len(result.Tasks))
	}
	if result.Analysis.CompletenessScore != 60 {
		t.Fatalf("CompletenessScore = %d, want legacy fallback 60", result.Analysis.CompletenessScore)
	}
	if result.Analysis.Confidence != 0.5 {
		t.Fatalf("Confidence = %v, want fallback 0.5", result.Analysis.Confidence)
	}
	if result.Analysis.MissingInfo == nil {
		t.Fatalf("MissingInfo should be an empty slice, got nil")
	}
	if result.Analysis.Risks == nil {
		t.Fatalf("Risks should be an empty slice, got nil")
	}
	if result.Analysis.AcceptanceCriteria == nil {
		t.Fatalf("AcceptanceCriteria should be an empty slice, got nil")
	}
}

func TestParseDeconstructResponseContentNormalizesAnalysis(t *testing.T) {
	raw := "```json\n" + `{
		"mappedRepos": null,
		"tasks": null,
		"analysis": {
			"completeness_score": 120,
			"missing_info": ["  确认权限边界  ", ""],
			"risks": ["跨系统接口未冻结"],
			"dependencies": null,
			"acceptance_criteria": ["完成端到端验收"],
			"schedule_notes": ["后端接口先行"],
			"meeting_questions": ["是否需要灰度发布？"],
			"confidence": 76
		}
	}` + "\n```"

	result, cleaned, err := parseDeconstructResponseContent(raw)
	if err != nil {
		t.Fatalf("parseDeconstructResponseContent() error = %v, cleaned = %s", err, cleaned)
	}

	if result.MappedRepos == nil {
		t.Fatalf("MappedRepos should be an empty slice, got nil")
	}
	if result.Tasks == nil {
		t.Fatalf("Tasks should be an empty slice, got nil")
	}
	if result.Analysis.CompletenessScore != 100 {
		t.Fatalf("CompletenessScore = %d, want capped 100", result.Analysis.CompletenessScore)
	}
	if result.Analysis.Confidence != 0.76 {
		t.Fatalf("Confidence = %v, want normalized 0.76", result.Analysis.Confidence)
	}
	if len(result.Analysis.MissingInfo) != 1 || result.Analysis.MissingInfo[0] != "确认权限边界" {
		t.Fatalf("MissingInfo = %#v, want trimmed single item", result.Analysis.MissingInfo)
	}
	if result.Analysis.Dependencies == nil {
		t.Fatalf("Dependencies should be an empty slice, got nil")
	}
}

func TestParseDeconstructResponseContentKeepsExplicitZeroCompleteness(t *testing.T) {
	raw := `{
		"mappedRepos": [],
		"tasks": [],
		"analysis": {
			"completeness_score": 0,
			"missing_info": ["缺少核心目标"],
			"risks": [],
			"dependencies": [],
			"acceptance_criteria": [],
			"schedule_notes": [],
			"meeting_questions": [],
			"confidence": 0.2
		}
	}`

	result, cleaned, err := parseDeconstructResponseContent(raw)
	if err != nil {
		t.Fatalf("parseDeconstructResponseContent() error = %v, cleaned = %s", err, cleaned)
	}

	if result.Analysis.CompletenessScore != 0 {
		t.Fatalf("CompletenessScore = %d, want explicit 0 preserved", result.Analysis.CompletenessScore)
	}
	if result.Analysis.Confidence != 0.2 {
		t.Fatalf("Confidence = %v, want explicit 0.2 preserved", result.Analysis.Confidence)
	}
}

func TestParseDeconstructResponseContentNormalizesEstimates(t *testing.T) {
	raw := `{
		"mappedRepos": ["backend-core", "frontend-dashboard"],
		"tasks": [
			{
				"id": "task-201",
				"repo": "backend-core",
				"title": "实现估算归档接口",
				"assignee": "Eddie",
				"priority": "High",
				"complexity": "High",
				"difficulty": "High",
				"estimated_days": 4,
				"estimate_basis": "涉及数据模型与回归测试"
			},
			{
				"id": "task-202",
				"repo": "frontend-dashboard",
				"title": "展示估算结果",
				"assignee": "Antigravity",
				"priority": "Medium",
				"complexity": "Medium"
			}
		],
		"analysis": {
			"completeness_score": 86,
			"overall_estimated_days": 6,
			"overall_difficulty": "High",
			"estimate_basis": "后端归档是关键路径，前端展示可并行",
			"missing_info": [],
			"risks": [],
			"dependencies": [],
			"acceptance_criteria": [],
			"schedule_notes": [],
			"meeting_questions": [],
			"confidence": 0.8
		}
	}`

	result, cleaned, err := parseDeconstructResponseContent(raw)
	if err != nil {
		t.Fatalf("parseDeconstructResponseContent() error = %v, cleaned = %s", err, cleaned)
	}

	if result.Analysis.OverallEstimatedDays != 6 {
		t.Fatalf("OverallEstimatedDays = %v, want 6", result.Analysis.OverallEstimatedDays)
	}
	if result.Analysis.OverallEstimatedHours != 48 {
		t.Fatalf("OverallEstimatedHours = %v, want 48", result.Analysis.OverallEstimatedHours)
	}
	if result.Tasks[0].EstimatedHours != 32 {
		t.Fatalf("Task 0 EstimatedHours = %v, want 32", result.Tasks[0].EstimatedHours)
	}
	if result.Tasks[1].EstimatedDays <= 0 {
		t.Fatalf("Task 1 should receive distributed estimate, got %v", result.Tasks[1].EstimatedDays)
	}
	if result.Tasks[1].Difficulty != "Medium" {
		t.Fatalf("Task 1 Difficulty = %q, want Medium", result.Tasks[1].Difficulty)
	}
	if result.Tasks[1].EstimateBasis == "" {
		t.Fatalf("Task 1 should receive fallback estimate basis")
	}
}

func TestParseDeconstructResponseContentUsesConfiguredWorkHours(t *testing.T) {
	raw := `{
		"mappedRepos": ["backend-core"],
		"tasks": [
			{
				"id": "task-301",
				"repo": "backend-core",
				"title": "按小时估算归档",
				"assignee": "Eddie",
				"priority": "Medium",
				"complexity": "Medium",
				"difficulty": "Medium",
				"estimated_hours": 15
			}
		],
		"analysis": {
			"completeness_score": 82,
			"overall_estimated_hours": 22.5,
			"overall_difficulty": "Medium",
			"missing_info": [],
			"risks": [],
			"dependencies": [],
			"acceptance_criteria": [],
			"schedule_notes": [],
			"meeting_questions": [],
			"confidence": 0.7
		}
	}`

	result, cleaned, err := parseDeconstructResponseContentWithWorkHours(raw, 7.5)
	if err != nil {
		t.Fatalf("parseDeconstructResponseContentWithWorkHours() error = %v, cleaned = %s", err, cleaned)
	}

	if result.Tasks[0].EstimatedHours != 15 {
		t.Fatalf("Task EstimatedHours = %v, want 15", result.Tasks[0].EstimatedHours)
	}
	if result.Tasks[0].EstimatedDays != 2 {
		t.Fatalf("Task EstimatedDays = %v, want 2 with 7.5h/day", result.Tasks[0].EstimatedDays)
	}
	if result.Analysis.OverallEstimatedHours != 22.5 {
		t.Fatalf("OverallEstimatedHours = %v, want 22.5", result.Analysis.OverallEstimatedHours)
	}
	if result.Analysis.OverallEstimatedDays != 3 {
		t.Fatalf("OverallEstimatedDays = %v, want 3 with 7.5h/day", result.Analysis.OverallEstimatedDays)
	}
}
