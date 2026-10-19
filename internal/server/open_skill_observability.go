package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

type openSubskillObservation struct {
	ID          string         `json:"id"`
	Calls       int            `json:"calls"`
	Succeeded   int            `json:"succeeded"`
	Failed      int            `json:"failed"`
	InProgress  int            `json:"in_progress"`
	SuccessRate *float64       `json:"success_rate"`
	AverageMS   *float64       `json:"average_ms"`
	ErrorCodes  map[string]int `json:"error_codes"`
	Conditions  []string       `json:"conditions"`
}
type openSkillCall struct {
	RequestID     string    `json:"request_id"`
	Subskill      string    `json:"subskill"`
	Tool          string    `json:"tool"`
	SourceID      string    `json:"source_id"`
	Transport     string    `json:"transport"`
	Outcome       string    `json:"outcome"`
	ErrorCode     string    `json:"error_code,omitempty"`
	DurationMS    int64     `json:"duration_ms"`
	CreatedAt     time.Time `json:"created_at"`
	PolicyVersion int       `json:"policy_version"`
}

func subskillForTool(tool string) string {
	for _, skill := range officialSkills {
		for _, canonical := range skill.tools {
			if tool == canonical || tool == "well-"+canonical || tool == "well-ambient-"+canonical {
				return skill.name
			}
		}
	}
	return ""
}

func (s *Server) handleGetOpenSkillsOverview(w http.ResponseWriter, r *http.Request) {
	if s.openAccess == nil || db.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skill_observation_unavailable"})
		return
	}
	report, err := s.openAccess.Intelligence(r.Context(), 24*time.Hour)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skill_observation_unavailable"})
		return
	}
	policy, policyErr := s.openAccess.ActivePolicy(r.Context())
	if policyErr != nil && openaccess.ErrorCode(policyErr) != "forbidden" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skill_policy_unavailable"})
		return
	}
	s.configMutationMu.Lock()
	features := s.openFeatures
	s.configMutationMu.Unlock()
	metrics := map[string]*openSubskillObservation{}
	durationSum := map[string]float64{}
	for _, child := range officialSkills {
		metrics[child.name] = &openSubskillObservation{ID: child.name, ErrorCodes: map[string]int{}, Conditions: []string{"实际调用仍需通过来源、项目或仓库、字段与配额校验"}}
	}
	total, successes, failures, inProgress := 0, 0, 0, 0
	for _, metric := range report.Invocations {
		id := subskillForTool(metric.ToolName)
		if id == "" {
			continue
		}
		child := metrics[id]
		child.Calls += metric.Total
		child.Succeeded += metric.Succeeded
		child.Failed += metric.Failed
		child.InProgress += metric.StartedCount
		durationSum[id] += metric.AverageMS * float64(metric.Total-metric.StartedCount)
		for code, count := range metric.ErrorCodes {
			child.ErrorCodes[code] += count
		}
		total += metric.Total
		successes += metric.Succeeded
		failures += metric.Failed
		inProgress += metric.StartedCount
	}
	observations := []openSubskillObservation{}
	for _, definition := range officialSkills {
		child := metrics[definition.name]
		completed := child.Succeeded + child.Failed
		if completed > 0 {
			rate := float64(child.Succeeded) / float64(completed)
			child.SuccessRate = &rate
			if child.Calls-child.InProgress == completed {
				avg := durationSum[definition.name] / float64(completed)
				child.AverageMS = &avg
			}
		}
		if policyErr != nil {
			child.Conditions = append(child.Conditions, "尚未配置开放发布策略")
		}
		if definition.name == "board-decision" {
			if !features.Prepare {
				child.Conditions = append(child.Conditions, "决策预备阶段未启用")
			}
			if !features.Execute {
				child.Conditions = append(child.Conditions, "决策执行阶段未启用；读取现有操作仍受只读阶段控制")
			}
		} else if !features.Read {
			child.Conditions = append(child.Conditions, "只读阶段未启用")
		}
		action := map[string]string{"jira-analysis": "jira.read", "board-decision": "decision.read", "code-review-reader": "review.read", "runtime-knowledge": "knowledge.read"}[definition.name]
		if policyErr == nil && !policy.AllowsAction(action) {
			child.Conditions = append(child.Conditions, "当前策略未发布 "+action)
		}
		observations = append(observations, *child)
	}
	var rows []db.CapabilityInvocation
	if err := db.DB.WithContext(r.Context()).Select("id, request_id, source_id, transport, tool_name, outcome, error_code, duration_ms, created_at, policy_version").Where("created_at >= ? AND created_at <= ?", report.WindowStart, report.GeneratedAt).Order("created_at DESC, id DESC").Limit(20).Find(&rows).Error; err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skill_calls_unavailable"})
		return
	}
	calls := []openSkillCall{}
	for _, row := range rows {
		calls = append(calls, openSkillCall{RequestID: row.RequestID, Subskill: subskillForTool(row.ToolName), Tool: row.ToolName, SourceID: row.SourceID, Transport: row.Transport, Outcome: row.Outcome, ErrorCode: row.ErrorCode, DurationMS: row.DurationMS, CreatedAt: row.CreatedAt, PolicyVersion: row.PolicyVersion})
	}
	var knowledge *db.RuntimeKnowledgeState
	var generation *db.RuntimeKnowledgeGeneration
	gaps := []string{}
	var state db.RuntimeKnowledgeState
	stateErr := db.DB.WithContext(r.Context()).Model(&db.RuntimeKnowledgeState{}).First(&state, 1).Error
	if stateErr == nil {
		knowledge = &state
		if state.GenerationID > 0 {
			var row db.RuntimeKnowledgeGeneration
			if err := db.DB.WithContext(r.Context()).First(&row, state.GenerationID).Error; err == nil {
				generation = &row
			} else {
				gaps = append(gaps, "knowledge_generation_unavailable")
			}
		}
	} else if !errors.Is(stateErr, gorm.ErrRecordNotFound) {
		gaps = append(gaps, "knowledge_status_unavailable")
	}
	var rate *float64
	if successes+failures > 0 {
		value := float64(successes) / float64(successes+failures)
		rate = &value
	}
	sort.Strings(gaps)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"generated_at": report.GeneratedAt, "window_start": report.WindowStart, "bundle": s.openSkillsCatalog(s.resolveRequestBaseURL(r))[0], "phases": features, "policy_version": policy.Version, "summary": map[string]any{"calls": total, "succeeded": successes, "failed": failures, "in_progress": inProgress, "success_rate": rate}, "subskills": observations, "recent_calls": calls, "recent_limit": 20, "knowledge": knowledge, "knowledge_generation": generation, "gaps": gaps, "notice": "发布状态不代表工具授权或实际调用；没有记录时仅表示当前窗口内暂无观测。知识刷新只覆盖已同步数据。"})
}
