package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
	"well-ambient/internal/db"
)

// Standard AgentRun event types
const (
	EventIntentCompiled     = "intent_compiled"
	EventCapabilitySelected = "capability_selected"
	EventCapabilityRejected = "capability_rejected"
	EventResourceLoaded     = "resource_loaded"
	EventToolCalled         = "tool_called"
	EventContextExpanded    = "context_expanded"
	EventModelCalled        = "model_called"
	EventValidationFailed   = "validation_failed"
	EventCompleted          = "completed"
	EventRolledBack         = "rolled_back"
)

// TraceCollector provides thread-safe append-only trace logging for AgentRuns.
type TraceCollector struct {
	db *gorm.DB
	mu sync.Mutex
}

// NewTraceCollector initializes a trace collector.
func NewTraceCollector(database *gorm.DB) *TraceCollector {
	return &TraceCollector{db: database}
}

// RecordEvent appends an audit event to the AgentRun trace log.
func (tc *TraceCollector) RecordEvent(ctx context.Context, runID uint, eventType string, payload any) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	var count int64
	tc.db.WithContext(ctx).Model(&db.AgentRunEvent{}).Where("run_id = ?", runID).Count(&count)

	var payloadJSON string
	if payload != nil {
		if str, ok := payload.(string); ok {
			payloadJSON = str
		} else {
			bytes, err := json.Marshal(payload)
			if err == nil {
				payloadJSON = string(bytes)
			}
		}
	}

	event := db.AgentRunEvent{
		RunID:       runID,
		Sequence:    int(count + 1),
		EventType:   eventType,
		PayloadJSON: payloadJSON,
		CreatedAt:   time.Now(),
	}

	if err := tc.db.WithContext(ctx).Create(&event).Error; err != nil {
		return fmt.Errorf("failed to record trace event: %w", err)
	}
	return nil
}

// GetRunEvents retrieves all chronological trace events for a given run ID.
func (tc *TraceCollector) GetRunEvents(ctx context.Context, runID uint) ([]db.AgentRunEvent, error) {
	var events []db.AgentRunEvent
	err := tc.db.WithContext(ctx).
		Where("run_id = ?", runID).
		Order("sequence ASC").
		Find(&events).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get trace events: %w", err)
	}
	return events, nil
}
