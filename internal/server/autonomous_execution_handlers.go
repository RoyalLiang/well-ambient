package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/delivery"
)

type executionPreflightRequest struct {
	DemandSpecVersionID uint                  `json:"demand_spec_version_id"`
	Repo                string                `json:"repo"`
	Author              string                `json:"author"`
	Unavailable         []string              `json:"unavailable_reviewers"`
	ChangeSet           []delivery.FileAction `json:"change_set"`
	TestCommands        []string              `json:"test_commands"`
}

type sourceFileSnapshot struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type generateChangeSetRequest struct {
	DemandSpecVersionID uint                 `json:"demand_spec_version_id"`
	Repo                string               `json:"repo"`
	SourcePaths         []string             `json:"source_paths"`
	SourceFiles         []sourceFileSnapshot `json:"source_files"`
	Instructions        string               `json:"instructions"`
}

type generatedChangeSet struct {
	Summary      string                `json:"summary"`
	ChangeSet    []delivery.FileAction `json:"change_set"`
	TestCommands []string              `json:"test_commands"`
	Model        string                `json:"model,omitempty"`
}

type preflightCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type executionPreflightResponse struct {
	Ready              bool                        `json:"ready"`
	Checks             []preflightCheck            `json:"checks"`
	Blockers           []string                    `json:"blockers"`
	ChangedPaths       []string                    `json:"changed_paths"`
	ReviewerResolution delivery.ReviewerResolution `json:"reviewer_resolution"`
	Spec               *demandSpecDTO              `json:"spec,omitempty"`
	ReviewContract     *reviewContractDTO          `json:"review_contract,omitempty"`
}

func (s *Server) handleExecutionPreflight(w http.ResponseWriter, r *http.Request) {
	var req executionPreflightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	response := s.buildExecutionPreflight(req)
	status := http.StatusOK
	if !response.Ready {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, response)
}

func (s *Server) handleGenerateExecutionChangeSet(w http.ResponseWriter, r *http.Request) {
	var req generateChangeSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	var spec db.DemandSpecVersion
	if req.DemandSpecVersionID == 0 || db.DB.First(&spec, req.DemandSpecVersionID).Error != nil {
		http.Error(w, "demand spec not found", http.StatusNotFound)
		return
	}
	if spec.Status != delivery.SpecFrozen {
		http.Error(w, "demand spec must be frozen", http.StatusConflict)
		return
	}
	repo := s.configuredRepo(req.Repo)
	if repo == nil || !containsStringFold(decodeStringList(spec.MappedReposJSON), req.Repo) {
		http.Error(w, "repository is not mapped and configured", http.StatusUnprocessableEntity)
		return
	}
	if !s.config.AI.Enabled || strings.TrimSpace(s.config.AI.APIToken) == "" || strings.TrimSpace(s.config.AI.BaseURL) == "" {
		http.Error(w, "AI code generation is not configured", http.StatusUnprocessableEntity)
		return
	}

	sources := append([]sourceFileSnapshot{}, req.SourceFiles...)
	if len(req.SourcePaths) > 0 {
		if len(req.SourcePaths) > 12 {
			http.Error(w, "at most 12 source paths can be loaded", http.StatusBadRequest)
			return
		}
		client, err := newGitLabDeliveryClient(&s.config.GitLab)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		projectRef := firstNonBlank(repo.ProjectID, repo.Path)
		project, err := client.GetProject(r.Context(), projectRef)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		for _, sourcePath := range normalizeDeliveryStrings(req.SourcePaths) {
			_, blockers := delivery.ValidateFileActions([]delivery.FileAction{{Action: "update", Path: sourcePath}})
			if len(blockers) > 0 {
				http.Error(w, "invalid source path: "+sourcePath, http.StatusBadRequest)
				return
			}
			content, err := client.GetFile(r.Context(), projectRef, sourcePath, project.DefaultBranch)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			if len(content) > 20000 {
				content = content[:20000]
			}
			sources = append(sources, sourceFileSnapshot{Path: sourcePath, Content: content})
		}
	}
	if totalSourceBytes(sources) > 80000 {
		http.Error(w, "source snapshot exceeds the 80KB generation budget", http.StatusBadRequest)
		return
	}

	specJSON, _ := json.Marshal(demandSpecFromModel(spec))
	sourceJSON, _ := json.Marshal(sources)
	systemPrompt := `你是受控自治交付系统中的代码变更生成器。只能输出严格 JSON，不得输出 Markdown。输出字段为 summary、change_set、test_commands。change_set 的 action 只能是 create 或 update；update 只能修改输入 source_files 中已经提供的路径；禁止删除文件、禁止修改 .git、禁止父目录路径、禁止输出 shell 脚本作为执行指令。test_commands 只描述应由 GitLab CI 执行的仓库标准检查。`
	userPrompt := fmt.Sprintf("[冻结需求规格]\n%s\n\n[目标仓库]\n%s\n\n[允许更新的源文件快照]\n%s\n\n[补充指令]\n%s", string(specJSON), req.Repo, string(sourceJSON), strings.TrimSpace(req.Instructions))
	raw, err := queryServerLLM(s.config, systemPrompt, userPrompt)
	if err != nil {
		http.Error(w, "AI code generation failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	generated, err := parseGeneratedChangeSet(raw, sources)
	if err != nil {
		http.Error(w, "AI returned an unsafe change set: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	generated.Model = s.config.AI.Model
	writeJSON(w, http.StatusOK, generated)
}

func parseGeneratedChangeSet(raw string, sources []sourceFileSnapshot) (generatedChangeSet, error) {
	var result generatedChangeSet
	if err := json.Unmarshal([]byte(cleanJSONContent(raw)), &result); err != nil {
		return result, fmt.Errorf("invalid JSON: %w", err)
	}
	paths, blockers := delivery.ValidateFileActions(result.ChangeSet)
	if len(result.ChangeSet) == 0 {
		blockers = append(blockers, "change_set is empty")
	}
	allowedUpdates := map[string]bool{}
	for _, source := range sources {
		allowedUpdates[strings.ToLower(strings.TrimSpace(strings.ReplaceAll(source.Path, "\\", "/")))] = true
	}
	for _, action := range result.ChangeSet {
		operation := delivery.NormalizeState(action.Action)
		filePath := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(action.Path, "\\", "/")))
		if operation == "delete" {
			blockers = append(blockers, "AI generated deletions are not allowed")
		}
		if operation == "update" && !allowedUpdates[filePath] {
			blockers = append(blockers, "AI may update only supplied source files: "+action.Path)
		}
	}
	result.TestCommands = normalizeDeliveryStrings(result.TestCommands)
	if len(result.TestCommands) == 0 {
		blockers = append(blockers, "test_commands is empty")
	}
	if len(blockers) > 0 {
		return result, fmt.Errorf("%s", strings.Join(normalizeDeliveryStrings(blockers), "; "))
	}
	_ = paths
	return result, nil
}

func totalSourceBytes(sources []sourceFileSnapshot) int {
	total := 0
	for _, source := range sources {
		total += len(source.Path) + len(source.Content)
	}
	return total
}

func (s *Server) buildExecutionPreflight(req executionPreflightRequest) executionPreflightResponse {
	response := executionPreflightResponse{Checks: make([]preflightCheck, 0), Blockers: make([]string, 0)}
	addCheck := func(name string, passed bool, detail string) {
		response.Checks = append(response.Checks, preflightCheck{Name: name, Passed: passed, Detail: detail})
		if !passed {
			response.Blockers = append(response.Blockers, detail)
		}
	}

	var spec db.DemandSpecVersion
	if req.DemandSpecVersionID == 0 || db.DB.First(&spec, req.DemandSpecVersionID).Error != nil {
		addCheck("frozen_spec", false, "demand spec not found")
		return response
	}
	specDTO := demandSpecFromModel(spec)
	response.Spec = &specDTO
	addCheck("frozen_spec", spec.Status == delivery.SpecFrozen, "demand spec must be frozen")

	var contract db.ReviewContract
	contractFound := db.DB.Where("demand_spec_version_id = ?", spec.ID).First(&contract).Error == nil
	addCheck("approved_review_contract", contractFound && contract.Status == delivery.ReviewApproved, "approved review contract is required")
	if contractFound {
		contractDTO := reviewContractFromModel(contract)
		response.ReviewContract = &contractDTO
	}

	repo := strings.TrimSpace(req.Repo)
	mapped := containsStringFold(decodeStringList(spec.MappedReposJSON), repo)
	configured := s.configuredRepo(repo) != nil
	addCheck("mapped_repository", repo != "" && mapped, "repository must be mapped by the frozen spec")
	addCheck("configured_repository", configured, "repository must exist in GitLab configuration")

	paths, changeBlockers := delivery.ValidateFileActions(req.ChangeSet)
	response.ChangedPaths = paths
	changeSetValid := len(req.ChangeSet) > 0 && len(changeBlockers) == 0
	changeSetDetail := "change set contains safe, unique file actions"
	if !changeSetValid {
		changeSetDetail = firstBlocker("change set must contain safe, unique file actions", changeBlockers)
	}
	addCheck("validated_change_set", changeSetValid, changeSetDetail)
	commands := normalizeDeliveryStrings(req.TestCommands)
	testDetail := "required test policy is present"
	if len(commands) == 0 {
		testDetail = "at least one required test command is needed"
	}
	addCheck("test_policy", len(commands) > 0, testDetail)

	if contractFound {
		rules := []delivery.ReviewPathRule{}
		_ = json.Unmarshal([]byte(contract.ProtectedPathRulesJSON), &rules)
		resolution := delivery.ResolveReviewers(delivery.ReviewerResolutionInput{
			RequiredRoles:      decodeStringList(contract.RequiredRolesJSON),
			ReviewerCandidates: decodeStringList(contract.ReviewerCandidatesJSON),
			MinimumApprovals:   contract.MinimumApprovals,
			AcceptanceOwner:    contract.AcceptanceOwner,
			ProtectedPathRules: rules,
			ChangedPaths:       paths,
			Author:             firstNonBlank(req.Author, authenticatedFallbackAuthor(spec)),
			Unavailable:        req.Unavailable,
		})
		response.ReviewerResolution = resolution
		addCheck("reviewer_resolution", resolution.Resolved, firstNonBlank(resolution.Reason, "reviewers could not be resolved"))
		if resolution.Resolved && contract.Status == delivery.ReviewApproved {
			contract.ResolvedReviewersJSON = encodeJSON(resolution.Reviewers)
			contract.ResolutionStatus = "resolved"
			contract.ResolutionReason = resolution.Reason
			_ = db.DB.Save(&contract).Error
		}
	} else {
		addCheck("reviewer_resolution", false, "review contract not found")
	}

	response.Blockers = normalizeDeliveryStrings(response.Blockers)
	response.Ready = len(response.Blockers) == 0
	return response
}

func (s *Server) configuredRepo(name string) *config.RepoMapping {
	for index := range s.config.GitLab.Repos {
		repo := &s.config.GitLab.Repos[index]
		if strings.EqualFold(strings.TrimSpace(repo.Name), strings.TrimSpace(name)) ||
			strings.EqualFold(strings.TrimSpace(repo.Path), strings.TrimSpace(name)) ||
			strings.EqualFold(strings.TrimSpace(repo.ProjectID), strings.TrimSpace(name)) {
			return repo
		}
	}
	return nil
}

func containsStringFold(items []string, candidate string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}

func firstBlocker(fallback string, blockers []string) string {
	if len(blockers) > 0 {
		return blockers[0]
	}
	return fallback
}

func authenticatedFallbackAuthor(spec db.DemandSpecVersion) string {
	return firstNonBlank(spec.AuthoredBy, spec.ReviewedBy)
}
