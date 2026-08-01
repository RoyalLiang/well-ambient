package dataassets

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"well-ambient/internal/db"

	"gorm.io/gorm"
)

const (
	defaultCompressionThreshold = 4 * 1024
	defaultMaxPayloadBytes      = 8 * 1024 * 1024
	defaultTimelineLimit        = 100
	defaultMaxTimelineLimit     = 200
	defaultMaxEvidence          = 5000
	evidenceBatchSize           = 400
)

type Module struct {
	conn                 *gorm.DB
	now                  func() time.Time
	compressionThreshold int
	maxPayloadBytes      int64
	maxTimelineLimit     int
	maxEvidence          int
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

func WithMaxPayloadBytes(bytes int64) Option {
	return func(module *Module) {
		if bytes > 0 {
			module.maxPayloadBytes = bytes
		}
	}
}

func New(conn *gorm.DB, options ...Option) *Module {
	module := &Module{
		conn:                 conn,
		now:                  time.Now,
		compressionThreshold: defaultCompressionThreshold,
		maxPayloadBytes:      defaultMaxPayloadBytes,
		maxTimelineLimit:     defaultMaxTimelineLimit,
		maxEvidence:          defaultMaxEvidence,
	}
	for _, option := range options {
		option(module)
	}
	return module
}

func (module *Module) Append(ctx context.Context, command AppendCommand) (AppendResult, error) {
	if module == nil || module.conn == nil {
		return AppendResult{}, ErrNotInitialized
	}
	prepared, err := module.prepareAppend(command)
	if err != nil {
		return AppendResult{}, err
	}

	row := db.DataAssetEvent{
		DedupeKey:         prepared.command.DedupeKey,
		Fingerprint:       prepared.fingerprint,
		ProjectKey:        prepared.command.ProjectKey,
		SubjectType:       prepared.command.SubjectType,
		SubjectID:         prepared.command.SubjectID,
		EventType:         prepared.command.EventType,
		SourceSystem:      prepared.command.SourceSystem,
		SourceRecordID:    prepared.command.SourceRecordID,
		SourceEventID:     prepared.command.SourceEventID,
		ActorID:           prepared.command.ActorID,
		ActorRole:         prepared.command.ActorRole,
		CorrelationID:     prepared.command.CorrelationID,
		CausationID:       prepared.command.CausationID,
		SupersedesEventID: prepared.command.SupersedesEventID,
		Classification:    prepared.command.Classification,
		RetentionClass:    prepared.command.RetentionClass,
		SchemaVersion:     prepared.command.SchemaVersion,
		OccurredAt:        prepared.command.OccurredAt,
		ObservedAt:        prepared.command.ObservedAt,
		RecordedAt:        module.now().UTC(),
		ExpiresAt:         prepared.command.ExpiresAt,
		PayloadHash:       prepared.encoded.hash,
		PayloadEncoding:   prepared.encoded.encoding,
		PayloadBytes:      int64(len(prepared.encoded.raw)),
		StoredBytes:       int64(len(prepared.encoded.stored)),
	}
	result := AppendResult{}
	err = module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing db.DataAssetEvent
		existingErr := tx.Where("dedupe_key = ?", row.DedupeKey).First(&existing).Error
		if existingErr == nil {
			if existing.Fingerprint != prepared.fingerprint {
				return fmt.Errorf("%w: dedupe key %q has different content", ErrIdempotencyConflict, row.DedupeKey)
			}
			result = AppendResult{Event: eventFromRow(existing), Replayed: true}
			return nil
		}
		if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
			return existingErr
		}
		if row.SupersedesEventID > 0 {
			var count int64
			if err := tx.Model(&db.DataAssetEvent{}).Where("id = ?", row.SupersedesEventID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("%w: superseded event %d does not exist", ErrInvalidCommand, row.SupersedesEventID)
			}
		}
		if err := tx.Create(&row).Error; err != nil {
			var raced db.DataAssetEvent
			if lookupErr := tx.Where("dedupe_key = ?", row.DedupeKey).First(&raced).Error; lookupErr == nil {
				if raced.Fingerprint != prepared.fingerprint {
					return fmt.Errorf("%w: dedupe key %q has different content", ErrIdempotencyConflict, row.DedupeKey)
				}
				result = AppendResult{Event: eventFromRow(raced), Replayed: true}
				return nil
			}
			return err
		}
		payload := db.DataAssetEventPayload{
			EventID:      row.ID,
			ContentType:  "application/json",
			Encoding:     prepared.encoded.encoding,
			Data:         prepared.encoded.stored,
			OriginalSize: int64(len(prepared.encoded.raw)),
			StoredSize:   int64(len(prepared.encoded.stored)),
			CreatedAt:    row.RecordedAt,
		}
		if err := tx.Create(&payload).Error; err != nil {
			return err
		}
		result = AppendResult{Event: eventFromRow(row)}
		return nil
	})
	return result, err
}

// InspectAppend classifies a prospective append using the exact normalization
// and fingerprint rules used by Append, without changing the ledger.
func (module *Module) InspectAppend(ctx context.Context, command AppendCommand) (AppendInspection, error) {
	if module == nil || module.conn == nil {
		return AppendInspection{}, ErrNotInitialized
	}
	prepared, err := module.prepareAppend(command)
	if err != nil {
		return AppendInspection{}, err
	}
	var existing db.DataAssetEvent
	err = module.conn.WithContext(ctx).Where("dedupe_key = ?", prepared.command.DedupeKey).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if prepared.command.SupersedesEventID > 0 {
			var count int64
			if countErr := module.conn.WithContext(ctx).Model(&db.DataAssetEvent{}).
				Where("id = ?", prepared.command.SupersedesEventID).Count(&count).Error; countErr != nil {
				return AppendInspection{}, countErr
			}
			if count != 1 {
				return AppendInspection{}, fmt.Errorf("%w: superseded event %d does not exist", ErrInvalidCommand, prepared.command.SupersedesEventID)
			}
		}
		return AppendInspection{}, nil
	}
	if err != nil {
		return AppendInspection{}, err
	}
	inspection := AppendInspection{Exists: true, Event: eventFromRow(existing)}
	if existing.Fingerprint == prepared.fingerprint {
		inspection.Replayed = true
	} else {
		inspection.Conflict = true
	}
	return inspection, nil
}

func (module *Module) Timeline(ctx context.Context, query TimelineQuery) (TimelinePage, error) {
	if module == nil || module.conn == nil {
		return TimelinePage{}, ErrNotInitialized
	}
	normalized, filterHash, err := module.normalizeTimeline(query)
	if err != nil {
		return TimelinePage{}, err
	}

	highWatermark := uint(0)
	var cursor timelineCursor
	if normalized.Cursor != "" {
		cursor, err = decodeTimelineCursor(normalized.Cursor)
		if err != nil {
			return TimelinePage{}, err
		}
		if cursor.FilterHash != filterHash {
			return TimelinePage{}, ErrCursorFilterMismatch
		}
		highWatermark = cursor.HighWatermark
	} else {
		var watermark struct{ MaxID uint }
		watermarkQuery := applyTimelineFilters(module.conn.WithContext(ctx).Model(&db.DataAssetEvent{}), normalized)
		if err := watermarkQuery.Select("COALESCE(MAX(id), 0) AS max_id").Scan(&watermark).Error; err != nil {
			return TimelinePage{}, err
		}
		highWatermark = watermark.MaxID
	}

	page := TimelinePage{Items: []Event{}, HighWatermark: highWatermark}
	if highWatermark == 0 {
		return page, nil
	}
	rows := make([]db.DataAssetEvent, 0, normalized.Limit+1)
	listQuery := applyTimelineFilters(module.conn.WithContext(ctx).Model(&db.DataAssetEvent{}), normalized).
		Where("id <= ?", highWatermark)
	if normalized.Cursor != "" {
		listQuery = listQuery.Where(
			"(occurred_at < ? OR (occurred_at = ? AND id < ?))",
			cursor.OccurredAt,
			cursor.OccurredAt,
			cursor.ID,
		)
	}
	if err := listQuery.
		Order("occurred_at DESC, id DESC").
		Limit(normalized.Limit + 1).
		Find(&rows).Error; err != nil {
		return TimelinePage{}, err
	}
	hasMore := len(rows) > normalized.Limit
	if hasMore {
		rows = rows[:normalized.Limit]
	}
	page.Items = make([]Event, 0, len(rows))
	for _, row := range rows {
		page.Items = append(page.Items, eventFromRow(row))
	}
	if hasMore && len(rows) > 0 {
		page.NextCursor, err = encodeTimelineCursor(timelineCursor{
			Version:       1,
			OccurredAt:    rows[len(rows)-1].OccurredAt.UTC(),
			ID:            rows[len(rows)-1].ID,
			HighWatermark: highWatermark,
			FilterHash:    filterHash,
		})
		if err != nil {
			return TimelinePage{}, err
		}
	}
	return page, nil
}

func (module *Module) Load(ctx context.Context, eventID uint) (EventRecord, error) {
	if module == nil || module.conn == nil {
		return EventRecord{}, ErrNotInitialized
	}
	if eventID == 0 {
		return EventRecord{}, fmt.Errorf("%w: event id is required", ErrInvalidCommand)
	}
	var event db.DataAssetEvent
	if err := module.conn.WithContext(ctx).First(&event, eventID).Error; err != nil {
		return EventRecord{}, err
	}
	var payload db.DataAssetEventPayload
	if err := module.conn.WithContext(ctx).Where("event_id = ?", eventID).First(&payload).Error; err != nil {
		return EventRecord{}, err
	}
	raw, err := decodePayload(payload.Data, payload.Encoding, event.PayloadHash, module.maxPayloadBytes)
	if err != nil {
		return EventRecord{}, err
	}
	return EventRecord{Event: eventFromRow(event), Payload: json.RawMessage(raw)}, nil
}

func (module *Module) SealSnapshot(ctx context.Context, command SealSnapshotCommand) (SealSnapshotResult, error) {
	if module == nil || module.conn == nil {
		return SealSnapshotResult{}, ErrNotInitialized
	}
	normalized, evidenceIDs, err := module.normalizeSnapshot(command)
	if err != nil {
		return SealSnapshotResult{}, err
	}
	encoded, err := encodePayload(normalized.Payload, module.compressionThreshold, module.maxPayloadBytes)
	if err != nil {
		return SealSnapshotResult{}, err
	}
	evidenceHash, err := hashJSON(evidenceIDs)
	if err != nil {
		return SealSnapshotResult{}, err
	}
	fingerprint, err := snapshotFingerprint(normalized, evidenceHash, encoded.hash)
	if err != nil {
		return SealSnapshotResult{}, err
	}
	row := db.DataAssetSnapshot{
		DedupeKey:          normalized.DedupeKey,
		Fingerprint:        fingerprint,
		Kind:               normalized.Kind,
		ScopeType:          normalized.ScopeType,
		ScopeID:            normalized.ScopeID,
		AsOf:               normalized.AsOf,
		InputHighWatermark: normalized.InputHighWatermark,
		Producer:           normalized.Producer,
		ProducerVersion:    normalized.ProducerVersion,
		CreatedBy:          normalized.CreatedBy,
		Classification:     normalized.Classification,
		RetentionClass:     normalized.RetentionClass,
		ExpiresAt:          normalized.ExpiresAt,
		EvidenceCount:      len(evidenceIDs),
		EvidenceHash:       evidenceHash,
		PayloadHash:        encoded.hash,
		PayloadEncoding:    encoded.encoding,
		PayloadBytes:       int64(len(encoded.raw)),
		StoredBytes:        int64(len(encoded.stored)),
		CreatedAt:          module.now().UTC(),
	}
	result := SealSnapshotResult{}
	err = module.conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateEvidence(tx, evidenceIDs, row.InputHighWatermark, row.AsOf); err != nil {
			return err
		}
		var existing db.DataAssetSnapshot
		existingErr := tx.Where("dedupe_key = ?", normalized.DedupeKey).First(&existing).Error
		if existingErr == nil {
			if existing.Fingerprint != fingerprint {
				return fmt.Errorf("%w: snapshot dedupe key %q has different content", ErrIdempotencyConflict, normalized.DedupeKey)
			}
			result = SealSnapshotResult{Snapshot: snapshotFromRow(existing), Replayed: true}
			return nil
		}
		if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
			return existingErr
		}
		if err := tx.Create(&row).Error; err != nil {
			var raced db.DataAssetSnapshot
			if lookupErr := tx.Where("dedupe_key = ?", normalized.DedupeKey).First(&raced).Error; lookupErr == nil {
				if raced.Fingerprint != fingerprint {
					return fmt.Errorf("%w: snapshot dedupe key %q has different content", ErrIdempotencyConflict, normalized.DedupeKey)
				}
				result = SealSnapshotResult{Snapshot: snapshotFromRow(raced), Replayed: true}
				return nil
			}
			return err
		}
		payload := db.DataAssetSnapshotPayload{
			SnapshotID:   row.ID,
			ContentType:  "application/json",
			Encoding:     encoded.encoding,
			Data:         encoded.stored,
			OriginalSize: int64(len(encoded.raw)),
			StoredSize:   int64(len(encoded.stored)),
			CreatedAt:    row.CreatedAt,
		}
		if err := tx.Create(&payload).Error; err != nil {
			return err
		}
		if len(evidenceIDs) > 0 {
			evidence := make([]db.DataAssetSnapshotEvidence, 0, len(evidenceIDs))
			for position, eventID := range evidenceIDs {
				evidence = append(evidence, db.DataAssetSnapshotEvidence{
					SnapshotID: row.ID,
					EventID:    eventID,
					Position:   position,
				})
			}
			if err := tx.CreateInBatches(evidence, evidenceBatchSize).Error; err != nil {
				return err
			}
		}
		result = SealSnapshotResult{Snapshot: snapshotFromRow(row)}
		return nil
	})
	return result, err
}

func (module *Module) LoadSnapshot(ctx context.Context, snapshotID uint) (SnapshotRecord, error) {
	if module == nil || module.conn == nil {
		return SnapshotRecord{}, ErrNotInitialized
	}
	if snapshotID == 0 {
		return SnapshotRecord{}, fmt.Errorf("%w: snapshot id is required", ErrInvalidCommand)
	}
	var row db.DataAssetSnapshot
	if err := module.conn.WithContext(ctx).First(&row, snapshotID).Error; err != nil {
		return SnapshotRecord{}, err
	}
	return module.loadSnapshotRow(ctx, row)
}

func (module *Module) LatestSnapshot(ctx context.Context, query LatestSnapshotQuery) (SnapshotRecord, error) {
	if module == nil || module.conn == nil {
		return SnapshotRecord{}, ErrNotInitialized
	}
	query.Kind = normalizeToken(query.Kind)
	query.ScopeType = normalizeToken(query.ScopeType)
	query.ScopeID = strings.TrimSpace(query.ScopeID)
	if query.Kind == "" || query.ScopeType == "" || query.ScopeID == "" {
		return SnapshotRecord{}, fmt.Errorf("%w: kind, scope type, and scope id are required", ErrInvalidCommand)
	}
	conn := module.conn.WithContext(ctx).Where(
		"kind = ? AND scope_type = ? AND scope_id = ?",
		query.Kind,
		query.ScopeType,
		query.ScopeID,
	)
	if query.AsOf != nil {
		asOf := query.AsOf.UTC()
		conn = conn.Where("as_of <= ?", asOf)
	}
	var row db.DataAssetSnapshot
	if err := conn.Order("as_of DESC, id DESC").First(&row).Error; err != nil {
		return SnapshotRecord{}, err
	}
	return module.loadSnapshotRow(ctx, row)
}

func (module *Module) loadSnapshotRow(ctx context.Context, row db.DataAssetSnapshot) (SnapshotRecord, error) {
	var payload db.DataAssetSnapshotPayload
	if err := module.conn.WithContext(ctx).Where("snapshot_id = ?", row.ID).First(&payload).Error; err != nil {
		return SnapshotRecord{}, err
	}
	raw, err := decodePayload(payload.Data, payload.Encoding, row.PayloadHash, module.maxPayloadBytes)
	if err != nil {
		return SnapshotRecord{}, err
	}
	var evidence []db.DataAssetSnapshotEvidence
	if err := module.conn.WithContext(ctx).
		Where("snapshot_id = ?", row.ID).
		Order("position ASC").
		Find(&evidence).Error; err != nil {
		return SnapshotRecord{}, err
	}
	evidenceIDs := make([]uint, 0, len(evidence))
	for _, item := range evidence {
		evidenceIDs = append(evidenceIDs, item.EventID)
	}
	return SnapshotRecord{
		Snapshot:         snapshotFromRow(row),
		EvidenceEventIDs: evidenceIDs,
		Payload:          json.RawMessage(raw),
	}, nil
}

func (module *Module) normalizeAppend(command AppendCommand) (AppendCommand, error) {
	command.DedupeKey = strings.TrimSpace(command.DedupeKey)
	command.ProjectKey = strings.ToUpper(strings.TrimSpace(command.ProjectKey))
	command.SubjectType = normalizeToken(command.SubjectType)
	command.SubjectID = strings.TrimSpace(command.SubjectID)
	command.EventType = normalizeToken(command.EventType)
	command.SourceSystem = normalizeToken(command.SourceSystem)
	command.SourceRecordID = strings.TrimSpace(command.SourceRecordID)
	command.SourceEventID = strings.TrimSpace(command.SourceEventID)
	command.ActorID = strings.TrimSpace(command.ActorID)
	command.ActorRole = normalizeToken(command.ActorRole)
	command.CorrelationID = strings.TrimSpace(command.CorrelationID)
	command.CausationID = strings.TrimSpace(command.CausationID)
	command.Classification = normalizeClassification(command.Classification)
	command.RetentionClass = normalizeRetention(command.RetentionClass)
	if command.ActorRole == "" {
		command.ActorRole = "system"
	}
	if command.SchemaVersion <= 0 {
		command.SchemaVersion = 1
	}
	if command.ObservedAt.IsZero() {
		command.ObservedAt = module.now()
	}
	command.OccurredAt = command.OccurredAt.UTC()
	command.ObservedAt = command.ObservedAt.UTC()
	command.ExpiresAt = normalizeTimePointer(command.ExpiresAt)
	if command.DedupeKey == "" || command.SubjectType == "" || command.SubjectID == "" ||
		command.EventType == "" || command.SourceSystem == "" || command.SourceRecordID == "" ||
		command.ActorID == "" || command.OccurredAt.IsZero() {
		return AppendCommand{}, fmt.Errorf("%w: dedupe key, subject, event type, source record, actor, and occurred time are required", ErrInvalidCommand)
	}
	if !validClassification(command.Classification) {
		return AppendCommand{}, fmt.Errorf("%w: unsupported classification %q", ErrInvalidCommand, command.Classification)
	}
	if !validRetention(command.RetentionClass) {
		return AppendCommand{}, fmt.Errorf("%w: unsupported retention class %q", ErrInvalidCommand, command.RetentionClass)
	}
	if command.RetentionClass == RetentionLegalHold && command.ExpiresAt != nil {
		return AppendCommand{}, fmt.Errorf("%w: legal-hold assets cannot expire", ErrInvalidCommand)
	}
	return command, nil
}

func (module *Module) normalizeTimeline(query TimelineQuery) (TimelineQuery, string, error) {
	query.ProjectKey = strings.ToUpper(strings.TrimSpace(query.ProjectKey))
	query.SubjectType = normalizeToken(query.SubjectType)
	query.SubjectID = strings.TrimSpace(query.SubjectID)
	query.SourceSystem = normalizeToken(query.SourceSystem)
	query.SourceRecordID = strings.TrimSpace(query.SourceRecordID)
	query.CorrelationID = strings.TrimSpace(query.CorrelationID)
	query.EventTypes = normalizeTokens(query.EventTypes)
	query.Since = normalizeTimePointer(query.Since)
	query.Until = normalizeTimePointer(query.Until)
	if (query.SubjectType == "") != (query.SubjectID == "") {
		return TimelineQuery{}, "", fmt.Errorf("%w: subject type and id must be supplied together", ErrInvalidCommand)
	}
	if query.SourceRecordID != "" && query.SourceSystem == "" {
		return TimelineQuery{}, "", fmt.Errorf("%w: source system is required with source record id", ErrInvalidCommand)
	}
	if query.Since != nil && query.Until != nil && query.Since.After(*query.Until) {
		return TimelineQuery{}, "", fmt.Errorf("%w: since must not be after until", ErrInvalidCommand)
	}
	if query.ProjectKey == "" && query.SubjectID == "" && query.SourceSystem == "" &&
		query.SourceRecordID == "" && query.CorrelationID == "" && len(query.EventTypes) == 0 &&
		query.Since == nil && query.Until == nil {
		return TimelineQuery{}, "", ErrQueryScopeRequired
	}
	if query.Limit <= 0 {
		query.Limit = defaultTimelineLimit
	}
	if query.Limit > module.maxTimelineLimit {
		query.Limit = module.maxTimelineLimit
	}
	filterHash, err := timelineFilterHash(query)
	if err != nil {
		return TimelineQuery{}, "", err
	}
	return query, filterHash, nil
}

func (module *Module) normalizeSnapshot(command SealSnapshotCommand) (SealSnapshotCommand, []uint, error) {
	command.DedupeKey = strings.TrimSpace(command.DedupeKey)
	command.Kind = normalizeToken(command.Kind)
	command.ScopeType = normalizeToken(command.ScopeType)
	command.ScopeID = strings.TrimSpace(command.ScopeID)
	command.Producer = normalizeToken(command.Producer)
	command.ProducerVersion = strings.TrimSpace(command.ProducerVersion)
	command.CreatedBy = strings.TrimSpace(command.CreatedBy)
	command.Classification = normalizeClassification(command.Classification)
	command.RetentionClass = normalizeRetention(command.RetentionClass)
	command.AsOf = command.AsOf.UTC()
	command.ExpiresAt = normalizeTimePointer(command.ExpiresAt)
	if command.DedupeKey == "" || command.Kind == "" || command.ScopeType == "" || command.ScopeID == "" ||
		command.AsOf.IsZero() || command.Producer == "" || command.ProducerVersion == "" || command.CreatedBy == "" {
		return SealSnapshotCommand{}, nil, fmt.Errorf("%w: snapshot identity, scope, as-of, producer version, and creator are required", ErrInvalidCommand)
	}
	if !validClassification(command.Classification) || !validRetention(command.RetentionClass) {
		return SealSnapshotCommand{}, nil, fmt.Errorf("%w: invalid snapshot classification or retention", ErrInvalidCommand)
	}
	if command.RetentionClass == RetentionLegalHold && command.ExpiresAt != nil {
		return SealSnapshotCommand{}, nil, fmt.Errorf("%w: legal-hold snapshots cannot expire", ErrInvalidCommand)
	}
	evidenceIDs := uniqueEvidenceIDs(command.EvidenceEventIDs)
	if len(evidenceIDs) > module.maxEvidence {
		return SealSnapshotCommand{}, nil, fmt.Errorf("%w: got %d evidence events", ErrEvidenceLimit, len(evidenceIDs))
	}
	return command, evidenceIDs, nil
}

func applyTimelineFilters(conn *gorm.DB, query TimelineQuery) *gorm.DB {
	if query.ProjectKey != "" {
		conn = conn.Where("project_key = ?", query.ProjectKey)
	}
	if query.SubjectID != "" {
		conn = conn.Where("subject_type = ? AND subject_id = ?", query.SubjectType, query.SubjectID)
	}
	if len(query.EventTypes) > 0 {
		conn = conn.Where("event_type IN ?", query.EventTypes)
	}
	if query.SourceSystem != "" {
		conn = conn.Where("source_system = ?", query.SourceSystem)
	}
	if query.SourceRecordID != "" {
		conn = conn.Where("source_record_id = ?", query.SourceRecordID)
	}
	if query.CorrelationID != "" {
		conn = conn.Where("correlation_id = ?", query.CorrelationID)
	}
	if query.Since != nil {
		conn = conn.Where("occurred_at >= ?", *query.Since)
	}
	if query.Until != nil {
		conn = conn.Where("occurred_at <= ?", *query.Until)
	}
	return conn
}

func validateEvidence(conn *gorm.DB, evidenceIDs []uint, highWatermark uint, asOf time.Time) error {
	var watermark struct{ MaxID uint }
	if err := conn.Model(&db.DataAssetEvent{}).Select("COALESCE(MAX(id), 0) AS max_id").Scan(&watermark).Error; err != nil {
		return err
	}
	if highWatermark > watermark.MaxID {
		return fmt.Errorf("%w: got %d, ledger maximum is %d", ErrInputWatermark, highWatermark, watermark.MaxID)
	}
	for _, eventID := range evidenceIDs {
		if eventID > highWatermark {
			return fmt.Errorf("%w: event %d is above input high watermark %d", ErrEvidenceMissing, eventID, highWatermark)
		}
	}
	for start := 0; start < len(evidenceIDs); start += evidenceBatchSize {
		end := start + evidenceBatchSize
		if end > len(evidenceIDs) {
			end = len(evidenceIDs)
		}
		var rows []struct {
			ID         uint
			OccurredAt time.Time
		}
		if err := conn.Model(&db.DataAssetEvent{}).
			Select("id, occurred_at").Where("id IN ?", evidenceIDs[start:end]).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) != end-start {
			return ErrEvidenceMissing
		}
		for _, row := range rows {
			if row.OccurredAt.After(asOf) {
				return fmt.Errorf("%w: event %d occurred at %s after %s", ErrEvidenceAfterAsOf, row.ID, row.OccurredAt.UTC().Format(time.RFC3339Nano), asOf.UTC().Format(time.RFC3339Nano))
			}
		}
	}
	return nil
}

type preparedAppend struct {
	command     AppendCommand
	encoded     encodedPayload
	fingerprint string
}

func (module *Module) prepareAppend(command AppendCommand) (preparedAppend, error) {
	observedAtDefaulted := command.ObservedAt.IsZero()
	normalized, err := module.normalizeAppend(command)
	if err != nil {
		return preparedAppend{}, err
	}
	encoded, err := encodePayload(normalized.Payload, module.compressionThreshold, module.maxPayloadBytes)
	if err != nil {
		return preparedAppend{}, err
	}
	fingerprintCommand := normalized
	if observedAtDefaulted {
		// The first append still records the actual observation clock, while the
		// idempotency identity remains stable when a caller retries without one.
		fingerprintCommand.ObservedAt = time.Time{}
	}
	fingerprint, err := appendFingerprint(fingerprintCommand, encoded.hash)
	if err != nil {
		return preparedAppend{}, err
	}
	return preparedAppend{command: normalized, encoded: encoded, fingerprint: fingerprint}, nil
}

func appendFingerprint(command AppendCommand, payloadHash string) (string, error) {
	return hashJSON(struct {
		DedupeKey         string     `json:"dedupe_key"`
		ProjectKey        string     `json:"project_key"`
		SubjectType       string     `json:"subject_type"`
		SubjectID         string     `json:"subject_id"`
		EventType         string     `json:"event_type"`
		SourceSystem      string     `json:"source_system"`
		SourceRecordID    string     `json:"source_record_id"`
		SourceEventID     string     `json:"source_event_id"`
		ActorID           string     `json:"actor_id"`
		ActorRole         string     `json:"actor_role"`
		CorrelationID     string     `json:"correlation_id"`
		CausationID       string     `json:"causation_id"`
		SupersedesEventID uint       `json:"supersedes_event_id"`
		Classification    string     `json:"classification"`
		RetentionClass    string     `json:"retention_class"`
		SchemaVersion     int        `json:"schema_version"`
		OccurredAt        time.Time  `json:"occurred_at"`
		ObservedAt        time.Time  `json:"observed_at"`
		ExpiresAt         *time.Time `json:"expires_at"`
		PayloadHash       string     `json:"payload_hash"`
	}{
		DedupeKey: command.DedupeKey, ProjectKey: command.ProjectKey,
		SubjectType: command.SubjectType, SubjectID: command.SubjectID, EventType: command.EventType,
		SourceSystem: command.SourceSystem, SourceRecordID: command.SourceRecordID, SourceEventID: command.SourceEventID,
		ActorID: command.ActorID, ActorRole: command.ActorRole, CorrelationID: command.CorrelationID,
		CausationID: command.CausationID, SupersedesEventID: command.SupersedesEventID,
		Classification: command.Classification, RetentionClass: command.RetentionClass,
		SchemaVersion: command.SchemaVersion, OccurredAt: command.OccurredAt, ObservedAt: command.ObservedAt,
		ExpiresAt: command.ExpiresAt, PayloadHash: payloadHash,
	})
}

func snapshotFingerprint(command SealSnapshotCommand, evidenceHash, payloadHash string) (string, error) {
	return hashJSON(struct {
		DedupeKey          string     `json:"dedupe_key"`
		Kind               string     `json:"kind"`
		ScopeType          string     `json:"scope_type"`
		ScopeID            string     `json:"scope_id"`
		AsOf               time.Time  `json:"as_of"`
		InputHighWatermark uint       `json:"input_high_watermark"`
		Producer           string     `json:"producer"`
		ProducerVersion    string     `json:"producer_version"`
		CreatedBy          string     `json:"created_by"`
		Classification     string     `json:"classification"`
		RetentionClass     string     `json:"retention_class"`
		ExpiresAt          *time.Time `json:"expires_at"`
		EvidenceHash       string     `json:"evidence_hash"`
		PayloadHash        string     `json:"payload_hash"`
	}{
		DedupeKey: command.DedupeKey, Kind: command.Kind, ScopeType: command.ScopeType, ScopeID: command.ScopeID,
		AsOf: command.AsOf, InputHighWatermark: command.InputHighWatermark, Producer: command.Producer,
		ProducerVersion: command.ProducerVersion, CreatedBy: command.CreatedBy,
		Classification: command.Classification, RetentionClass: command.RetentionClass,
		ExpiresAt: command.ExpiresAt, EvidenceHash: evidenceHash, PayloadHash: payloadHash,
	})
}

type timelineCursor struct {
	Version       int       `json:"v"`
	OccurredAt    time.Time `json:"occurred_at"`
	ID            uint      `json:"id"`
	HighWatermark uint      `json:"high_watermark"`
	FilterHash    string    `json:"filter_hash"`
}

func encodeTimelineCursor(cursor timelineCursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func decodeTimelineCursor(value string) (timelineCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return timelineCursor{}, ErrCursorInvalid
	}
	var cursor timelineCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return timelineCursor{}, ErrCursorInvalid
	}
	if cursor.Version != 1 || cursor.ID == 0 || cursor.HighWatermark == 0 || cursor.OccurredAt.IsZero() || cursor.FilterHash == "" {
		return timelineCursor{}, ErrCursorInvalid
	}
	cursor.OccurredAt = cursor.OccurredAt.UTC()
	return cursor, nil
}

func timelineFilterHash(query TimelineQuery) (string, error) {
	return hashJSON(struct {
		ProjectKey     string     `json:"project_key"`
		SubjectType    string     `json:"subject_type"`
		SubjectID      string     `json:"subject_id"`
		EventTypes     []string   `json:"event_types"`
		SourceSystem   string     `json:"source_system"`
		SourceRecordID string     `json:"source_record_id"`
		CorrelationID  string     `json:"correlation_id"`
		Since          *time.Time `json:"since"`
		Until          *time.Time `json:"until"`
	}{
		ProjectKey: query.ProjectKey, SubjectType: query.SubjectType, SubjectID: query.SubjectID,
		EventTypes: query.EventTypes, SourceSystem: query.SourceSystem, SourceRecordID: query.SourceRecordID,
		CorrelationID: query.CorrelationID, Since: query.Since, Until: query.Until,
	})
}

func eventFromRow(row db.DataAssetEvent) Event {
	return Event{
		ID: row.ID, ProjectKey: row.ProjectKey, SubjectType: row.SubjectType, SubjectID: row.SubjectID,
		EventType: row.EventType, SourceSystem: row.SourceSystem, SourceRecordID: row.SourceRecordID,
		SourceEventID: row.SourceEventID, ActorID: row.ActorID, ActorRole: row.ActorRole,
		CorrelationID: row.CorrelationID, CausationID: row.CausationID,
		SupersedesEventID: row.SupersedesEventID, Classification: row.Classification,
		RetentionClass: row.RetentionClass, SchemaVersion: row.SchemaVersion,
		OccurredAt: row.OccurredAt, ObservedAt: row.ObservedAt, RecordedAt: row.RecordedAt,
		ExpiresAt: row.ExpiresAt, PayloadHash: row.PayloadHash, PayloadEncoding: row.PayloadEncoding,
		PayloadBytes: row.PayloadBytes, StoredBytes: row.StoredBytes,
	}
}

func snapshotFromRow(row db.DataAssetSnapshot) Snapshot {
	return Snapshot{
		ID: row.ID, Kind: row.Kind, ScopeType: row.ScopeType, ScopeID: row.ScopeID, AsOf: row.AsOf,
		InputHighWatermark: row.InputHighWatermark, Producer: row.Producer, ProducerVersion: row.ProducerVersion,
		CreatedBy: row.CreatedBy, Classification: row.Classification, RetentionClass: row.RetentionClass,
		ExpiresAt: row.ExpiresAt, EvidenceCount: row.EvidenceCount, EvidenceHash: row.EvidenceHash,
		PayloadHash: row.PayloadHash, PayloadEncoding: row.PayloadEncoding, PayloadBytes: row.PayloadBytes,
		StoredBytes: row.StoredBytes, CreatedAt: row.CreatedAt,
	}
}

func normalizeClassification(value string) string {
	value = normalizeToken(value)
	if value == "" {
		return ClassificationInternal
	}
	return value
}

func normalizeRetention(value string) string {
	value = normalizeToken(value)
	if value == "" {
		return RetentionStandard
	}
	return value
}

func validClassification(value string) bool {
	switch value {
	case ClassificationPublic, ClassificationInternal, ClassificationConfidential, ClassificationRestricted:
		return true
	default:
		return false
	}
}

func validRetention(value string) bool {
	switch value {
	case RetentionEphemeral, RetentionStandard, RetentionLongTerm, RetentionLegalHold:
		return true
	default:
		return false
	}
}

func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func normalizeTokens(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeToken(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func normalizeTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func uniqueEvidenceIDs(values []uint) []uint {
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
