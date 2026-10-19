package db

import "time"

// RuntimeKnowledgeState is the CAS publication pointer and scheduler configuration.
// Source records remain authoritative; generations are disposable projections.
type RuntimeKnowledgeState struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Enabled         bool       `json:"enabled"`
	IntervalMinutes int        `json:"interval_minutes"`
	Version         int        `json:"version"`
	GenerationID    uint       `json:"generation_id"`
	LastCheckedAt   *time.Time `json:"last_checked_at"`
	LastError       string     `json:"last_error,omitempty"`
}

type RuntimeKnowledgeGeneration struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ContentHash string    `gorm:"size:64" json:"content_hash"`
	ItemCount   int       `json:"item_count"`
	GapsJSON    string    `gorm:"type:text" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

type RuntimeKnowledgeItem struct {
	ID              uint      `gorm:"primaryKey" json:"-"`
	GenerationID    uint      `gorm:"index:idx_runtime_knowledge_scope,priority:1;index" json:"-"`
	Scope           string    `gorm:"index:idx_runtime_knowledge_scope,priority:2;size:32" json:"scope"`
	ScopeID         string    `gorm:"index:idx_runtime_knowledge_scope,priority:3;size:160" json:"scope_id"`
	Kind            string    `gorm:"size:32;index" json:"kind"`
	SourceID        string    `gorm:"size:160" json:"-"`
	SourceRef       string    `gorm:"size:255" json:"source_ref"`
	Fingerprint     string    `gorm:"size:64" json:"-"`
	ContentHash     string    `gorm:"size:64" json:"content_hash"`
	Title           string    `gorm:"type:text" json:"title"`
	Content         string    `gorm:"type:text" json:"content"`
	Claim           string    `gorm:"size:64" json:"claim"`
	SourceUpdatedAt time.Time `json:"source_updated_at"`
	Truncated       bool      `json:"source_truncated"`
	ChunkIndex      int       `json:"chunk_index"`
	ChunkCount      int       `json:"chunk_count"`
	FullContent     string    `gorm:"-" json:"-"`
}
