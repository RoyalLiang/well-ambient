package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"well-ambient/internal/codereview"
	"well-ambient/internal/db"
)

type StrongestBrainReviewIntelligenceResponse struct {
	GeneratedAt      string                                  `json:"generated_at"`
	Summary          StrongestBrainReviewIntelligenceSummary `json:"summary"`
	ActiveSkills     []StrongestBrainReviewSkillStatus       `json:"active_skills"`
	Versions         []StrongestBrainReviewVersionMetrics    `json:"versions"`
	Recommendations  []string                                `json:"recommendations"`
	RecentExceptions []StrongestBrainReviewException         `json:"recent_exceptions"`
}

type StrongestBrainReviewIntelligenceSummary struct {
	Total            int `json:"total"`
	Completed        int `json:"completed"`
	Partial          int `json:"partial"`
	Failed           int `json:"failed"`
	Active           int `json:"active"`
	Retried          int `json:"retried"`
	EvidenceGapRuns  int `json:"evidence_gap_runs"`
	HighRiskFindings int `json:"high_risk_findings"`
	Questions        int `json:"questions"`
}

type StrongestBrainReviewSkillStatus struct {
	ID               uint   `json:"id"`
	Version          int    `json:"version"`
	Name             string `json:"name"`
	ScopeType        string `json:"scope_type"`
	ScopeID          string `json:"scope_id"`
	ContentHash      string `json:"content_hash"`
	ValidationStatus string `json:"validation_status"`
	ActivatedAt      string `json:"activated_at"`
}

type StrongestBrainReviewVersionMetrics struct {
	SkillVersionID   uint   `json:"skill_version_id"`
	Version          int    `json:"version"`
	Name             string `json:"name"`
	PromptVersion    string `json:"prompt_version"`
	Runs             int    `json:"runs"`
	Completed        int    `json:"completed"`
	Partial          int    `json:"partial"`
	Failed           int    `json:"failed"`
	Retried          int    `json:"retried"`
	EvidenceGapRuns  int    `json:"evidence_gap_runs"`
	HighRiskFindings int    `json:"high_risk_findings"`
	Questions        int    `json:"questions"`
}

type StrongestBrainReviewException struct {
	RunID            uint   `json:"run_id"`
	ProjectID        string `json:"project_id"`
	Repo             string `json:"repo"`
	Kind             string `json:"kind"`
	Ref              string `json:"ref"`
	Status           string `json:"status"`
	SkillVersionID   uint   `json:"skill_version_id"`
	SkillVersion     int    `json:"skill_version"`
	SkillName        string `json:"skill_name"`
	HighRiskFindings int    `json:"high_risk_findings"`
	Questions        int    `json:"questions"`
	UpdatedAt        string `json:"updated_at"`
}

func (s *Server) handleGetStrongestBrainReviewIntelligence(w http.ResponseWriter, r *http.Request) {
	if db.DB == nil {
		http.Error(w, "Database not initialized", http.StatusInternalServerError)
		return
	}
	limit := boundedQueryLimit(r, 500, 1000)
	projectID := r.URL.Query().Get("project_id")
	query := db.DB.WithContext(r.Context()).Order("created_at DESC, id DESC").Limit(limit)
	if projectID != "" {
		if _, err := s.codeReview.Repo(projectID); err != nil {
			http.Error(w, "unknown configured project_id", http.StatusBadRequest)
			return
		}
		query = query.Where("project_id = ?", projectID)
	} else {
		ids := make([]string, 0, len(s.codeReview.Config().GitLab.Repos))
		for _, repo := range s.codeReview.Config().GitLab.Repos {
			if repo.ProjectID != "" {
				ids = append(ids, repo.ProjectID)
			}
		}
		if len(ids) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("project_id IN ?", ids)
		}
	}
	var runs []db.CodeReviewRun
	if err := query.Find(&runs).Error; err != nil {
		http.Error(w, "Failed to read code review intelligence", http.StatusInternalServerError)
		return
	}
	var active []db.SolutionPromptTemplate
	if err := db.DB.WithContext(r.Context()).
		Where("purpose = ? AND status = ?", "code_review", "active").
		Order("scope_type ASC, scope_id ASC, version DESC").
		Find(&active).Error; err != nil {
		http.Error(w, "Failed to read active review skills", http.StatusInternalServerError)
		return
	}
	response := buildStrongestBrainReviewIntelligence(runs, active, time.Now())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func buildStrongestBrainReviewIntelligence(
	runs []db.CodeReviewRun,
	active []db.SolutionPromptTemplate,
	now time.Time,
) StrongestBrainReviewIntelligenceResponse {
	response := StrongestBrainReviewIntelligenceResponse{
		GeneratedAt: formatDateTime(now), ActiveSkills: []StrongestBrainReviewSkillStatus{},
		Versions: []StrongestBrainReviewVersionMetrics{}, Recommendations: []string{},
		RecentExceptions: []StrongestBrainReviewException{},
	}
	for _, skill := range active {
		activatedAt := ""
		if skill.ActivatedAt != nil {
			activatedAt = formatDateTime(*skill.ActivatedAt)
		}
		response.ActiveSkills = append(response.ActiveSkills, StrongestBrainReviewSkillStatus{
			ID: skill.ID, Version: skill.Version, Name: skill.Name, ScopeType: skill.ScopeType,
			ScopeID: skill.ScopeID, ContentHash: skill.ContentHash,
			ValidationStatus: skill.ValidationStatus, ActivatedAt: activatedAt,
		})
	}
	metrics := map[string]*StrongestBrainReviewVersionMetrics{}
	for _, run := range runs {
		response.Summary.Total++
		switch run.Status {
		case "completed":
			response.Summary.Completed++
		case "partial":
			response.Summary.Partial++
		case "failed":
			response.Summary.Failed++
		case "queued", "running":
			response.Summary.Active++
		}
		if run.RetryOfID > 0 {
			response.Summary.Retried++
		}
		key := fmt.Sprintf("%d:%s", run.SkillVersionID, run.PromptVersion)
		item := metrics[key]
		if item == nil {
			item = &StrongestBrainReviewVersionMetrics{
				SkillVersionID: run.SkillVersionID, Version: run.SkillVersion,
				Name: run.SkillName, PromptVersion: run.PromptVersion,
			}
			if item.Name == "" {
				item.Name = "legacy review runtime"
			}
			metrics[key] = item
		}
		item.Runs++
		if run.RetryOfID > 0 {
			item.Retried++
		}
		switch run.Status {
		case "completed":
			item.Completed++
		case "partial":
			item.Partial++
		case "failed":
			item.Failed++
		}
		var report codereview.Report
		if run.ReportJSON != "" && json.Unmarshal([]byte(run.ReportJSON), &report) == nil {
			high, questions := reviewReportSignals(report)
			item.HighRiskFindings += high
			item.Questions += questions
			response.Summary.HighRiskFindings += high
			response.Summary.Questions += questions
			if !report.EvidenceComplete || questions > 0 {
				item.EvidenceGapRuns++
				response.Summary.EvidenceGapRuns++
			}
			if (run.Status == "partial" || run.Status == "failed" || high > 0) &&
				len(response.RecentExceptions) < 20 {
				response.RecentExceptions = append(response.RecentExceptions, StrongestBrainReviewException{
					RunID: run.ID, ProjectID: run.ProjectID, Repo: run.Repo, Kind: run.Kind, Ref: run.Ref,
					Status: run.Status, SkillVersionID: run.SkillVersionID, SkillVersion: run.SkillVersion,
					SkillName: run.SkillName, HighRiskFindings: high, Questions: questions,
					UpdatedAt: formatDateTime(run.UpdatedAt),
				})
			}
		} else if run.Status == "partial" || run.Status == "failed" {
			item.EvidenceGapRuns++
			response.Summary.EvidenceGapRuns++
		}
	}
	for _, item := range metrics {
		response.Versions = append(response.Versions, *item)
	}
	sort.Slice(response.Versions, func(i, j int) bool {
		if response.Versions[i].SkillVersionID != response.Versions[j].SkillVersionID {
			return response.Versions[i].SkillVersionID > response.Versions[j].SkillVersionID
		}
		return response.Versions[i].PromptVersion > response.Versions[j].PromptVersion
	})
	response.Recommendations = reviewIntelligenceRecommendations(response.Summary, len(response.ActiveSkills))
	return response
}

func reviewReportSignals(report codereview.Report) (int, int) {
	high := 0
	for _, finding := range report.Findings {
		if finding.Severity == "high" {
			high++
		}
	}
	return high, len(report.Questions)
}

func reviewIntelligenceRecommendations(summary StrongestBrainReviewIntelligenceSummary, activeSkills int) []string {
	result := []string{}
	if activeSkills == 0 {
		result = append(result, "启用一个已验证的全局代码评审技能；当前新事件应停止入队。")
	}
	if summary.Total == 0 {
		return append(result, "暂无代码评审运行样本；先完成受控试运行再调整技能。")
	}
	if summary.Partial*5 >= summary.Total {
		result = append(result, "部分评审占比偏高：优先补齐仓库知识范围、diff 完整性和业务规则。")
	}
	if summary.Failed*10 >= summary.Total {
		result = append(result, "评审失败率偏高：检查模型可用性、输出结构和在线技能验证结果。")
	}
	if summary.EvidenceGapRuns*5 >= summary.Total {
		result = append(result, "证据缺口偏高：扩充系统知识与跨服务契约，避免通过提示词猜测业务事实。")
	}
	if summary.Retried*10 >= summary.Total {
		result = append(result, "人工重评率偏高：对失败原因按技能版本分组，验证后再发布新版本。")
	}
	if summary.HighRiskFindings > summary.Completed {
		result = append(result, "高风险发现密度较高：将重复出现的缺陷模式沉淀为仓库补充规则和回归测试。")
	}
	if len(result) == 0 {
		result = append(result, "当前评审运行稳定；保持版本冻结，通过新增证据与人工反馈迭代下一草稿。")
	}
	return result
}
