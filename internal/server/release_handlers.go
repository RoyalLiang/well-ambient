package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	appconfig "well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/deliveryplanning"

	"gorm.io/gorm"
)

type releaseMutationRequest struct {
	ProjectKey  *string `json:"project_key"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	StartDate   *string `json:"start_date"`
	ReleaseDate *string `json:"release_date"`
}

type releasePlanItem struct {
	Release        db.ReleaseVersion `json:"release"`
	JiraIssueCount int64             `json:"jira_issue_count"`
}

type releaseJiraLinkRequest struct {
	JiraProjectKey string `json:"jira_project_key"`
	JiraVersionID  string `json:"jira_version_id"`
}

type releaseJiraIssueItem struct {
	WorkItemID         string `json:"work_item_id"`
	JiraKey            string `json:"jira_key"`
	JiraURL            string `json:"jira_url"`
	Title              string `json:"title"`
	IssueType          string `json:"issue_type"`
	Status             string `json:"status"`
	Assignee           string `json:"assignee"`
	ProjectKey         string `json:"project_key"`
	Linked             bool   `json:"linked"`
	CurrentReleaseID   uint   `json:"current_release_id,omitempty"`
	CurrentReleaseName string `json:"current_release_name,omitempty"`
}

type bulkReleaseJiraIssueRequest struct {
	WorkItemIDs []string `json:"work_item_ids"`
	Reason      string   `json:"reason"`
}

func (s *Server) handleListReleases(w http.ResponseWriter, r *http.Request) {
	projectKey := deliveryplanning.NormalizeProjectKey(r.URL.Query().Get("project_key"))
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !validReleaseStatus(status) {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_release_status", "release status must be planned, released, or archived")
		return
	}

	var releases []db.ReleaseVersion
	query := db.DB.WithContext(r.Context()).
		Order("release_date IS NULL, release_date ASC, name ASC")
	if projectKey != "" {
		query = query.Where("project_key = ?", projectKey)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Find(&releases).Error; err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "release_query_failed", err.Error())
		return
	}

	releaseIDs := make([]uint, 0, len(releases))
	for _, release := range releases {
		releaseIDs = append(releaseIDs, release.ID)
	}
	issueCountsByReleaseID := make(map[uint]int64, len(releaseIDs))
	if len(releaseIDs) > 0 {
		var counts []struct {
			ReleaseVersionID uint
			Count            int64
		}
		if err := db.DB.WithContext(r.Context()).
			Model(&db.WorkItemReleaseLink{}).
			Select("release_version_id, COUNT(*) AS count").
			Where(
				"release_version_id IN ? AND relation = ? AND active = ? AND is_primary = ?",
				releaseIDs,
				deliveryplanning.ReleaseTargetFix,
				true,
				true,
			).
			Group("release_version_id").
			Scan(&counts).Error; err != nil {
			writeDeliveryError(w, http.StatusInternalServerError, "release_issue_count_query_failed", err.Error())
			return
		}
		for _, count := range counts {
			issueCountsByReleaseID[count.ReleaseVersionID] = count.Count
		}
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	items := make([]releasePlanItem, 0, len(releases))
	for _, release := range releases {
		if search != "" {
			haystack := strings.ToLower(strings.Join([]string{
				release.Name,
				release.Description,
				release.ProjectKey,
				release.ExternalID,
			}, " "))
			if !strings.Contains(haystack, search) {
				continue
			}
		}
		items = append(items, releasePlanItem{
			Release:        release,
			JiraIssueCount: issueCountsByReleaseID[release.ID],
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

func (s *Server) handleListReleaseJiraIssues(w http.ResponseWriter, r *http.Request) {
	releaseID, err := parseReleaseID(r.PathValue("id"))
	if err != nil || releaseID == 0 {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_release_id", "release id must be a positive integer")
		return
	}
	var release db.ReleaseVersion
	if err := db.DB.WithContext(r.Context()).First(&release, releaseID).Error; err != nil {
		writeDomainError(w, err)
		return
	}
	if ok, err := requestCanAccessReleaseProject(r, release.ProjectKey); err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "project_scope_failed", err.Error())
		return
	} else if !ok {
		writeDeliveryError(w, http.StatusNotFound, "release_not_found", "release was not found in the current project scope")
		return
	}

	scope := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	if scope == "" {
		scope = "linked"
	}
	if scope != "linked" && scope != "candidates" {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_jira_issue_scope", "scope must be linked or candidates")
		return
	}
	limit := 100
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, parseErr := strconv.Atoi(rawLimit)
		if parseErr != nil || parsed < 1 {
			writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_limit", "limit must be a positive integer")
			return
		}
		if parsed > 200 {
			parsed = 200
		}
		limit = parsed
	}

	type jiraIssueRow struct {
		WorkItemID         string
		ExternalKey        string
		Title              string
		IssueType          string
		Status             string
		Assignee           string
		ProjectKey         string
		CurrentReleaseID   uint
		CurrentReleaseName string
	}
	rows := make([]jiraIssueRow, 0)
	query := db.DB.WithContext(r.Context()).
		Table("task_telemetries AS tasks").
		Select(`
			tasks.task_id AS work_item_id,
			tasks.external_key,
			tasks.title,
			tasks.issue_type,
			tasks.status,
			tasks.assignee,
			tasks.project_key,
			COALESCE(links.release_version_id, 0) AS current_release_id,
			COALESCE(current_release.name, '') AS current_release_name
		`).
		Joins(`
			LEFT JOIN work_item_release_links AS links
				ON links.work_item_id = tasks.task_id
				AND links.active = ?
				AND links.relation = ?
				AND links.is_primary = ?
		`, true, deliveryplanning.ReleaseTargetFix, true).
		Joins("LEFT JOIN release_versions AS current_release ON current_release.id = links.release_version_id").
		Where("tasks.source = ?", "jira").
		Where("tasks.project_key = ?", deliveryplanning.NormalizeProjectKey(release.ProjectKey)).
		Where("LOWER(tasks.issue_type) IN ?", []string{"demand", "requirement", "story", "bug", "defect", "缺陷", "故障"}).
		Order("tasks.last_update DESC, tasks.task_id ASC").
		Limit(limit)
	if scope == "linked" {
		query = query.Where("links.release_version_id = ?", release.ID)
	} else {
		query = query.Where("links.release_version_id IS NULL OR links.release_version_id <> ?", release.ID)
	}
	if search := strings.TrimSpace(r.URL.Query().Get("q")); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where(
			"(LOWER(tasks.task_id) LIKE ? OR LOWER(tasks.external_key) LIKE ? OR LOWER(tasks.title) LIKE ?)",
			pattern,
			pattern,
			pattern,
		)
	}
	if err := query.Scan(&rows).Error; err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "jira_issue_query_failed", err.Error())
		return
	}

	items := make([]releaseJiraIssueItem, 0, len(rows))
	for _, row := range rows {
		jiraKey := strings.TrimSpace(row.ExternalKey)
		if jiraKey == "" {
			jiraKey = strings.TrimSpace(row.WorkItemID)
		}
		items = append(items, releaseJiraIssueItem{
			WorkItemID:         row.WorkItemID,
			JiraKey:            jiraKey,
			JiraURL:            s.jiraIssuePageURL(jiraKey),
			Title:              row.Title,
			IssueType:          row.IssueType,
			Status:             row.Status,
			Assignee:           row.Assignee,
			ProjectKey:         deliveryplanning.NormalizeProjectKey(row.ProjectKey),
			Linked:             row.CurrentReleaseID == release.ID,
			CurrentReleaseID:   row.CurrentReleaseID,
			CurrentReleaseName: row.CurrentReleaseName,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"total":       len(items),
		"project_key": deliveryplanning.NormalizeProjectKey(release.ProjectKey),
		"scope":       scope,
	})
}

func (s *Server) handleBulkAddReleaseJiraIssues(w http.ResponseWriter, r *http.Request) {
	releaseID, err := parseReleaseID(r.PathValue("id"))
	if err != nil || releaseID == 0 {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_release_id", "release id must be a positive integer")
		return
	}
	var request bulkReleaseJiraIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	request.WorkItemIDs = normalizeReleaseWorkItemIDs(request.WorkItemIDs)
	if len(request.WorkItemIDs) == 0 {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "work_items_required", "choose at least one Jira issue")
		return
	}
	if len(request.WorkItemIDs) > 100 {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "work_item_limit_exceeded", "at most 100 Jira issues can be associated at once")
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Reason == "" {
		request.Reason = "associate Jira issues with local release"
	}

	var release db.ReleaseVersion
	if err := db.DB.WithContext(r.Context()).First(&release, releaseID).Error; err != nil {
		writeDomainError(w, err)
		return
	}
	if ok, err := requestCanAccessReleaseProject(r, release.ProjectKey); err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "project_scope_failed", err.Error())
		return
	} else if !ok {
		writeDeliveryError(w, http.StatusNotFound, "release_not_found", "release was not found in the current project scope")
		return
	}

	actor := authenticatedActor(r)
	associated := 0
	err = db.DB.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		var tasks []db.TaskTelemetry
		if err := tx.Where("task_id IN ?", request.WorkItemIDs).Find(&tasks).Error; err != nil {
			return err
		}
		taskByID := make(map[string]db.TaskTelemetry, len(tasks))
		for _, task := range tasks {
			taskByID[task.TaskID] = task
		}
		if len(taskByID) != len(request.WorkItemIDs) {
			return &deliveryplanning.DomainError{
				Code:       "jira_issue_not_found",
				Message:    "one or more Jira issues are not available in the local catalog",
				StatusCode: http.StatusNotFound,
			}
		}

		var existingLinks []db.WorkItemReleaseLink
		if err := tx.
			Where(
				"work_item_id IN ? AND relation = ? AND active = ? AND is_primary = ?",
				request.WorkItemIDs,
				deliveryplanning.ReleaseTargetFix,
				true,
				true,
			).
			Find(&existingLinks).Error; err != nil {
			return err
		}
		linkByWorkItemID := make(map[string]db.WorkItemReleaseLink, len(existingLinks))
		for _, link := range existingLinks {
			linkByWorkItemID[link.WorkItemID] = link
		}

		for _, workItemID := range request.WorkItemIDs {
			task := taskByID[workItemID]
			if !strings.EqualFold(strings.TrimSpace(task.Source), "jira") {
				return &deliveryplanning.DomainError{
					Code:       "jira_issue_required",
					Message:    fmt.Sprintf("%s is not a Jira issue", workItemID),
					StatusCode: http.StatusUnprocessableEntity,
				}
			}
			if deliveryplanning.NormalizeProjectKey(task.ProjectKey) != deliveryplanning.NormalizeProjectKey(release.ProjectKey) {
				return &deliveryplanning.DomainError{
					Code:       "release_project_mismatch",
					Message:    fmt.Sprintf("%s does not belong to release project %s", workItemID, release.ProjectKey),
					StatusCode: http.StatusConflict,
				}
			}
			if _, err := deliveryplanning.NormalizeIssueType(task.IssueType); err != nil {
				return err
			}
			if existing, ok := linkByWorkItemID[workItemID]; ok && existing.ReleaseVersionID != release.ID {
				return &deliveryplanning.DomainError{
					Code:       "jira_issue_already_linked",
					Message:    fmt.Sprintf("%s is already associated with another release", workItemID),
					StatusCode: http.StatusConflict,
				}
			}
		}

		service := deliveryplanning.NewService(tx)
		for _, workItemID := range request.WorkItemIDs {
			if existing, ok := linkByWorkItemID[workItemID]; ok && existing.ReleaseVersionID == release.ID {
				continue
			}
			task := taskByID[workItemID]
			targetReleaseID := release.ID
			if _, err := service.ApplyPlanningChange(r.Context(), deliveryplanning.PlanningCommand{
				WorkItemID:             workItemID,
				ExpectedRevision:       task.Revision,
				PrimaryTargetReleaseID: &targetReleaseID,
				Reason:                 request.Reason,
				Actor:                  actor,
				Source:                 "manual",
			}); err != nil {
				return err
			}
			associated++
		}
		return nil
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"release_id":       release.ID,
		"associated_count": associated,
	})
}

func (s *Server) handleDeleteReleaseJiraIssue(w http.ResponseWriter, r *http.Request) {
	releaseID, err := parseReleaseID(r.PathValue("id"))
	if err != nil || releaseID == 0 {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_release_id", "release id must be a positive integer")
		return
	}
	workItemID := strings.TrimSpace(r.PathValue("work_item_id"))
	if workItemID == "" {
		writeDeliveryError(w, http.StatusBadRequest, "work_item_required", "work item id is required")
		return
	}
	var release db.ReleaseVersion
	if err := db.DB.WithContext(r.Context()).First(&release, releaseID).Error; err != nil {
		writeDomainError(w, err)
		return
	}
	if ok, err := requestCanAccessReleaseProject(r, release.ProjectKey); err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "project_scope_failed", err.Error())
		return
	} else if !ok {
		writeDeliveryError(w, http.StatusNotFound, "release_not_found", "release was not found in the current project scope")
		return
	}

	var task db.TaskTelemetry
	if err := db.DB.WithContext(r.Context()).Where("task_id = ?", workItemID).First(&task).Error; err != nil {
		writeDomainError(w, err)
		return
	}
	var link db.WorkItemReleaseLink
	if err := db.DB.WithContext(r.Context()).
		Where(
			"work_item_id = ? AND release_version_id = ? AND relation = ? AND active = ? AND is_primary = ?",
			workItemID,
			release.ID,
			deliveryplanning.ReleaseTargetFix,
			true,
			true,
		).
		First(&link).Error; err != nil {
		writeDomainError(w, err)
		return
	}
	targetReleaseID := uint(0)
	if _, err := deliveryplanning.NewService(db.DB).ApplyPlanningChange(r.Context(), deliveryplanning.PlanningCommand{
		WorkItemID:             workItemID,
		ExpectedRevision:       task.Revision,
		PrimaryTargetReleaseID: &targetReleaseID,
		Reason:                 fmt.Sprintf("remove Jira issue from local release %s", release.Name),
		Actor:                  authenticatedActor(r),
		Source:                 "manual",
	}); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSearchJiraReleases(w http.ResponseWriter, r *http.Request) {
	if s.config == nil || !s.config.Jira.Enabled {
		writeDeliveryError(w, http.StatusServiceUnavailable, "jira_disabled", "Jira integration is disabled")
		return
	}
	if !s.config.Jira.VersionCatalogEnabled {
		writeDeliveryError(w, http.StatusConflict, "version_catalog_disabled", "Jira version catalog synchronization is disabled")
		return
	}
	if s.jiraReleaseList == nil {
		writeDeliveryError(w, http.StatusServiceUnavailable, "jira_unavailable", "Jira version catalog adapter is unavailable")
		return
	}

	projectKey := deliveryplanning.NormalizeProjectKey(r.URL.Query().Get("project_key"))
	projectKeys := []string{}
	if projectKey != "" {
		projectKeys = append(projectKeys, projectKey)
	} else {
		projectKeys = append(projectKeys, appconfig.JiraProjectKeys(&s.config.Jira)...)
	}
	if len(projectKeys) == 0 {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "jira_project_required", "choose a Jira project before searching versions")
		return
	}

	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	items := make([]deliveryplanning.ExternalRelease, 0)
	for _, key := range projectKeys {
		releases, err := s.jiraReleaseList(r.Context(), key)
		if err != nil {
			writeDeliveryError(w, http.StatusBadGateway, "jira_release_search_failed", err.Error())
			return
		}
		for _, release := range releases {
			haystack := strings.ToLower(strings.Join([]string{
				release.ProjectKey,
				release.ExternalID,
				release.Name,
				release.Description,
			}, " "))
			if search != "" && !strings.Contains(haystack, search) {
				continue
			}
			release.ProjectKey = deliveryplanning.NormalizeProjectKey(release.ProjectKey)
			if release.SourceURL == "" {
				release.SourceURL = s.jiraVersionPageURL(release.ProjectKey, release.ExternalID)
			}
			items = append(items, release)
			if len(items) >= 50 {
				break
			}
		}
		if len(items) >= 50 {
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"total": len(items),
	})
}

func (s *Server) handleListProjectReleases(w http.ResponseWriter, r *http.Request) {
	projectKey := deliveryplanning.NormalizeProjectKey(r.PathValue("project_key"))
	if projectKey == "" {
		writeDeliveryError(w, http.StatusBadRequest, "project_required", "project key is required")
		return
	}
	releases, err := deliveryplanning.NewRepository(db.DB).ListReleases(r.Context(), projectKey)
	if err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "release_query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"project_key": projectKey,
		"items":       releases,
		"total":       len(releases),
	})
}

func (s *Server) handleSyncProjectReleases(w http.ResponseWriter, r *http.Request) {
	projectKey := deliveryplanning.NormalizeProjectKey(r.PathValue("project_key"))
	if projectKey == "" {
		writeDeliveryError(w, http.StatusBadRequest, "project_required", "project key is required")
		return
	}
	if s.config == nil || !s.config.Jira.Enabled {
		writeDeliveryError(w, http.StatusServiceUnavailable, "jira_disabled", "Jira integration is disabled")
		return
	}
	if !s.config.Jira.VersionCatalogEnabled {
		writeDeliveryError(w, http.StatusConflict, "version_catalog_disabled", "Jira version catalog synchronization is disabled")
		return
	}
	if s.jiraReleaseList == nil {
		writeDeliveryError(w, http.StatusServiceUnavailable, "jira_unavailable", "Jira version catalog adapter is unavailable")
		return
	}
	external, err := s.jiraReleaseList(r.Context(), projectKey)
	if err != nil {
		writeDeliveryError(w, http.StatusBadGateway, "jira_release_sync_failed", err.Error())
		return
	}
	syncedAt := time.Now().UTC()
	releases, err := deliveryplanning.NewRepository(db.DB).UpsertExternalReleases(r.Context(), external, syncedAt)
	if err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "release_sync_failed", err.Error())
		return
	}
	for _, release := range releases {
		var candidate *deliveryplanning.ExternalRelease
		for index := range external {
			if strings.TrimSpace(external[index].ExternalID) == release.ExternalID {
				candidate = &external[index]
				break
			}
		}
		if candidate == nil {
			continue
		}
		if _, err := s.saveReleaseJiraLink(
			r,
			release,
			deliveryplanning.NormalizeProjectKey(candidate.ProjectKey),
			strings.TrimSpace(candidate.ExternalID),
			*candidate,
			"jira_sync",
		); err != nil {
			writeDeliveryError(w, http.StatusInternalServerError, "release_link_failed", err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"project_key": projectKey,
		"items":       releases,
		"synced_at":   syncedAt,
		"total":       len(releases),
	})
}

func (s *Server) handleCreateRelease(w http.ResponseWriter, r *http.Request) {
	s.createLocalRelease(w, r, "")
}

func (s *Server) handleCreateProjectRelease(w http.ResponseWriter, r *http.Request) {
	projectKey := deliveryplanning.NormalizeProjectKey(r.PathValue("project_key"))
	if projectKey == "" {
		writeDeliveryError(w, http.StatusBadRequest, "project_required", "project key is required")
		return
	}
	s.createLocalRelease(w, r, projectKey)
}

func (s *Server) createLocalRelease(w http.ResponseWriter, r *http.Request, scopedProjectKey string) {
	var request releaseMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	projectKey := scopedProjectKey
	if projectKey == "" && request.ProjectKey != nil {
		projectKey = deliveryplanning.NormalizeProjectKey(*request.ProjectKey)
	}
	if projectKey != "" && scopedProjectKey == "" {
		exists, err := s.releaseProjectExists(r, projectKey)
		if err != nil {
			writeDeliveryError(w, http.StatusInternalServerError, "project_query_failed", err.Error())
			return
		}
		if !exists {
			writeDeliveryError(w, http.StatusUnprocessableEntity, "unknown_project", "the selected project is not in the project catalog")
			return
		}
	}
	name := ""
	if request.Name != nil {
		name = strings.TrimSpace(*request.Name)
	}
	if name == "" {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "release_name_required", "release name is required")
		return
	}
	startDate, err := parseOptionalReleaseDate(request.StartDate)
	if err != nil {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_start_date", err.Error())
		return
	}
	releaseDate, err := parseOptionalReleaseDate(request.ReleaseDate)
	if err != nil {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_release_date", err.Error())
		return
	}
	status := deliveryplanning.ReleasePlanned
	if request.Status != nil {
		status = strings.ToLower(strings.TrimSpace(*request.Status))
	}
	if !validReleaseStatus(status) {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_release_status", "release status must be planned, released, or archived")
		return
	}
	description := ""
	if request.Description != nil {
		description = strings.TrimSpace(*request.Description)
	}
	release := db.ReleaseVersion{
		ProjectKey:  projectKey,
		Source:      "local",
		ExternalID:  fmt.Sprintf("local-%d", time.Now().UTC().UnixNano()),
		Name:        name,
		Description: description,
		Status:      status,
		StartDate:   startDate,
		ReleaseDate: releaseDate,
	}
	if err := db.DB.WithContext(r.Context()).Create(&release).Error; err != nil {
		writeDeliveryError(w, http.StatusConflict, "release_create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, release)
}

func (s *Server) handleGetRelease(w http.ResponseWriter, r *http.Request) {
	releaseID, err := parseReleaseID(r.PathValue("id"))
	if err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_release_id", err.Error())
		return
	}
	snapshot, err := deliveryplanning.NewRepository(db.DB).ReleaseSnapshot(r.Context(), releaseID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeDeliveryError(w, http.StatusNotFound, "release_not_found", "release was not found")
		return
	}
	if err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "release_query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handlePatchRelease(w http.ResponseWriter, r *http.Request) {
	releaseID, err := parseReleaseID(r.PathValue("id"))
	if err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_release_id", err.Error())
		return
	}
	var release db.ReleaseVersion
	if err := db.DB.WithContext(r.Context()).First(&release, releaseID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		writeDeliveryError(w, http.StatusNotFound, "release_not_found", "release was not found")
		return
	} else if err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "release_query_failed", err.Error())
		return
	}
	var request releaseMutationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	updates := map[string]any{}
	if request.ProjectKey != nil {
		projectKey := deliveryplanning.NormalizeProjectKey(*request.ProjectKey)
		if projectKey != "" {
			exists, err := s.releaseProjectExists(r, projectKey)
			if err != nil {
				writeDeliveryError(w, http.StatusInternalServerError, "project_query_failed", err.Error())
				return
			}
			if !exists {
				writeDeliveryError(w, http.StatusUnprocessableEntity, "unknown_project", "the selected project is not in the project catalog")
				return
			}
		}
		if projectKey != deliveryplanning.NormalizeProjectKey(release.ProjectKey) {
			var activeWorkItemCount int64
			if err := db.DB.WithContext(r.Context()).
				Model(&db.WorkItemReleaseLink{}).
				Where(
					"release_version_id = ? AND relation = ? AND active = ?",
					release.ID,
					deliveryplanning.ReleaseTargetFix,
					true,
				).
				Count(&activeWorkItemCount).Error; err != nil {
				writeDeliveryError(w, http.StatusInternalServerError, "release_work_item_query_failed", err.Error())
				return
			}
			if activeWorkItemCount > 0 {
				writeDeliveryError(w, http.StatusConflict, "release_has_work_items", "remove associated work items before changing the release project")
				return
			}
		}
		updates["project_key"] = projectKey
	}
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if name == "" {
			writeDeliveryError(w, http.StatusUnprocessableEntity, "release_name_required", "release name cannot be empty")
			return
		}
		updates["name"] = name
	}
	if request.Description != nil {
		updates["description"] = strings.TrimSpace(*request.Description)
	}
	if request.StartDate != nil {
		value, err := parseOptionalReleaseDate(request.StartDate)
		if err != nil {
			writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_start_date", err.Error())
			return
		}
		updates["start_date"] = value
	}
	if request.ReleaseDate != nil {
		value, err := parseOptionalReleaseDate(request.ReleaseDate)
		if err != nil {
			writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_release_date", err.Error())
			return
		}
		updates["release_date"] = value
	}
	if request.Status != nil {
		nextStatus := strings.ToLower(strings.TrimSpace(*request.Status))
		if !validReleaseStatus(nextStatus) {
			writeDeliveryError(w, http.StatusUnprocessableEntity, "invalid_release_status", "release status must be planned, released, or archived")
			return
		}
		if !validReleaseStatusTransition(release.Status, nextStatus) {
			writeDeliveryError(w, http.StatusConflict, "invalid_release_transition", "archived releases cannot be reopened and released releases cannot return to planned")
			return
		}
		updates["status"] = nextStatus
	}
	if len(updates) == 0 {
		writeJSON(w, http.StatusOK, release)
		return
	}
	if err := db.DB.WithContext(r.Context()).Model(&release).Updates(updates).Error; err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "release_update_failed", err.Error())
		return
	}
	if err := db.DB.WithContext(r.Context()).First(&release, releaseID).Error; err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "release_query_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, release)
}

func (s *Server) handlePutReleaseJiraLink(w http.ResponseWriter, r *http.Request) {
	releaseID, err := parseReleaseID(r.PathValue("id"))
	if err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_release_id", err.Error())
		return
	}
	var release db.ReleaseVersion
	if err := db.DB.WithContext(r.Context()).First(&release, releaseID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		writeDeliveryError(w, http.StatusNotFound, "release_not_found", "release was not found")
		return
	} else if err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "release_query_failed", err.Error())
		return
	}

	var request releaseJiraLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	projectKey := deliveryplanning.NormalizeProjectKey(request.JiraProjectKey)
	versionID := strings.TrimSpace(request.JiraVersionID)
	if projectKey == "" || versionID == "" {
		writeDeliveryError(w, http.StatusUnprocessableEntity, "jira_release_required", "Jira project and version are required")
		return
	}
	if release.ProjectKey != "" && !strings.EqualFold(release.ProjectKey, projectKey) {
		writeDeliveryError(w, http.StatusConflict, "release_jira_project_mismatch", "the Jira version must belong to the release project")
		return
	}
	if s.config == nil || !s.config.Jira.Enabled || !s.config.Jira.VersionCatalogEnabled || s.jiraReleaseList == nil {
		writeDeliveryError(w, http.StatusServiceUnavailable, "jira_version_catalog_unavailable", "Jira version catalog is unavailable")
		return
	}

	releases, err := s.jiraReleaseList(r.Context(), projectKey)
	if err != nil {
		writeDeliveryError(w, http.StatusBadGateway, "jira_release_lookup_failed", err.Error())
		return
	}
	var candidate *deliveryplanning.ExternalRelease
	for index := range releases {
		if strings.TrimSpace(releases[index].ExternalID) == versionID {
			candidate = &releases[index]
			break
		}
	}
	if candidate == nil {
		writeDeliveryError(w, http.StatusNotFound, "jira_release_not_found", "the selected Jira version no longer exists")
		return
	}

	link, err := s.saveReleaseJiraLink(r, release, projectKey, versionID, *candidate, authenticatedActor(r))
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeDeliveryError(w, http.StatusConflict, "jira_release_already_linked", "this Jira version is already associated with another release")
			return
		}
		writeDeliveryError(w, http.StatusInternalServerError, "release_link_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"release":   release,
		"jira_link": link,
	})
}

func (s *Server) handleDeleteReleaseJiraLink(w http.ResponseWriter, r *http.Request) {
	releaseID, err := parseReleaseID(r.PathValue("id"))
	if err != nil {
		writeDeliveryError(w, http.StatusBadRequest, "invalid_release_id", err.Error())
		return
	}
	var release db.ReleaseVersion
	if err := db.DB.WithContext(r.Context()).First(&release, releaseID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		writeDeliveryError(w, http.StatusNotFound, "release_not_found", "release was not found")
		return
	} else if err != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "release_query_failed", err.Error())
		return
	}
	result := db.DB.WithContext(r.Context()).Where("release_version_id = ?", releaseID).Delete(&db.ReleaseJiraLink{})
	if result.Error != nil {
		writeDeliveryError(w, http.StatusInternalServerError, "release_unlink_failed", result.Error.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"release":   release,
		"jira_link": nil,
		"unlinked":  result.RowsAffected > 0,
	})
}

func (s *Server) saveReleaseJiraLink(
	r *http.Request,
	release db.ReleaseVersion,
	projectKey string,
	versionID string,
	candidate deliveryplanning.ExternalRelease,
	actor string,
) (db.ReleaseJiraLink, error) {
	var occupied db.ReleaseJiraLink
	err := db.DB.WithContext(r.Context()).
		Where("jira_project_key = ? AND jira_version_id = ?", projectKey, versionID).
		First(&occupied).Error
	if err == nil && occupied.ReleaseVersionID != release.ID {
		return db.ReleaseJiraLink{}, gorm.ErrDuplicatedKey
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return db.ReleaseJiraLink{}, err
	}

	now := time.Now().UTC()
	link := db.ReleaseJiraLink{
		ReleaseVersionID: release.ID,
		JiraProjectKey:   projectKey,
		JiraVersionID:    versionID,
		JiraVersionName:  strings.TrimSpace(candidate.Name),
		JiraVersionURL:   s.jiraVersionPageURL(projectKey, versionID),
		JiraStatus:       strings.ToLower(strings.TrimSpace(candidate.Status)),
		LinkedBy:         strings.TrimSpace(actor),
		LinkedAt:         now,
	}
	if link.JiraVersionURL == "" {
		link.JiraVersionURL = strings.TrimSpace(candidate.SourceURL)
	}
	var existing db.ReleaseJiraLink
	err = db.DB.WithContext(r.Context()).Where("release_version_id = ?", release.ID).First(&existing).Error
	if err == nil {
		link.ID = existing.ID
		link.CreatedAt = existing.CreatedAt
		if saveErr := db.DB.WithContext(r.Context()).Save(&link).Error; saveErr != nil {
			return db.ReleaseJiraLink{}, saveErr
		}
		return link, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return db.ReleaseJiraLink{}, err
	}
	if err := db.DB.WithContext(r.Context()).Create(&link).Error; err != nil {
		return db.ReleaseJiraLink{}, err
	}
	return link, nil
}

func (s *Server) releaseProjectExists(r *http.Request, projectKey string) (bool, error) {
	var count int64
	if err := db.DB.WithContext(r.Context()).
		Model(&db.ProjectConfig{}).
		Where("project_key = ?", projectKey).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if s.config == nil {
		return false, nil
	}
	for _, key := range appconfig.JiraProjectKeys(&s.config.Jira) {
		if key == projectKey {
			return true, nil
		}
	}
	return false, nil
}

func (s *Server) jiraVersionPageURL(projectKey, versionID string) string {
	if s.config == nil {
		return ""
	}
	baseURL := strings.TrimRight(strings.TrimSpace(s.config.Jira.BaseURL), "/")
	if baseURL == "" {
		return ""
	}
	return fmt.Sprintf(
		"%s/projects/%s/versions/%s",
		baseURL,
		url.PathEscape(projectKey),
		url.PathEscape(versionID),
	)
}

func (s *Server) jiraIssuePageURL(issueKey string) string {
	if s.config == nil {
		return ""
	}
	baseURL := strings.TrimRight(strings.TrimSpace(s.config.Jira.BaseURL), "/")
	issueKey = strings.TrimSpace(issueKey)
	if baseURL == "" || issueKey == "" {
		return ""
	}
	return fmt.Sprintf("%s/browse/%s", baseURL, url.PathEscape(issueKey))
}

func requestCanAccessReleaseProject(r *http.Request, projectKey string) (bool, error) {
	keys, err := requestProjectPreferenceKeys(r)
	if err != nil {
		return false, err
	}
	if len(keys) == 0 {
		return true, nil
	}
	projectKey = deliveryplanning.NormalizeProjectKey(projectKey)
	for _, key := range keys {
		if deliveryplanning.NormalizeProjectKey(key) == projectKey {
			return true, nil
		}
	}
	return false, nil
}

func normalizeReleaseWorkItemIDs(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToUpper(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func parseOptionalReleaseDate(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*value))
	if err != nil {
		return nil, fmt.Errorf("date must use YYYY-MM-DD")
	}
	return &parsed, nil
}

func parseReleaseID(value string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("release id must be a positive integer")
	}
	return uint(parsed), nil
}

func validReleaseStatus(value string) bool {
	switch value {
	case deliveryplanning.ReleasePlanned, deliveryplanning.ReleaseReleased, deliveryplanning.ReleaseArchived:
		return true
	default:
		return false
	}
}

func validReleaseStatusTransition(current, next string) bool {
	current = strings.ToLower(strings.TrimSpace(current))
	next = strings.ToLower(strings.TrimSpace(next))
	if current == next {
		return true
	}
	switch current {
	case deliveryplanning.ReleasePlanned:
		return next == deliveryplanning.ReleaseReleased || next == deliveryplanning.ReleaseArchived
	case deliveryplanning.ReleaseReleased:
		return next == deliveryplanning.ReleaseArchived
	case deliveryplanning.ReleaseArchived:
		return false
	default:
		return next == deliveryplanning.ReleasePlanned
	}
}

func writeDeliveryError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error":   code,
		"message": message,
	})
}
