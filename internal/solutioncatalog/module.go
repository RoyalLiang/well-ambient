package solutioncatalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"well-ambient/internal/db"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound  = errors.New("catalog record not found")
	ErrConflict  = errors.New("catalog record conflict")
	ErrInvalid   = errors.New("invalid catalog command")
	ErrIntegrity = errors.New("catalog payload integrity failure")
)

const (
	SyncQueued     = "queued"
	SyncProcessing = "processing"
	SyncSucceeded  = "succeeded"
	SyncFailed     = "failed"
	SyncObsolete   = "obsolete"

	ComparisonQueued       = "queued"
	ComparisonRunning      = "running"
	ComparisonDismissed    = "not_similar"
	ComparisonIncompatible = "incompatible"
	ComparisonNeedsReview  = "needs_review"
	ComparisonCompleted    = "completed"
	ComparisonFailed       = "failed"

	ComparisonStageRoundOne = "round_one"
	ComparisonStageRoundTwo = "round_two"

	ProposalPending  = "pending"
	ProposalAccepted = "accepted"
	ProposalRejected = "rejected"

	StandardActive  = "active"
	StandardRetired = "retired"
)

type Settings struct {
	Interval        time.Duration
	CandidateLimit  int
	RecallThreshold float64
}

func (settings Settings) normalized() Settings {
	if settings.Interval <= 0 {
		settings.Interval = 30 * time.Minute
	}
	if settings.CandidateLimit <= 0 {
		settings.CandidateLimit = 8
	}
	if settings.CandidateLimit > 30 {
		settings.CandidateLimit = 30
	}
	if settings.RecallThreshold <= 0 || settings.RecallThreshold > 1 {
		settings.RecallThreshold = 0.18
	}
	return settings
}

type Module struct {
	conn     *gorm.DB
	settings Settings
	now      func() time.Time

	runMu   sync.Mutex
	stateMu sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
}

type Option func(*Module)

func WithClock(now func() time.Time) Option {
	return func(module *Module) {
		if now != nil {
			module.now = now
		}
	}
}

func New(conn *gorm.DB, settings Settings, options ...Option) *Module {
	module := &Module{conn: conn, settings: settings.normalized(), now: time.Now}
	for _, option := range options {
		option(module)
	}
	return module
}

// EnqueuePublished records the catalog hand-off in the caller's publication
// transaction. Replays are harmless because the revision identity is unique.
func EnqueuePublished(tx *gorm.DB, asset db.SolutionAsset, revision db.SolutionRevision, now time.Time) error {
	if tx == nil || asset.ID == 0 || revision.ID == 0 {
		return fmt.Errorf("%w: published asset and revision are required", ErrInvalid)
	}
	job := db.SolutionCatalogSyncJob{
		SolutionAssetID: asset.ID, PublishedRevisionID: revision.ID,
		Status: SyncQueued, CreatedAt: now, UpdatedAt: now,
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "published_revision_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"solution_asset_id": asset.ID, "status": SyncQueued, "last_error": "",
			"next_attempt_at": nil, "completed_at": nil, "updated_at": now,
		}),
		Where: clause.Where{Exprs: []clause.Expression{clause.Neq{Column: "status", Value: SyncProcessing}}},
	}).Create(&job).Error
}

// Start runs a local-only periodic reconciler. Incremental publication jobs are
// also drained by the existing solution worker, so a long interval does not
// delay normal publishes.
func (module *Module) Start(parent context.Context) bool {
	if module == nil || module.conn == nil {
		return false
	}
	module.stateMu.Lock()
	if module.cancel != nil {
		module.stateMu.Unlock()
		return false
	}
	ctx, cancel := context.WithCancel(parent)
	module.cancel = cancel
	module.done = make(chan struct{})
	done := module.done
	module.stateMu.Unlock()
	go func() {
		defer close(done)
		_, _ = module.Reconcile(ctx)
		ticker := time.NewTicker(module.settings.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = module.Reconcile(ctx)
			}
		}
	}()
	return true
}

func (module *Module) Stop() {
	if module == nil {
		return
	}
	module.stateMu.Lock()
	cancel, done := module.cancel, module.done
	module.cancel, module.done = nil, nil
	module.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// Reconcile repairs missed publication hand-offs without rewriting catalog
// history. It only queues entries whose published revision is not projected.
func (module *Module) Reconcile(ctx context.Context) (int, error) {
	if module == nil || module.conn == nil {
		return 0, ErrNotFound
	}
	module.runMu.Lock()
	defer module.runMu.Unlock()

	var assets []db.SolutionAsset
	if err := module.conn.WithContext(ctx).
		Where("published_revision_id <> 0").Order("id ASC").Find(&assets).Error; err != nil {
		return 0, err
	}
	queued := 0
	for _, asset := range assets {
		var entry db.SolutionCatalogEntry
		lookup := module.conn.WithContext(ctx).Where("solution_asset_id = ?", asset.ID).Limit(1).Find(&entry)
		if lookup.Error != nil {
			return queued, lookup.Error
		}
		if lookup.RowsAffected > 0 && entry.PublishedRevisionID == asset.PublishedRevisionID {
			continue
		}
		var revision db.SolutionRevision
		if err := module.conn.WithContext(ctx).First(&revision, asset.PublishedRevisionID).Error; err != nil {
			return queued, err
		}
		if err := EnqueuePublished(module.conn.WithContext(ctx), asset, revision, module.now().UTC()); err != nil {
			return queued, err
		}
		queued++
	}
	return queued, nil
}

func (module *Module) ProcessPending(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 1
	}
	processed := 0
	for processed < limit {
		job, err := module.claimSyncJob(ctx)
		if err != nil {
			return processed, err
		}
		if job == nil {
			return processed, nil
		}
		if err := module.syncPublished(ctx, *job); err != nil {
			if failErr := module.failSyncJob(ctx, job.ID, err); failErr != nil {
				return processed, errors.Join(err, failErr)
			}
			return processed + 1, err
		}
		processed++
	}
	return processed, nil
}

func (module *Module) claimSyncJob(ctx context.Context) (*db.SolutionCatalogSyncJob, error) {
	var claimed *db.SolutionCatalogSyncJob
	err := module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := module.now().UTC()
		staleBefore := now.Add(-10 * time.Minute)
		if err := tx.Model(&db.SolutionCatalogSyncJob{}).
			Where("status = ? AND updated_at < ?", SyncProcessing, staleBefore).
			Updates(map[string]any{"status": SyncQueued, "next_attempt_at": now, "updated_at": now, "last_error": "catalog worker lease expired"}).Error; err != nil {
			return err
		}
		var job db.SolutionCatalogSyncJob
		lookup := tx.Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", SyncQueued, now).
			Order("id ASC").Limit(1).Find(&job)
		if lookup.Error != nil || lookup.RowsAffected == 0 {
			return lookup.Error
		}
		update := tx.Model(&db.SolutionCatalogSyncJob{}).Where("id = ? AND status = ?", job.ID, SyncQueued).
			Updates(map[string]any{"status": SyncProcessing, "attempt_count": job.AttemptCount + 1, "started_at": now, "next_attempt_at": nil, "updated_at": now})
		if update.Error != nil || update.RowsAffected == 0 {
			return update.Error
		}
		job.Status, job.AttemptCount, job.StartedAt, job.UpdatedAt = SyncProcessing, job.AttemptCount+1, &now, now
		claimed = &job
		return nil
	})
	return claimed, err
}

func (module *Module) syncPublished(ctx context.Context, job db.SolutionCatalogSyncJob) error {
	return module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset db.SolutionAsset
		if err := tx.First(&asset, job.SolutionAssetID).Error; err != nil {
			return err
		}
		if asset.PublishedRevisionID != job.PublishedRevisionID {
			now := module.now().UTC()
			return tx.Model(&db.SolutionCatalogSyncJob{}).Where("id = ?", job.ID).
				Updates(map[string]any{"status": SyncObsolete, "completed_at": now, "updated_at": now}).Error
		}
		var revision db.SolutionRevision
		if err := tx.First(&revision, job.PublishedRevisionID).Error; err != nil {
			return err
		}
		if revision.Status != "published" {
			return fmt.Errorf("%w: revision %d is not published", ErrConflict, revision.ID)
		}

		var demand db.TaskTelemetry
		demandLookup := tx.Where("task_id = ?", asset.DemandID).Limit(1).Find(&demand)
		if demandLookup.Error != nil {
			return demandLookup.Error
		}
		projectKey := projectKeyFromDemandID(asset.DemandID)
		demandTitle := ""
		if demandLookup.RowsAffected > 0 {
			projectKey = db.ResolveTaskProjectKey(demand)
			demandTitle = strings.TrimSpace(demand.Title)
		}
		tokens := catalogTokens(asset.DemandID, projectKey, demandTitle, revision.Title, revision.Summary)
		now := module.now().UTC()
		publishedAt := asset.UpdatedAt.UTC()
		if publishedAt.IsZero() {
			publishedAt = now
		}
		entry := db.SolutionCatalogEntry{
			SolutionAssetID: asset.ID, PublishedRevisionID: revision.ID,
			DemandID: asset.DemandID, ProjectKey: projectKey, DemandTitle: demandTitle,
			SolutionTitle: revision.Title, Summary: revision.Summary,
			ContentHash: revision.ContentHash, ContentBytes: revision.ContentBytes, StoredBytes: revision.StoredBytes,
			SearchTokenCount: len(tokens), PublishedAt: publishedAt, SyncedAt: now,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "solution_asset_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"published_revision_id", "demand_id", "project_key", "demand_title", "solution_title", "summary",
				"content_hash", "content_bytes", "stored_bytes", "search_token_count", "published_at", "synced_at", "updated_at",
			}),
		}).Create(&entry).Error; err != nil {
			return err
		}
		if err := tx.Where("solution_asset_id = ?", asset.ID).First(&entry).Error; err != nil {
			return err
		}
		if err := tx.Where("entry_id = ?", entry.ID).Delete(&db.SolutionCatalogSearchToken{}).Error; err != nil {
			return err
		}
		if len(tokens) > 0 {
			rows := make([]db.SolutionCatalogSearchToken, 0, len(tokens))
			for _, token := range tokens {
				rows = append(rows, db.SolutionCatalogSearchToken{EntryID: entry.ID, Token: token})
			}
			if err := tx.CreateInBatches(&rows, 100).Error; err != nil {
				return err
			}
		}
		if err := module.queueSimilarityCandidates(tx, entry, tokens); err != nil {
			return err
		}
		return tx.Model(&db.SolutionCatalogSyncJob{}).Where("id = ? AND status = ?", job.ID, SyncProcessing).
			Updates(map[string]any{"status": SyncSucceeded, "last_error": "", "completed_at": now, "updated_at": now}).Error
	})
}

func (module *Module) queueSimilarityCandidates(tx *gorm.DB, entry db.SolutionCatalogEntry, tokens []string) error {
	type candidateMatch struct {
		EntryID uint
		Matches int
	}
	matches := make(map[uint]int)
	if len(tokens) > 0 {
		var rows []candidateMatch
		if err := tx.Model(&db.SolutionCatalogSearchToken{}).
			Select("entry_id, COUNT(*) AS matches").
			Where("token IN ? AND entry_id <> ?", tokens, entry.ID).
			Group("entry_id").Order("matches DESC").Limit(module.settings.CandidateLimit * 4).
			Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			matches[row.EntryID] = row.Matches
		}
	}
	var duplicates []db.SolutionCatalogEntry
	if err := tx.Where("content_hash = ? AND id <> ?", entry.ContentHash, entry.ID).
		Limit(module.settings.CandidateLimit).Find(&duplicates).Error; err != nil {
		return err
	}
	for _, duplicate := range duplicates {
		matches[duplicate.ID] = max(matches[duplicate.ID], min(entry.SearchTokenCount, duplicate.SearchTokenCount))
	}
	if len(matches) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(matches))
	for id := range matches {
		ids = append(ids, id)
	}
	var candidates []db.SolutionCatalogEntry
	if err := tx.Where("id IN ?", ids).Find(&candidates).Error; err != nil {
		return err
	}
	type scoredCandidate struct {
		entry db.SolutionCatalogEntry
		score float64
	}
	scored := make([]scoredCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		denominator := entry.SearchTokenCount + candidate.SearchTokenCount
		score := 0.0
		if denominator > 0 {
			score = 2 * float64(matches[candidate.ID]) / float64(denominator)
		}
		if entry.ContentHash == candidate.ContentHash {
			score = 1
		}
		if score >= module.settings.RecallThreshold {
			scored = append(scored, scoredCandidate{entry: candidate, score: score})
		}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	if len(scored) > module.settings.CandidateLimit {
		scored = scored[:module.settings.CandidateLimit]
	}
	now := module.now().UTC()
	for _, candidate := range scored {
		left, right := entry, candidate.entry
		if left.PublishedRevisionID > right.PublishedRevisionID {
			left, right = right, left
		}
		comparison := db.SolutionComparison{
			PairKey:     fmt.Sprintf("revision:%d:%d", left.PublishedRevisionID, right.PublishedRevisionID),
			LeftEntryID: left.ID, RightEntryID: right.ID,
			LeftRevisionID: left.PublishedRevisionID, RightRevisionID: right.PublishedRevisionID,
			RecallScore: candidate.score, Status: ComparisonQueued, Stage: ComparisonStageRoundOne,
			RoundOnePromptVersion: "", RoundTwoPromptVersion: "",
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&comparison).Error; err != nil {
			return err
		}
	}
	return nil
}

func (module *Module) failSyncJob(ctx context.Context, id uint, cause error) error {
	return module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job db.SolutionCatalogSyncJob
		if err := tx.First(&job, id).Error; err != nil {
			return err
		}
		if job.Status != SyncProcessing {
			return nil
		}
		now := module.now().UTC()
		status := SyncFailed
		var next *time.Time
		if job.AttemptCount < 5 {
			status = SyncQueued
			retry := now.Add(time.Duration(job.AttemptCount*job.AttemptCount) * time.Minute)
			next = &retry
		}
		message := "catalog sync failed"
		if cause != nil {
			message = cause.Error()
		}
		return tx.Model(&job).Updates(map[string]any{
			"status": status, "last_error": message, "next_attempt_at": next, "updated_at": now,
		}).Error
	})
}

func allowedProject(projectKey string, scope []string) bool {
	scope = db.NormalizeProjectKeys(scope)
	if len(scope) == 0 {
		return true
	}
	projectKey = strings.ToUpper(strings.TrimSpace(projectKey))
	for _, candidate := range scope {
		if projectKey == candidate {
			return true
		}
	}
	return false
}

func applyProjectScope(query *gorm.DB, scope []string) *gorm.DB {
	scope = db.NormalizeProjectKeys(scope)
	if len(scope) == 0 {
		return query
	}
	return query.Where("project_key IN ?", scope)
}

func decodeSolutionRevision(revision db.SolutionRevision) (string, error) {
	raw, err := decodePayload(revision.Content, revision.ContentEncoding, revision.ContentHash)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func marshalAndEncode(value any) (encodedPayload, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return encodedPayload{}, err
	}
	return encodePayload(raw)
}
