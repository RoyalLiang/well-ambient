package solutions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"well-ambient/internal/db"
	"well-ambient/internal/solutioncatalog"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound  = errors.New("solution not found")
	ErrConflict  = errors.New("solution revision conflict")
	ErrImmutable = errors.New("published solution is immutable")
	ErrInvalid   = errors.New("invalid solution command")
	ErrTooLarge  = errors.New("solution markdown too large")
	ErrIntegrity = errors.New("solution content integrity failure")
)

const (
	KindHumanDraft     = "human_draft"
	KindHumanEdit      = "human_edit"
	KindSystemSeed     = "system_seed"
	KindAgentDraft     = "agent_draft"
	KindAgentCandidate = "agent_candidate"

	StatusSeed       = "seed"
	StatusDraft      = "draft"
	StatusCandidate  = "candidate"
	StatusApplied    = "applied"
	StatusPublished  = "published"
	StatusSuperseded = "superseded"

	JobQueued    = "queued"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
)

type Module struct {
	conn                 *gorm.DB
	now                  func() time.Time
	compressionThreshold int
	maxMarkdownBytes     int
}

type Option func(*Module)

func WithClock(now func() time.Time) Option {
	return func(module *Module) {
		if now != nil {
			module.now = now
		}
	}
}

func WithCompressionThreshold(bytes int) Option {
	return func(module *Module) {
		if bytes >= 0 {
			module.compressionThreshold = bytes
		}
	}
}

func New(conn *gorm.DB, options ...Option) *Module {
	module := &Module{
		conn: conn, now: time.Now,
		compressionThreshold: defaultCompressionThreshold,
		maxMarkdownBytes:     defaultMaxMarkdownBytes,
	}
	for _, option := range options {
		option(module)
	}
	return module
}

type Revision struct {
	ID                      uint      `json:"id"`
	Version                 int       `json:"version"`
	ParentRevisionID        uint      `json:"parent_revision_id"`
	DerivedFromRevisionID   uint      `json:"derived_from_revision_id"`
	Kind                    string    `json:"kind"`
	Status                  string    `json:"status"`
	Title                   string    `json:"title"`
	Summary                 string    `json:"summary"`
	Markdown                string    `json:"markdown,omitempty"`
	ContentHash             string    `json:"content_hash"`
	ContentBytes            int       `json:"content_bytes"`
	StoredBytes             int       `json:"stored_bytes"`
	ContentEncoding         string    `json:"content_encoding"`
	PromptTemplateVersionID uint      `json:"prompt_template_version_id"`
	ModelVersion            string    `json:"model_version"`
	SourceWatermark         string    `json:"source_watermark"`
	AuthoredBy              string    `json:"authored_by"`
	CreatedAt               time.Time `json:"created_at"`
}

type SourceRef struct {
	ID              uint       `json:"id"`
	SourceSystem    string     `json:"source_system"`
	ExternalID      string     `json:"external_id"`
	ContentHash     string     `json:"content_hash"`
	Author          string     `json:"author"`
	Marker          string     `json:"marker"`
	Eligible        bool       `json:"eligible"`
	Current         bool       `json:"current"`
	SourceCreatedAt *time.Time `json:"source_created_at"`
	SourceUpdatedAt *time.Time `json:"source_updated_at"`
	ObservedAt      time.Time  `json:"observed_at"`
}

type Workspace struct {
	Asset      db.SolutionAsset       `json:"asset"`
	Working    *Revision              `json:"working,omitempty"`
	Published  *Revision              `json:"published,omitempty"`
	Candidates []Revision             `json:"candidates"`
	History    []Revision             `json:"history"`
	Sources    []SourceRef            `json:"sources"`
	Jobs       []db.SolutionPolishJob `json:"jobs"`
}

type EnsureDraftCommand struct {
	DemandID string
	Title    string
	Markdown string
	Actor    string
}

func (module *Module) EnsureDraft(ctx context.Context, command EnsureDraftCommand) (Workspace, error) {
	if module == nil || module.conn == nil {
		return Workspace{}, ErrNotFound
	}
	command.DemandID = strings.TrimSpace(command.DemandID)
	command.Actor = fallback(command.Actor, "system")
	if command.DemandID == "" {
		return Workspace{}, fmt.Errorf("%w: demand_id is required", ErrInvalid)
	}
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, err := module.ensureAsset(tx, command.DemandID, command.Actor)
		if err != nil {
			return err
		}
		if asset.WorkingRevisionID != 0 {
			return nil
		}
		encoded, err := encodeMarkdown(command.Markdown, module.compressionThreshold, module.maxMarkdownBytes)
		if err != nil {
			return err
		}
		revision := module.revisionModel(asset, encoded, revisionFields{
			Kind: KindHumanDraft, Status: StatusDraft, Title: command.Title, AuthoredBy: command.Actor,
		})
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		return module.advanceWorking(tx, asset, revision.ID, asset.Revision)
	})
	if err != nil {
		return Workspace{}, err
	}
	return module.GetWorkspace(ctx, command.DemandID)
}

type SaveDraftCommand struct {
	DemandID         string
	ExpectedRevision uint
	BaseRevisionID   uint
	Title            string
	Markdown         string
	Actor            string
}

func (module *Module) SaveDraft(ctx context.Context, command SaveDraftCommand) (Workspace, error) {
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, current, err := module.loadWorking(tx, command.DemandID)
		if err != nil {
			return err
		}
		if asset.Revision != command.ExpectedRevision || (command.BaseRevisionID != 0 && current.ID != command.BaseRevisionID) {
			return ErrConflict
		}
		if current.Status == StatusPublished {
			return ErrImmutable
		}
		encoded, err := encodeMarkdown(command.Markdown, module.compressionThreshold, module.maxMarkdownBytes)
		if err != nil {
			return err
		}
		if encoded.hash == current.ContentHash {
			return nil
		}
		revision := module.revisionModel(asset, encoded, revisionFields{
			ParentRevisionID: current.ID, Kind: KindHumanEdit, Status: StatusDraft,
			Title: command.Title, AuthoredBy: fallback(command.Actor, "unknown"),
		})
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		return module.advanceWorking(tx, asset, revision.ID, command.ExpectedRevision)
	})
	if err != nil {
		return Workspace{}, err
	}
	return module.GetWorkspace(ctx, command.DemandID)
}

type ForkDraftCommand struct {
	DemandID         string
	ExpectedRevision uint
	Actor            string
}

func (module *Module) ForkDraft(ctx context.Context, command ForkDraftCommand) (Workspace, error) {
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, current, err := module.loadWorking(tx, command.DemandID)
		if err != nil {
			return err
		}
		if asset.Revision != command.ExpectedRevision {
			return ErrConflict
		}
		if current.Status != StatusPublished {
			return nil
		}
		markdown, err := decodeMarkdown(current.Content, current.ContentEncoding, current.ContentHash, module.maxMarkdownBytes)
		if err != nil {
			return err
		}
		encoded, err := encodeMarkdown(markdown, module.compressionThreshold, module.maxMarkdownBytes)
		if err != nil {
			return err
		}
		revision := module.revisionModel(asset, encoded, revisionFields{
			ParentRevisionID: current.ID, Kind: KindHumanDraft, Status: StatusDraft,
			Title: current.Title, Summary: current.Summary, AuthoredBy: fallback(command.Actor, "unknown"),
		})
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		return module.advanceWorking(tx, asset, revision.ID, command.ExpectedRevision)
	})
	if err != nil {
		return Workspace{}, err
	}
	return module.GetWorkspace(ctx, command.DemandID)
}

type ObserveSourceCommand struct {
	DemandID        string
	SourceSystem    string
	ExternalID      string
	Author          string
	Marker          string
	Eligible        bool
	Body            string
	SourceCreatedAt *time.Time
	SourceUpdatedAt *time.Time
	Actor           string
}

type ObserveSourceResult struct {
	Source   db.SolutionSourceRef
	Replayed bool
}

func (module *Module) ObserveSource(ctx context.Context, command ObserveSourceCommand) (ObserveSourceResult, error) {
	command.DemandID = strings.TrimSpace(command.DemandID)
	command.SourceSystem = strings.ToLower(strings.TrimSpace(command.SourceSystem))
	command.ExternalID = strings.TrimSpace(command.ExternalID)
	if command.DemandID == "" || command.SourceSystem == "" || command.ExternalID == "" {
		return ObserveSourceResult{}, fmt.Errorf("%w: demand_id, source_system and external_id are required", ErrInvalid)
	}
	encoded, err := encodeMarkdown(command.Body, module.compressionThreshold, module.maxMarkdownBytes)
	if err != nil {
		return ObserveSourceResult{}, err
	}
	var result ObserveSourceResult
	err = module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, err := module.ensureAsset(tx, command.DemandID, fallback(command.Actor, "source_sync"))
		if err != nil {
			return err
		}
		source := db.SolutionSourceRef{
			SolutionAssetID: asset.ID, SourceSystem: command.SourceSystem, ExternalID: command.ExternalID,
			ContentHash: encoded.hash, Author: strings.TrimSpace(command.Author), Marker: strings.TrimSpace(command.Marker),
			Eligible: command.Eligible, Current: true, Content: encoded.stored, ContentEncoding: encoded.encoding,
			ContentBytes: encoded.rawBytes, StoredBytes: len(encoded.stored),
			SourceCreatedAt: command.SourceCreatedAt, SourceUpdatedAt: command.SourceUpdatedAt, ObservedAt: module.now(),
		}
		create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&source)
		if create.Error != nil {
			return create.Error
		}
		if create.RowsAffected == 0 {
			if err := tx.Where("solution_asset_id = ? AND source_system = ? AND external_id = ? AND content_hash = ?", asset.ID, source.SourceSystem, source.ExternalID, source.ContentHash).First(&result.Source).Error; err != nil {
				return err
			}
			classificationUpdates := map[string]any{}
			if result.Source.Marker != source.Marker {
				classificationUpdates["marker"] = source.Marker
			}
			if result.Source.Eligible != source.Eligible {
				classificationUpdates["eligible"] = source.Eligible
			}
			if len(classificationUpdates) > 0 {
				if err := tx.Model(&db.SolutionSourceRef{}).
					Where("id = ?", result.Source.ID).
					Updates(classificationUpdates).Error; err != nil {
					return err
				}
				result.Source.Marker = source.Marker
				result.Source.Eligible = source.Eligible
			}
			if result.Source.Current {
				result.Replayed = true
				return nil
			}
			if err := tx.Model(&db.SolutionSourceRef{}).
				Where("solution_asset_id = ? AND source_system = ? AND external_id = ? AND current = ?", asset.ID, source.SourceSystem, source.ExternalID, true).
				Update("current", false).Error; err != nil {
				return err
			}
			if err := tx.Model(&result.Source).Update("current", true).Error; err != nil {
				return err
			}
			result.Source.Current = true
			return nil
		}
		if err := tx.Model(&db.SolutionSourceRef{}).
			Where("solution_asset_id = ? AND source_system = ? AND external_id = ? AND id != ? AND current = ?", asset.ID, source.SourceSystem, source.ExternalID, source.ID, true).
			Update("current", false).Error; err != nil {
			return err
		}
		result.Source = source
		return nil
	})
	return result, err
}

// ReconcileSourceSet marks externally deleted records non-current while
// preserving every observed snapshot and historical polish reference.
func (module *Module) ReconcileSourceSet(ctx context.Context, demandID, sourceSystem string, externalIDs []string) error {
	var asset db.SolutionAsset
	lookup := module.conn.WithContext(ctx).
		Where("demand_id = ?", strings.TrimSpace(demandID)).
		Limit(1).
		Find(&asset)
	if lookup.Error != nil {
		return lookup.Error
	}
	if lookup.RowsAffected == 0 {
		return nil
	}
	query := module.conn.WithContext(ctx).Model(&db.SolutionSourceRef{}).
		Where("solution_asset_id = ? AND source_system = ? AND current = ?", asset.ID, strings.ToLower(strings.TrimSpace(sourceSystem)), true)
	if len(externalIDs) > 0 {
		query = query.Where("external_id NOT IN ?", externalIDs)
	}
	return query.Update("current", false).Error
}

type RequestPolishCommand struct {
	DemandID       string
	ProjectKey     string
	RequestedBy    string
	IdempotencyKey string
}

type RequestInitialDraftCommand struct {
	DemandID       string
	ProjectKey     string
	Title          string
	Markdown       string
	RequestedBy    string
	IdempotencyKey string
}

// RequestInitialDraft queues the only automatic generation path. The input is
// kept as a hidden v0 seed so no placeholder is exposed as a user draft.
func (module *Module) RequestInitialDraft(ctx context.Context, command RequestInitialDraftCommand) (db.SolutionPolishJob, bool, error) {
	var result db.SolutionPolishJob
	var replayed bool
	command.DemandID = strings.TrimSpace(command.DemandID)
	if command.DemandID == "" {
		return result, false, fmt.Errorf("%w: demand_id is required", ErrInvalid)
	}
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, err := module.ensureAsset(tx, command.DemandID, fallback(command.RequestedBy, "system"))
		if err != nil {
			return err
		}
		if asset.WorkingRevisionID != 0 {
			return ErrConflict
		}
		var seed db.SolutionRevision
		lookup := tx.Where("solution_asset_id = ? AND kind = ?", asset.ID, KindSystemSeed).
			Order("id DESC").Limit(1).Find(&seed)
		if lookup.Error != nil {
			return lookup.Error
		}
		if lookup.RowsAffected == 0 {
			encoded, err := encodeMarkdown(command.Markdown, module.compressionThreshold, module.maxMarkdownBytes)
			if err != nil {
				return err
			}
			seed = module.revisionModel(asset, encoded, revisionFields{
				Kind: KindSystemSeed, Status: StatusSeed, Title: command.Title,
				AuthoredBy: fallback(command.RequestedBy, "system"),
			})
			seed.Version = 0
			if err := tx.Create(&seed).Error; err != nil {
				return err
			}
		}
		prompt, err := module.activePrompt(tx, command.ProjectKey)
		if err != nil {
			return err
		}
		result, replayed, err = module.createPolishJob(tx, asset, seed, prompt, command.RequestedBy, command.IdempotencyKey)
		return err
	})
	return result, replayed, err
}

func (module *Module) RequestPolish(ctx context.Context, command RequestPolishCommand) (db.SolutionPolishJob, bool, error) {
	var result db.SolutionPolishJob
	var replayed bool
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, current, err := module.loadWorking(tx, command.DemandID)
		if err != nil {
			return err
		}
		prompt, err := module.activePrompt(tx, command.ProjectKey)
		if err != nil {
			return err
		}
		result, replayed, err = module.createPolishJob(tx, asset, current, prompt, command.RequestedBy, command.IdempotencyKey)
		if err != nil {
			return err
		}
		return nil
	})
	return result, replayed, err
}

func (module *Module) createPolishJob(tx *gorm.DB, asset db.SolutionAsset, input db.SolutionRevision, prompt db.SolutionPromptTemplate, requestedBy, idempotencyKey string) (db.SolutionPolishJob, bool, error) {
	sources, watermark, err := module.latestEligibleSources(tx, asset.ID)
	if err != nil {
		return db.SolutionPolishJob{}, false, err
	}
	sourceIDs := make([]uint, 0, len(sources))
	for _, source := range sources {
		sourceIDs = append(sourceIDs, source.ID)
	}
	sourceJSON, _ := json.Marshal(sourceIDs)
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		key = fmt.Sprintf("solution:%d:input:%d:prompt:%d:sources:%s", asset.ID, input.ID, prompt.ID, watermark)
	}
	job := db.SolutionPolishJob{
		IdempotencyKey: key, SolutionAssetID: asset.ID, InputRevisionID: input.ID,
		PromptTemplateVersionID: prompt.ID, SourceRefsJSON: string(sourceJSON), SourceWatermark: watermark,
		Status: JobQueued, RequestedBy: fallback(requestedBy, "unknown"),
		CreatedAt: module.now(), UpdatedAt: module.now(),
	}
	create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&job)
	if create.Error != nil {
		return db.SolutionPolishJob{}, false, create.Error
	}
	if create.RowsAffected == 0 {
		var existing db.SolutionPolishJob
		err := tx.Where("idempotency_key = ?", key).First(&existing).Error
		return existing, true, err
	}
	return job, false, nil
}

type RetryFailedPolishCommand struct {
	DemandID    string
	JobID       uint
	RequestedBy string
}

// RetryFailedPolish starts a fresh, auditable attempt cycle for the latest
// terminal failure. The failed job remains immutable evidence; repeated
// requests for the same failure converge on one replacement job.
func (module *Module) RetryFailedPolish(ctx context.Context, command RetryFailedPolishCommand) (db.SolutionPolishJob, bool, error) {
	var result db.SolutionPolishJob
	var replayed bool
	command.DemandID = strings.TrimSpace(command.DemandID)
	if command.DemandID == "" || command.JobID == 0 {
		return result, false, fmt.Errorf("%w: demand_id and job_id are required", ErrInvalid)
	}
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset db.SolutionAsset
		assetLookup := tx.Where("demand_id = ?", command.DemandID).Limit(1).Find(&asset)
		if assetLookup.Error != nil {
			return assetLookup.Error
		}
		if assetLookup.RowsAffected == 0 {
			return ErrNotFound
		}
		if asset.WorkingRevisionID != 0 {
			return fmt.Errorf("%w: a working solution already exists", ErrConflict)
		}

		var failed db.SolutionPolishJob
		failedLookup := tx.Where("id = ? AND solution_asset_id = ? AND status = ?", command.JobID, asset.ID, JobFailed).
			Limit(1).Find(&failed)
		if failedLookup.Error != nil {
			return failedLookup.Error
		}
		if failedLookup.RowsAffected == 0 {
			return fmt.Errorf("%w: polish job is not a terminal failure for this demand", ErrConflict)
		}

		key := fmt.Sprintf("solution:manual-retry:%d", failed.ID)
		var existing db.SolutionPolishJob
		existingLookup := tx.Where("idempotency_key = ?", key).Limit(1).Find(&existing)
		if existingLookup.Error != nil {
			return existingLookup.Error
		}
		if existingLookup.RowsAffected > 0 {
			result = existing
			replayed = true
			return nil
		}

		var latest db.SolutionPolishJob
		latestLookup := tx.Where("solution_asset_id = ?", asset.ID).Order("id DESC").Limit(1).Find(&latest)
		if latestLookup.Error != nil {
			return latestLookup.Error
		}
		if latestLookup.RowsAffected == 0 || latest.ID != failed.ID {
			return fmt.Errorf("%w: a newer generation task already exists", ErrConflict)
		}

		now := module.now()
		result = db.SolutionPolishJob{
			IdempotencyKey:          key,
			SolutionAssetID:         failed.SolutionAssetID,
			InputRevisionID:         failed.InputRevisionID,
			PromptTemplateVersionID: failed.PromptTemplateVersionID,
			SourceRefsJSON:          failed.SourceRefsJSON,
			SourceWatermark:         failed.SourceWatermark,
			Status:                  JobQueued,
			RequestedBy:             fallback(command.RequestedBy, "unknown"),
			CreatedAt:               now,
			UpdatedAt:               now,
		}
		create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&result)
		if create.Error != nil {
			return create.Error
		}
		if create.RowsAffected > 0 {
			return nil
		}
		if err := tx.Where("idempotency_key = ?", key).First(&result).Error; err != nil {
			return err
		}
		replayed = true
		return nil
	})
	return result, replayed, err
}

type ClaimedJob struct {
	Job      db.SolutionPolishJob
	Prompt   db.SolutionPromptTemplate
	Input    Revision
	Sources  []RevisionSource
	DemandID string
}

type RevisionSource struct {
	ID       uint
	Author   string
	Markdown string
}

func (module *Module) ClaimNextJob(ctx context.Context) (*ClaimedJob, error) {
	var claimed *ClaimedJob
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := module.now()
		staleBefore := now.Add(-10 * time.Minute)
		if err := tx.Model(&db.SolutionPolishJob{}).
			Where("status = ? AND updated_at < ?", JobRunning, staleBefore).
			Updates(map[string]any{"status": JobQueued, "next_attempt_at": now, "updated_at": now, "last_error": "worker lease expired; queued for retry"}).Error; err != nil {
			return err
		}
		var job db.SolutionPolishJob
		query := tx.Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", JobQueued, now).
			Order("id ASC").Limit(1).Find(&job)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected == 0 {
			return nil
		}
		update := tx.Model(&db.SolutionPolishJob{}).Where("id = ? AND status = ?", job.ID, JobQueued).
			Updates(map[string]any{"status": JobRunning, "attempt_count": job.AttemptCount + 1, "started_at": now, "updated_at": now, "next_attempt_at": nil})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			return nil
		}
		job.Status = JobRunning
		job.AttemptCount++
		job.StartedAt = &now
		var asset db.SolutionAsset
		if err := tx.First(&asset, job.SolutionAssetID).Error; err != nil {
			return err
		}
		var prompt db.SolutionPromptTemplate
		if err := tx.First(&prompt, job.PromptTemplateVersionID).Error; err != nil {
			return err
		}
		var inputModel db.SolutionRevision
		if err := tx.First(&inputModel, job.InputRevisionID).Error; err != nil {
			return err
		}
		input, err := module.revisionDTO(inputModel, true)
		if err != nil {
			return err
		}
		var sourceIDs []uint
		if err := json.Unmarshal([]byte(job.SourceRefsJSON), &sourceIDs); err != nil {
			return err
		}
		var sourceModels []db.SolutionSourceRef
		if len(sourceIDs) > 0 {
			if err := tx.Where("id IN ?", sourceIDs).Order("id ASC").Find(&sourceModels).Error; err != nil {
				return err
			}
		}
		sources := make([]RevisionSource, 0, len(sourceModels))
		for _, source := range sourceModels {
			body, err := decodeMarkdown(source.Content, source.ContentEncoding, source.ContentHash, module.maxMarkdownBytes)
			if err != nil {
				return err
			}
			sources = append(sources, RevisionSource{ID: source.ID, Author: source.Author, Markdown: body})
		}
		claimed = &ClaimedJob{Job: job, Prompt: prompt, Input: input, Sources: sources, DemandID: asset.DemandID}
		return nil
	})
	return claimed, err
}

type CompletePolishCommand struct {
	JobID        uint
	Markdown     string
	ModelVersion string
}

func (module *Module) CompletePolish(ctx context.Context, command CompletePolishCommand) (Revision, error) {
	var result Revision
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job db.SolutionPolishJob
		if err := tx.First(&job, command.JobID).Error; err != nil {
			return err
		}
		if job.Status == JobSucceeded && job.OutputRevisionID != 0 {
			var existing db.SolutionRevision
			if err := tx.First(&existing, job.OutputRevisionID).Error; err != nil {
				return err
			}
			var err error
			result, err = module.revisionDTO(existing, true)
			return err
		}
		if job.Status != JobRunning {
			return fmt.Errorf("%w: polish job is %s", ErrConflict, job.Status)
		}
		var asset db.SolutionAsset
		if err := tx.First(&asset, job.SolutionAssetID).Error; err != nil {
			return err
		}
		var input db.SolutionRevision
		if err := tx.First(&input, job.InputRevisionID).Error; err != nil {
			return err
		}
		if input.Kind == KindSystemSeed && asset.WorkingRevisionID != 0 {
			var working db.SolutionRevision
			if err := tx.First(&working, asset.WorkingRevisionID).Error; err != nil {
				return err
			}
			now := module.now()
			complete := tx.Model(&db.SolutionPolishJob{}).Where("id = ? AND status = ?", job.ID, JobRunning).
				Updates(map[string]any{"status": JobSucceeded, "output_revision_id": working.ID, "completed_at": now, "updated_at": now, "last_error": ""})
			if complete.Error != nil {
				return complete.Error
			}
			if complete.RowsAffected == 0 {
				return ErrConflict
			}
			var err error
			result, err = module.revisionDTO(working, true)
			return err
		}
		encoded, err := encodeMarkdown(command.Markdown, module.compressionThreshold, module.maxMarkdownBytes)
		if err != nil {
			return err
		}
		fields := revisionFields{
			DerivedFromRevisionID: job.InputRevisionID, Kind: KindAgentCandidate, Status: StatusCandidate,
			PromptTemplateVersionID: job.PromptTemplateVersionID, ModelVersion: command.ModelVersion,
			SourceWatermark: job.SourceWatermark, AuthoredBy: "agent",
		}
		if input.Kind == KindSystemSeed {
			fields.Kind = KindAgentDraft
			fields.Status = StatusDraft
		}
		revision := module.revisionModel(asset, encoded, fields)
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		if input.Kind == KindSystemSeed {
			if err := module.advanceWorking(tx, asset, revision.ID, asset.Revision); err != nil {
				return err
			}
		} else {
			advance := tx.Model(&db.SolutionAsset{}).Where("id = ? AND sequence = ?", asset.ID, asset.Sequence).
				Update("sequence", revision.Version)
			if advance.Error != nil {
				return advance.Error
			}
			if advance.RowsAffected == 0 {
				return ErrConflict
			}
		}
		now := module.now()
		complete := tx.Model(&db.SolutionPolishJob{}).Where("id = ? AND status = ?", job.ID, JobRunning).
			Updates(map[string]any{"status": JobSucceeded, "output_revision_id": revision.ID, "completed_at": now, "updated_at": now, "last_error": ""})
		if complete.Error != nil {
			return complete.Error
		}
		if complete.RowsAffected == 0 {
			return ErrConflict
		}
		result, err = module.revisionDTO(revision, true)
		return err
	})
	return result, err
}

func (module *Module) FailPolish(ctx context.Context, jobID uint, cause error) error {
	return module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job db.SolutionPolishJob
		if err := tx.First(&job, jobID).Error; err != nil {
			return err
		}
		if job.Status != JobRunning {
			return nil
		}
		now := module.now()
		status := JobFailed
		var next *time.Time
		if job.AttemptCount < 3 {
			status = JobQueued
			retry := now.Add(time.Duration(job.AttemptCount*job.AttemptCount) * time.Minute)
			next = &retry
		}
		message := "generation failed"
		if cause != nil {
			message = cause.Error()
		}
		return tx.Model(&job).Updates(map[string]any{
			"status": status, "last_error": message, "next_attempt_at": next,
			"completed_at": nil, "updated_at": now,
		}).Error
	})
}

type ApplyCandidateCommand struct {
	DemandID         string
	CandidateID      uint
	ExpectedRevision uint
	Actor            string
}

func (module *Module) ApplyCandidate(ctx context.Context, command ApplyCandidateCommand) (Workspace, error) {
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, current, err := module.loadWorking(tx, command.DemandID)
		if err != nil {
			return err
		}
		if asset.Revision != command.ExpectedRevision {
			return ErrConflict
		}
		var candidate db.SolutionRevision
		if err := tx.Where("id = ? AND solution_asset_id = ? AND status = ?", command.CandidateID, asset.ID, StatusCandidate).First(&candidate).Error; err != nil {
			return ErrNotFound
		}
		markdown, err := decodeMarkdown(candidate.Content, candidate.ContentEncoding, candidate.ContentHash, module.maxMarkdownBytes)
		if err != nil {
			return err
		}
		encoded, err := encodeMarkdown(markdown, module.compressionThreshold, module.maxMarkdownBytes)
		if err != nil {
			return err
		}
		revision := module.revisionModel(asset, encoded, revisionFields{
			ParentRevisionID: current.ID, DerivedFromRevisionID: candidate.ID, Kind: KindHumanEdit,
			Status: StatusDraft, Title: candidate.Title, Summary: candidate.Summary,
			PromptTemplateVersionID: candidate.PromptTemplateVersionID, ModelVersion: candidate.ModelVersion,
			SourceWatermark: candidate.SourceWatermark, AuthoredBy: fallback(command.Actor, "unknown"),
		})
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.SolutionRevision{}).
			Where("id = ? AND status = ?", candidate.ID, StatusCandidate).
			Update("status", StatusApplied).Error; err != nil {
			return err
		}
		return module.advanceWorking(tx, asset, revision.ID, command.ExpectedRevision)
	})
	if err != nil {
		return Workspace{}, err
	}
	return module.GetWorkspace(ctx, command.DemandID)
}

type PublishCommand struct {
	DemandID         string
	ExpectedRevision uint
	Actor            string
}

func (module *Module) Publish(ctx context.Context, command PublishCommand) (Workspace, error) {
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, current, err := module.loadWorking(tx, command.DemandID)
		if err != nil {
			return err
		}
		if asset.Revision != command.ExpectedRevision {
			return ErrConflict
		}
		if current.Status == StatusPublished && asset.PublishedRevisionID == current.ID {
			return nil
		}
		if current.Status != StatusDraft {
			return fmt.Errorf("%w: only a draft can be published", ErrInvalid)
		}
		now := module.now()
		if asset.PublishedRevisionID != 0 && asset.PublishedRevisionID != current.ID {
			if err := tx.Model(&db.SolutionRevision{}).Where("id = ?", asset.PublishedRevisionID).Update("status", StatusSuperseded).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&db.SolutionRevision{}).Where("id = ? AND status = ?", current.ID, StatusDraft).Update("status", StatusPublished).Error; err != nil {
			return err
		}
		advance := tx.Model(&db.SolutionAsset{}).Where("id = ? AND revision = ?", asset.ID, command.ExpectedRevision).
			Updates(map[string]any{"revision": command.ExpectedRevision + 1, "published_revision_id": current.ID, "updated_at": now})
		if advance.Error != nil {
			return advance.Error
		}
		if advance.RowsAffected == 0 {
			return ErrConflict
		}
		return solutioncatalog.EnqueuePublished(tx, asset, current, now)
	})
	if err != nil {
		return Workspace{}, err
	}
	return module.GetWorkspace(ctx, command.DemandID)
}

// MigrateLegacyInitialDrafts repairs the former Jira lifecycle without deleting
// revisions. A jira-sync placeholder becomes a hidden seed; an existing Agent
// candidate becomes the canonical editable draft.
func (module *Module) MigrateLegacyInitialDrafts(ctx context.Context) error {
	if module == nil || module.conn == nil {
		return ErrNotFound
	}
	var assets []db.SolutionAsset
	if err := module.conn.WithContext(ctx).
		Joins("JOIN solution_revisions working ON working.id = solution_assets.working_revision_id").
		Where("working.kind = ? AND working.authored_by = ?", KindHumanDraft, "jira-sync").
		Find(&assets).Error; err != nil {
		return err
	}
	for _, candidateAsset := range assets {
		if err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var asset db.SolutionAsset
			if err := tx.First(&asset, candidateAsset.ID).Error; err != nil {
				return err
			}
			var seed db.SolutionRevision
			if err := tx.First(&seed, asset.WorkingRevisionID).Error; err != nil {
				return err
			}
			if seed.Kind != KindHumanDraft || seed.AuthoredBy != "jira-sync" {
				return nil
			}
			var humanCount int64
			if err := tx.Model(&db.SolutionRevision{}).
				Where("solution_asset_id = ? AND authored_by NOT IN ?", asset.ID, []string{"jira-sync", "agent"}).
				Count(&humanCount).Error; err != nil {
				return err
			}
			if humanCount > 0 {
				return nil
			}
			var agentDraft db.SolutionRevision
			lookup := tx.Where("solution_asset_id = ? AND kind = ? AND status = ?", asset.ID, KindAgentCandidate, StatusCandidate).
				Order("version DESC, id DESC").Limit(1).Find(&agentDraft)
			if lookup.Error != nil {
				return lookup.Error
			}
			if lookup.RowsAffected == 0 {
				var revisionCount int64
				if err := tx.Model(&db.SolutionRevision{}).Where("solution_asset_id = ?", asset.ID).Count(&revisionCount).Error; err != nil {
					return err
				}
				if revisionCount != 1 {
					return nil
				}
			}
			if err := tx.Model(&db.SolutionRevision{}).Where("id = ?", seed.ID).
				Updates(map[string]any{"kind": KindSystemSeed, "status": StatusSeed}).Error; err != nil {
				return err
			}
			if lookup.RowsAffected > 0 {
				if err := tx.Model(&db.SolutionRevision{}).Where("id = ?", agentDraft.ID).
					Updates(map[string]any{"kind": KindAgentDraft, "status": StatusDraft}).Error; err != nil {
					return err
				}
				return tx.Model(&db.SolutionAsset{}).Where("id = ? AND working_revision_id = ?", asset.ID, seed.ID).
					Updates(map[string]any{
						"revision": gorm.Expr("revision + 1"), "working_revision_id": agentDraft.ID, "updated_at": module.now(),
					}).Error
			}
			if err := tx.Model(&db.SolutionRevision{}).Where("id = ?", seed.ID).Update("version", 0).Error; err != nil {
				return err
			}
			return tx.Model(&db.SolutionAsset{}).Where("id = ? AND working_revision_id = ?", asset.ID, seed.ID).
				Updates(map[string]any{
					"revision": gorm.Expr("revision + 1"), "sequence": 0, "working_revision_id": 0, "updated_at": module.now(),
				}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

func (module *Module) GetWorkspace(ctx context.Context, demandID string) (Workspace, error) {
	var workspace Workspace
	if err := module.conn.WithContext(ctx).Where("demand_id = ?", strings.TrimSpace(demandID)).First(&workspace.Asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Workspace{}, ErrNotFound
		}
		return Workspace{}, err
	}
	var revisions []db.SolutionRevision
	if err := module.conn.WithContext(ctx).Where("solution_asset_id = ?", workspace.Asset.ID).Order("version DESC").Find(&revisions).Error; err != nil {
		return Workspace{}, err
	}
	for _, model := range revisions {
		if model.Kind == KindSystemSeed {
			continue
		}
		includeContent := model.ID == workspace.Asset.WorkingRevisionID || model.ID == workspace.Asset.PublishedRevisionID || model.Status == StatusCandidate
		revision, err := module.revisionDTO(model, includeContent)
		if err != nil {
			return Workspace{}, err
		}
		if model.ID == workspace.Asset.WorkingRevisionID {
			copy := revision
			workspace.Working = &copy
		}
		if model.ID == workspace.Asset.PublishedRevisionID {
			copy := revision
			workspace.Published = &copy
		}
		if model.Status == StatusCandidate {
			workspace.Candidates = append(workspace.Candidates, revision)
		} else {
			workspace.History = append(workspace.History, revision)
		}
	}
	var sources []db.SolutionSourceRef
	if err := module.conn.WithContext(ctx).Where("solution_asset_id = ?", workspace.Asset.ID).Order("observed_at DESC, id DESC").Find(&sources).Error; err != nil {
		return Workspace{}, err
	}
	for _, source := range sources {
		workspace.Sources = append(workspace.Sources, sourceDTO(source))
	}
	if err := module.conn.WithContext(ctx).Where("solution_asset_id = ?", workspace.Asset.ID).Order("id DESC").Limit(20).Find(&workspace.Jobs).Error; err != nil {
		return Workspace{}, err
	}
	workspace.Candidates = nonNil(workspace.Candidates)
	workspace.History = nonNil(workspace.History)
	workspace.Sources = nonNil(workspace.Sources)
	if workspace.Jobs == nil {
		workspace.Jobs = []db.SolutionPolishJob{}
	}
	return workspace, nil
}

type SavePromptCommand struct {
	Purpose      string
	ScopeType    string
	ScopeID      string
	Name         string
	SystemPrompt string
	Actor        string
	Activate     bool
}

func promptContentHash(value string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(hash[:])
}

func (module *Module) SavePrompt(ctx context.Context, command SavePromptCommand) (db.SolutionPromptTemplate, error) {
	command.Purpose = normalizePromptPurpose(command.Purpose)
	if !validPromptPurpose(command.Purpose) {
		return db.SolutionPromptTemplate{}, fmt.Errorf("%w: unsupported prompt purpose", ErrInvalid)
	}
	command.ScopeType = strings.ToLower(strings.TrimSpace(command.ScopeType))
	if command.ScopeType == "" {
		command.ScopeType = "global"
	}
	command.ScopeID = strings.TrimSpace(command.ScopeID)
	if command.ScopeType == "global" {
		command.ScopeID = ""
	}
	if command.ScopeType != "global" && command.ScopeType != "project" {
		return db.SolutionPromptTemplate{}, fmt.Errorf("%w: scope_type must be global or project", ErrInvalid)
	}
	if command.ScopeType == "project" && command.ScopeID == "" {
		return db.SolutionPromptTemplate{}, fmt.Errorf("%w: project scope_id is required", ErrInvalid)
	}
	if command.Purpose == "code_review" && command.ScopeType != "global" {
		return db.SolutionPromptTemplate{}, fmt.Errorf("%w: code_review skill currently supports global scope only", ErrInvalid)
	}
	if command.ScopeType == "project" {
		command.ScopeID = strings.ToUpper(command.ScopeID)
	}
	if strings.TrimSpace(command.SystemPrompt) == "" {
		return db.SolutionPromptTemplate{}, fmt.Errorf("%w: system_prompt is required", ErrInvalid)
	}
	if command.Purpose == "code_review" && command.Activate {
		return db.SolutionPromptTemplate{}, fmt.Errorf("%w: code_review skill must be saved, tested and activated explicitly", ErrInvalid)
	}
	var result db.SolutionPromptTemplate
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxVersion int
		if err := tx.Model(&db.SolutionPromptTemplate{}).
			Where("purpose = ? AND scope_type = ? AND scope_id = ?", command.Purpose, command.ScopeType, command.ScopeID).
			Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error; err != nil {
			return err
		}
		now := module.now()
		status := "draft"
		var activatedAt *time.Time
		activatedBy := ""
		if command.Activate {
			status = "active"
			activatedAt = &now
			activatedBy = command.Actor
			if err := tx.Model(&db.SolutionPromptTemplate{}).
				Where("purpose = ? AND scope_type = ? AND scope_id = ? AND status = ?", command.Purpose, command.ScopeType, command.ScopeID, "active").
				Update("status", "retired").Error; err != nil {
				return err
			}
		}
		result = db.SolutionPromptTemplate{
			Purpose: command.Purpose, ScopeType: command.ScopeType, ScopeID: command.ScopeID,
			Version: maxVersion + 1, Status: status, Name: fallback(command.Name, defaultPromptName(command.Purpose, maxVersion+1)),
			SystemPrompt: strings.TrimSpace(command.SystemPrompt), ContentHash: promptContentHash(command.SystemPrompt),
			ValidationStatus: "untested", CreatedBy: fallback(command.Actor, "unknown"),
			ActivatedBy: activatedBy, ActivatedAt: activatedAt, CreatedAt: now,
		}
		return tx.Create(&result).Error
	})
	return result, err
}

func defaultPromptName(purpose string, version int) string {
	label := "方案润色"
	switch purpose {
	case "solution_compare_requirement":
		label = "需求等价性对比"
	case "solution_compare_compatibility":
		label = "方案兼容性对比"
	case "code_review":
		label = "代码评审技能"
	}
	return fmt.Sprintf("%s v%d", label, version)
}

func (module *Module) ListPrompts(ctx context.Context) ([]db.SolutionPromptTemplate, error) {
	var prompts []db.SolutionPromptTemplate
	err := module.conn.WithContext(ctx).Where("purpose IN ?", []string{"solution_polish", "solution_compare_requirement", "solution_compare_compatibility", "code_review"}).
		Order("purpose ASC, scope_type ASC, scope_id ASC, version DESC").Find(&prompts).Error
	if prompts == nil {
		prompts = []db.SolutionPromptTemplate{}
	}
	return prompts, err
}

func (module *Module) ActivatePrompt(ctx context.Context, id uint, actor string) (db.SolutionPromptTemplate, error) {
	var result db.SolutionPromptTemplate
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND purpose IN ?", id, []string{"solution_polish", "solution_compare_requirement", "solution_compare_compatibility", "code_review"}).First(&result).Error; err != nil {
			return ErrNotFound
		}
		now := module.now()
		if result.Purpose == "code_review" {
			if result.ValidationStatus != "passed" || result.ContentHash != promptContentHash(result.SystemPrompt) {
				return fmt.Errorf("%w: code_review skill must pass validation before activation", ErrConflict)
			}
		}
		if err := tx.Model(&db.SolutionPromptTemplate{}).
			Where("purpose = ? AND scope_type = ? AND scope_id = ? AND status = ? AND id != ?", result.Purpose, result.ScopeType, result.ScopeID, "active", result.ID).
			Update("status", "retired").Error; err != nil {
			return err
		}
		if err := tx.Model(&result).Updates(map[string]any{
			"status": "active", "activated_by": fallback(actor, "unknown"), "activated_at": now,
		}).Error; err != nil {
			return err
		}
		result.Status = "active"
		result.ActivatedBy = fallback(actor, "unknown")
		result.ActivatedAt = &now
		return nil
	})
	return result, err
}

func (module *Module) RecordPromptValidation(ctx context.Context, id uint, actor, summary string, passed bool) (db.SolutionPromptTemplate, error) {
	var prompt db.SolutionPromptTemplate
	if err := module.conn.WithContext(ctx).Where("id = ? AND purpose = ?", id, "code_review").First(&prompt).Error; err != nil {
		return prompt, ErrNotFound
	}
	status := "failed"
	if passed {
		status = "passed"
	}
	now := module.now()
	if err := module.conn.WithContext(ctx).Model(&prompt).Updates(map[string]any{
		"content_hash":       promptContentHash(prompt.SystemPrompt),
		"validation_status":  status,
		"validation_summary": strings.TrimSpace(summary),
		"validated_by":       fallback(actor, "unknown"),
		"validated_at":       now,
	}).Error; err != nil {
		return prompt, err
	}
	prompt.ContentHash = promptContentHash(prompt.SystemPrompt)
	prompt.ValidationStatus = status
	prompt.ValidationSummary = strings.TrimSpace(summary)
	prompt.ValidatedBy = fallback(actor, "unknown")
	prompt.ValidatedAt = &now
	return prompt, nil
}

func normalizePromptPurpose(purpose string) string {
	purpose = strings.ToLower(strings.TrimSpace(purpose))
	if purpose == "" {
		return "solution_polish"
	}
	return purpose
}

func validPromptPurpose(purpose string) bool {
	switch purpose {
	case "solution_polish", "solution_compare_requirement", "solution_compare_compatibility", "code_review":
		return true
	default:
		return false
	}
}

func (module *Module) ActivePrompt(ctx context.Context, purpose, projectKey string) (db.SolutionPromptTemplate, error) {
	purpose = normalizePromptPurpose(purpose)
	if !validPromptPurpose(purpose) {
		return db.SolutionPromptTemplate{}, ErrInvalid
	}
	projectKey = strings.ToUpper(strings.TrimSpace(projectKey))
	var prompt db.SolutionPromptTemplate
	if projectKey != "" {
		lookup := module.conn.WithContext(ctx).
			Where("purpose = ? AND scope_type = ? AND scope_id = ? AND status = ?", purpose, "project", projectKey, "active").
			Order("version DESC").Limit(1).Find(&prompt)
		if lookup.Error != nil {
			return db.SolutionPromptTemplate{}, lookup.Error
		}
		if lookup.RowsAffected > 0 {
			return prompt, nil
		}
	}
	lookup := module.conn.WithContext(ctx).
		Where("purpose = ? AND scope_type = ? AND scope_id = ? AND status = ?", purpose, "global", "", "active").
		Order("version DESC").Limit(1).Find(&prompt)
	if lookup.Error != nil {
		return db.SolutionPromptTemplate{}, lookup.Error
	}
	if lookup.RowsAffected == 0 {
		return db.SolutionPromptTemplate{}, ErrNotFound
	}
	return prompt, nil
}

func (module *Module) ClaimNextOutbox(ctx context.Context) (*db.SolutionJiraOutbox, error) {
	var claimed *db.SolutionJiraOutbox
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := module.now()
		staleBefore := now.Add(-10 * time.Minute)
		if err := tx.Model(&db.SolutionJiraOutbox{}).
			Where("status = ? AND updated_at < ?", "processing", staleBefore).
			Updates(map[string]any{"status": "pending", "next_attempt_at": now, "updated_at": now, "last_error": "worker lease expired; queued for retry"}).Error; err != nil {
			return err
		}
		var item db.SolutionJiraOutbox
		query := tx.Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", "pending", now).
			Order("id ASC").Limit(1).Find(&item)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected == 0 {
			return nil
		}
		update := tx.Model(&db.SolutionJiraOutbox{}).Where("id = ? AND status = ?", item.ID, "pending").
			Updates(map[string]any{"status": "processing", "attempt_count": item.AttemptCount + 1, "updated_at": now, "next_attempt_at": nil})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			return nil
		}
		item.Status = "processing"
		item.AttemptCount++
		item.UpdatedAt = now
		claimed = &item
		return nil
	})
	return claimed, err
}

func (module *Module) CompleteOutbox(ctx context.Context, id uint) error {
	now := module.now()
	result := module.conn.WithContext(ctx).Model(&db.SolutionJiraOutbox{}).
		Where("id = ? AND status = ?", id, "processing").
		Updates(map[string]any{"status": "succeeded", "last_error": "", "next_attempt_at": nil, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrConflict
	}
	return nil
}

func (module *Module) FailOutbox(ctx context.Context, id uint, cause error) error {
	return module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item db.SolutionJiraOutbox
		if err := tx.First(&item, id).Error; err != nil {
			return err
		}
		if item.Status != "processing" {
			return nil
		}
		now := module.now()
		status := "failed"
		var next *time.Time
		if item.AttemptCount < 5 {
			status = "pending"
			retry := now.Add(time.Duration(item.AttemptCount*item.AttemptCount) * time.Minute)
			next = &retry
		}
		message := "Jira write failed"
		if cause != nil {
			message = cause.Error()
		}
		return tx.Model(&item).Updates(map[string]any{
			"status": status, "last_error": message, "next_attempt_at": next, "updated_at": now,
		}).Error
	})
}

func (module *Module) ensureAsset(tx *gorm.DB, demandID, actor string) (db.SolutionAsset, error) {
	var asset db.SolutionAsset
	lookup := tx.Where("demand_id = ?", demandID).Limit(1).Find(&asset)
	if lookup.Error != nil {
		return db.SolutionAsset{}, lookup.Error
	}
	if lookup.RowsAffected > 0 {
		return asset, nil
	}
	now := module.now()
	asset = db.SolutionAsset{DemandID: demandID, CreatedBy: actor, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&asset).Error; err != nil {
		return db.SolutionAsset{}, err
	}
	return asset, nil
}

func (module *Module) loadWorking(tx *gorm.DB, demandID string) (db.SolutionAsset, db.SolutionRevision, error) {
	var asset db.SolutionAsset
	if err := tx.Where("demand_id = ?", strings.TrimSpace(demandID)).First(&asset).Error; err != nil {
		return db.SolutionAsset{}, db.SolutionRevision{}, ErrNotFound
	}
	if asset.WorkingRevisionID == 0 {
		return db.SolutionAsset{}, db.SolutionRevision{}, ErrNotFound
	}
	var working db.SolutionRevision
	if err := tx.First(&working, asset.WorkingRevisionID).Error; err != nil {
		return db.SolutionAsset{}, db.SolutionRevision{}, err
	}
	return asset, working, nil
}

type revisionFields struct {
	ParentRevisionID        uint
	DerivedFromRevisionID   uint
	Kind                    string
	Status                  string
	Title                   string
	Summary                 string
	PromptTemplateVersionID uint
	ModelVersion            string
	SourceWatermark         string
	AuthoredBy              string
}

func (module *Module) revisionModel(asset db.SolutionAsset, encoded encodedMarkdown, fields revisionFields) db.SolutionRevision {
	return db.SolutionRevision{
		SolutionAssetID: asset.ID, Version: asset.Sequence + 1,
		ParentRevisionID: fields.ParentRevisionID, DerivedFromRevisionID: fields.DerivedFromRevisionID,
		Kind: fields.Kind, Status: fields.Status, Title: strings.TrimSpace(fields.Title), Summary: strings.TrimSpace(fields.Summary),
		Content: encoded.stored, ContentEncoding: encoded.encoding, ContentHash: encoded.hash,
		ContentBytes: encoded.rawBytes, StoredBytes: len(encoded.stored),
		PromptTemplateVersionID: fields.PromptTemplateVersionID, ModelVersion: strings.TrimSpace(fields.ModelVersion),
		SourceWatermark: fields.SourceWatermark, AuthoredBy: fields.AuthoredBy, CreatedAt: module.now(),
	}
}

func (module *Module) advanceWorking(tx *gorm.DB, asset db.SolutionAsset, revisionID uint, expectedRevision uint) error {
	now := module.now()
	update := tx.Model(&db.SolutionAsset{}).
		Where("id = ? AND revision = ? AND sequence = ?", asset.ID, expectedRevision, asset.Sequence).
		Updates(map[string]any{
			"revision": expectedRevision + 1, "sequence": asset.Sequence + 1,
			"working_revision_id": revisionID, "updated_at": now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected == 0 {
		return ErrConflict
	}
	return nil
}

func (module *Module) revisionDTO(model db.SolutionRevision, includeContent bool) (Revision, error) {
	revision := Revision{
		ID: model.ID, Version: model.Version, ParentRevisionID: model.ParentRevisionID,
		DerivedFromRevisionID: model.DerivedFromRevisionID, Kind: model.Kind, Status: model.Status,
		Title: model.Title, Summary: model.Summary, ContentHash: model.ContentHash,
		ContentBytes: model.ContentBytes, StoredBytes: model.StoredBytes, ContentEncoding: model.ContentEncoding,
		PromptTemplateVersionID: model.PromptTemplateVersionID, ModelVersion: model.ModelVersion,
		SourceWatermark: model.SourceWatermark, AuthoredBy: model.AuthoredBy, CreatedAt: model.CreatedAt,
	}
	if includeContent {
		markdown, err := decodeMarkdown(model.Content, model.ContentEncoding, model.ContentHash, module.maxMarkdownBytes)
		if err != nil {
			return Revision{}, err
		}
		revision.Markdown = markdown
	}
	return revision, nil
}

func sourceDTO(model db.SolutionSourceRef) SourceRef {
	return SourceRef{
		ID: model.ID, SourceSystem: model.SourceSystem, ExternalID: model.ExternalID,
		ContentHash: model.ContentHash, Author: model.Author, Marker: model.Marker, Eligible: model.Eligible, Current: model.Current,
		SourceCreatedAt: model.SourceCreatedAt, SourceUpdatedAt: model.SourceUpdatedAt, ObservedAt: model.ObservedAt,
	}
}

func (module *Module) activePrompt(tx *gorm.DB, projectKey string) (db.SolutionPromptTemplate, error) {
	projectKey = strings.TrimSpace(projectKey)
	var prompt db.SolutionPromptTemplate
	if projectKey != "" {
		lookup := tx.Where("purpose = ? AND scope_type = ? AND scope_id = ? AND status = ?", "solution_polish", "project", projectKey, "active").
			Order("version DESC").Limit(1).Find(&prompt)
		if lookup.Error != nil {
			return db.SolutionPromptTemplate{}, lookup.Error
		}
		if lookup.RowsAffected > 0 {
			return prompt, nil
		}
	}
	err := tx.Where("purpose = ? AND scope_type = ? AND scope_id = ? AND status = ?", "solution_polish", "global", "", "active").
		Order("version DESC").First(&prompt).Error
	if err != nil {
		return db.SolutionPromptTemplate{}, fmt.Errorf("%w: no active solution prompt", ErrNotFound)
	}
	return prompt, nil
}

func (module *Module) latestEligibleSources(tx *gorm.DB, assetID uint) ([]db.SolutionSourceRef, string, error) {
	var all []db.SolutionSourceRef
	if err := tx.Where("solution_asset_id = ? AND eligible = ? AND current = ?", assetID, true, true).Order("observed_at ASC, id ASC").Find(&all).Error; err != nil {
		return nil, "", err
	}
	latest := make(map[string]db.SolutionSourceRef)
	for _, source := range all {
		key := source.SourceSystem + ":" + source.ExternalID
		latest[key] = source
	}
	keys := make([]string, 0, len(latest))
	for key := range latest {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]db.SolutionSourceRef, 0, len(keys))
	var fingerprint strings.Builder
	for _, key := range keys {
		source := latest[key]
		result = append(result, source)
		fmt.Fprintf(&fingerprint, "%s:%d:%s\n", key, source.ID, source.ContentHash)
	}
	return result, hashBytes([]byte(fingerprint.String())), nil
}

func fallback(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
