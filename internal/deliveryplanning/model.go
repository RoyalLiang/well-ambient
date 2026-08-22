package deliveryplanning

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"well-ambient/internal/db"
)

const (
	WorkItemRequirement = "requirement"
	WorkItemBug         = "bug"
	ExecutionTask       = "execution_task"

	ReleaseTargetFix = "target_fix"
	ReleaseAffected  = "affected"

	PlanningDraft        = "draft"
	PlanningReady        = "ready"
	PlanningPlanned      = "planned"
	PlanningCommitted    = "committed"
	PlanningInProgress   = "in_progress"
	PlanningVerification = "verification"
	PlanningDone         = "done"
	PlanningArchived     = "archived"

	ReleasePlanned   = "planned"
	ReleaseReleased  = "released"
	ReleaseArchived  = "archived"
	ReleaseDiscarded = "discarded"
)

type DomainError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *DomainError) Error() string {
	return e.Message
}

func ErrorCode(err error) string {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code
	}
	return ""
}

func NormalizeIssueType(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "requirement", "demand", "story", "需求", "user story", "product requirement":
		return WorkItemRequirement, nil
	case "bug", "defect", "缺陷", "故障":
		return WorkItemBug, nil
	case "execution_task", "execution-task", "task", "sub-task", "subtask", "执行任务":
		return ExecutionTask, nil
	default:
		return "", &DomainError{
			Code:       "unsupported_work_item_kind",
			Message:    fmt.Sprintf("unsupported issue type %q", value),
			StatusCode: 422,
		}
	}
}

func NormalizeProjectKey(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func ValidPlanningState(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PlanningDraft, PlanningReady, PlanningPlanned, PlanningCommitted,
		PlanningInProgress, PlanningVerification, PlanningDone, PlanningArchived:
		return true
	default:
		return false
	}
}

func ValidatePlanningGate(kind, projectKey, planningState string, primaryReleaseID uint) error {
	kind, err := NormalizeIssueType(kind)
	if err != nil {
		return err
	}
	if kind == ExecutionTask {
		return &DomainError{
			Code:       "execution_task_not_plannable",
			Message:    "execution tasks inherit project and release facts from their parent work item",
			StatusCode: 422,
		}
	}
	planningState = strings.ToLower(strings.TrimSpace(planningState))
	if planningState == "" {
		planningState = PlanningDraft
	}
	if !ValidPlanningState(planningState) {
		return &DomainError{
			Code:       "invalid_planning_state",
			Message:    fmt.Sprintf("invalid planning state %q", planningState),
			StatusCode: 422,
		}
	}
	if planningState != PlanningDraft && NormalizeProjectKey(projectKey) == "" {
		return &DomainError{
			Code:       "project_required",
			Message:    "a project is required before a work item can leave draft",
			StatusCode: 422,
		}
	}
	if planningState == PlanningCommitted && primaryReleaseID == 0 {
		return &DomainError{
			Code:       "primary_release_required",
			Message:    "a primary target release is required before commitment",
			StatusCode: 422,
		}
	}
	return nil
}

func ValidateReleaseForWorkItem(release db.ReleaseVersion, projectKey string, relation string) error {
	projectKey = NormalizeProjectKey(projectKey)
	if NormalizeProjectKey(release.ProjectKey) != projectKey {
		return &DomainError{
			Code:       "release_project_mismatch",
			Message:    "the selected release belongs to a different project",
			StatusCode: 409,
		}
	}
	if relation == ReleaseTargetFix && release.Status != ReleasePlanned {
		return &DomainError{
			Code:       "release_closed",
			Message:    "released or archived releases cannot accept a new target commitment",
			StatusCode: 409,
		}
	}
	return nil
}

type WorkItemSnapshot struct {
	WorkItem       db.TaskTelemetry         `json:"work_item"`
	TargetReleases []db.ReleaseVersion      `json:"target_releases"`
	Affected       []db.ReleaseVersion      `json:"affected_releases"`
	Links          []db.WorkItemReleaseLink `json:"release_links"`
	SyncState      string                   `json:"sync_state"`
}

type PlanQuery struct {
	ProjectKeys   []string
	Assignees     []string
	ReleaseID     uint
	Kinds         []string
	Statuses      []string
	PlanningState []string
	Search        string
	ActiveOnly    bool
	Limit         int
	Offset        int
}

type PlanSummary struct {
	Total        int64 `json:"total"`
	Active       int64 `json:"active"`
	Done         int64 `json:"done"`
	Backlog      int64 `json:"backlog"`
	Progress     int64 `json:"progress"`
	Review       int64 `json:"review"`
	Requirements int64 `json:"requirements"`
	Bugs         int64 `json:"bugs"`
	Planned      int64 `json:"planned"`
	Unplanned    int64 `json:"unplanned"`
}

type PlanSnapshot struct {
	Items   []WorkItemSnapshot `json:"items"`
	Total   int64              `json:"total"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
	Summary PlanSummary        `json:"summary"`
}

type PlanningCommand struct {
	WorkItemID             string
	ExpectedRevision       uint
	ProjectKey             *string
	PrimaryTargetReleaseID *uint
	AffectedReleaseIDs     *[]uint
	DueDate                **time.Time
	Assignee               *string
	PlanningState          *string
	Reason                 string
	Actor                  string
	Source                 string
}

type ReleaseQuery struct {
	ProjectKey string
	ReleaseID  uint
}

type ReleaseSnapshot struct {
	Release          db.ReleaseVersion `json:"release"`
	WorkItemCount    int64             `json:"work_item_count"`
	CompletedCount   int64             `json:"completed_count"`
	OpenCount        int64             `json:"open_count"`
	EstimatedDays    float64           `json:"estimated_days"`
	CompletedPercent float64           `json:"completed_percent"`
}

type ExternalRelease struct {
	ProjectKey  string     `json:"project_key"`
	ExternalID  string     `json:"external_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	StartDate   *time.Time `json:"start_date"`
	ReleaseDate *time.Time `json:"release_date"`
	SourceURL   string     `json:"source_url"`
}

type ExternalIssueVersionState struct {
	IssueKey         string
	ProjectKey       string
	Kind             string
	UpdatedAt        time.Time
	TargetReleases   []ExternalRelease
	AffectedReleases []ExternalRelease
}

type IssueVersionUpdate struct {
	IssueKey            string
	TargetExternalIDs   []string
	AffectedExternalIDs []string
}

type IssueTrackerPort interface {
	ListProjectReleases(ctx context.Context, projectKey string) ([]ExternalRelease, error)
	LoadIssueVersionState(ctx context.Context, issueKey string) (ExternalIssueVersionState, error)
	UpdateIssueVersionState(ctx context.Context, command IssueVersionUpdate) error
}
