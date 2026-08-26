package db

import "time"

// SolutionCatalogEntry is the permission-filterable read model for the latest
// published revision of one demand solution. The Markdown remains in
// SolutionRevision so the catalog does not duplicate the largest payload.
type SolutionCatalogEntry struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	SolutionAssetID     uint      `gorm:"uniqueIndex;not null;column:solution_asset_id" json:"solution_asset_id"`
	PublishedRevisionID uint      `gorm:"uniqueIndex;not null;column:published_revision_id" json:"published_revision_id"`
	DemandID            string    `gorm:"index;size:160;not null;column:demand_id" json:"demand_id"`
	ProjectKey          string    `gorm:"index:idx_solution_catalog_project_published,priority:1;index;size:64;not null;column:project_key" json:"project_key"`
	DemandTitle         string    `gorm:"size:512;column:demand_title" json:"demand_title"`
	SolutionTitle       string    `gorm:"size:512;column:solution_title" json:"solution_title"`
	Summary             string    `gorm:"type:text" json:"summary"`
	ContentHash         string    `gorm:"index;size:64;not null;column:content_hash" json:"content_hash"`
	ContentBytes        int       `gorm:"not null;column:content_bytes" json:"content_bytes"`
	StoredBytes         int       `gorm:"not null;column:stored_bytes" json:"stored_bytes"`
	SearchTokenCount    int       `gorm:"not null;default:0;column:search_token_count" json:"search_token_count"`
	PublishedAt         time.Time `gorm:"index:idx_solution_catalog_project_published,priority:2;index;not null;column:published_at" json:"published_at"`
	SyncedAt            time.Time `gorm:"index;not null;column:synced_at" json:"synced_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// SolutionCatalogSearchToken is an indexed inverted search row. It keeps
// catalog search and similarity recall index-backed without duplicating full
// Markdown or depending on a SQLite-specific tokenizer.
type SolutionCatalogSearchToken struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	EntryID uint   `gorm:"uniqueIndex:idx_solution_catalog_entry_token,priority:1;index;not null;column:entry_id" json:"entry_id"`
	Token   string `gorm:"uniqueIndex:idx_solution_catalog_entry_token,priority:2;index:idx_solution_catalog_token_entry,priority:1;size:96;not null" json:"token"`
}

// SolutionCatalogSyncJob is the transactional hand-off between publishing and
// the catalog projection. A periodic reconciler recreates a missing job.
type SolutionCatalogSyncJob struct {
	ID                  uint       `gorm:"primaryKey" json:"id"`
	SolutionAssetID     uint       `gorm:"index;not null;column:solution_asset_id" json:"solution_asset_id"`
	PublishedRevisionID uint       `gorm:"uniqueIndex;not null;column:published_revision_id" json:"published_revision_id"`
	Status              string     `gorm:"index;size:32;not null" json:"status"`
	AttemptCount        int        `gorm:"not null;default:0;column:attempt_count" json:"attempt_count"`
	LastError           string     `gorm:"type:text;column:last_error" json:"last_error"`
	NextAttemptAt       *time.Time `gorm:"index;column:next_attempt_at" json:"next_attempt_at"`
	StartedAt           *time.Time `json:"started_at"`
	CompletedAt         *time.Time `json:"completed_at"`
	CreatedAt           time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// SolutionComparison is the append-only, revision-bound two-round comparison
// record. Model output is compressed and integrity-checked like solution text.
type SolutionComparison struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	PairKey               string     `gorm:"uniqueIndex;size:160;not null;column:pair_key" json:"pair_key"`
	LeftEntryID           uint       `gorm:"index;not null;column:left_entry_id" json:"left_entry_id"`
	RightEntryID          uint       `gorm:"index;not null;column:right_entry_id" json:"right_entry_id"`
	LeftRevisionID        uint       `gorm:"index;not null;column:left_revision_id" json:"left_revision_id"`
	RightRevisionID       uint       `gorm:"index;not null;column:right_revision_id" json:"right_revision_id"`
	RecallScore           float64    `gorm:"index;not null;column:recall_score" json:"recall_score"`
	Status                string     `gorm:"index;size:32;not null" json:"status"`
	Stage                 string     `gorm:"index;size:32;not null" json:"stage"`
	RoundOnePromptVersion string     `gorm:"size:96;column:round_one_prompt_version" json:"round_one_prompt_version"`
	RoundOneVerdict       string     `gorm:"index;size:32;column:round_one_verdict" json:"round_one_verdict"`
	RoundOneScore         float64    `gorm:"column:round_one_score" json:"round_one_score"`
	RoundOnePayload       []byte     `gorm:"column:round_one_payload" json:"-"`
	RoundOneEncoding      string     `gorm:"size:16;column:round_one_encoding" json:"round_one_encoding"`
	RoundOneHash          string     `gorm:"size:64;column:round_one_hash" json:"round_one_hash"`
	RoundOneBytes         int        `gorm:"column:round_one_bytes" json:"round_one_bytes"`
	RoundOneStoredBytes   int        `gorm:"column:round_one_stored_bytes" json:"round_one_stored_bytes"`
	RoundTwoPromptVersion string     `gorm:"size:96;column:round_two_prompt_version" json:"round_two_prompt_version"`
	RoundTwoVerdict       string     `gorm:"index;size:32;column:round_two_verdict" json:"round_two_verdict"`
	RoundTwoScore         float64    `gorm:"column:round_two_score" json:"round_two_score"`
	RoundTwoPayload       []byte     `gorm:"column:round_two_payload" json:"-"`
	RoundTwoEncoding      string     `gorm:"size:16;column:round_two_encoding" json:"round_two_encoding"`
	RoundTwoHash          string     `gorm:"size:64;column:round_two_hash" json:"round_two_hash"`
	RoundTwoBytes         int        `gorm:"column:round_two_bytes" json:"round_two_bytes"`
	RoundTwoStoredBytes   int        `gorm:"column:round_two_stored_bytes" json:"round_two_stored_bytes"`
	AttemptCount          int        `gorm:"not null;default:0;column:attempt_count" json:"attempt_count"`
	LastError             string     `gorm:"type:text;column:last_error" json:"last_error"`
	NextAttemptAt         *time.Time `gorm:"index;column:next_attempt_at" json:"next_attempt_at"`
	StartedAt             *time.Time `json:"started_at"`
	CompletedAt           *time.Time `json:"completed_at"`
	CreatedAt             time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

// SolutionStandardizationProposal is a reviewable model suggestion. Accepting
// it creates an immutable standard revision; rejecting it preserves the audit.
type SolutionStandardizationProposal struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	ComparisonID    uint       `gorm:"uniqueIndex;not null;column:comparison_id" json:"comparison_id"`
	StandardID      uint       `gorm:"index;column:standard_id" json:"standard_id"`
	Status          string     `gorm:"index;size:32;not null" json:"status"`
	Title           string     `gorm:"size:512" json:"title"`
	Summary         string     `gorm:"type:text" json:"summary"`
	Content         []byte     `gorm:"not null" json:"-"`
	ContentEncoding string     `gorm:"size:16;not null;column:content_encoding" json:"content_encoding"`
	ContentHash     string     `gorm:"index;size:64;not null;column:content_hash" json:"content_hash"`
	ContentBytes    int        `gorm:"not null;column:content_bytes" json:"content_bytes"`
	StoredBytes     int        `gorm:"not null;column:stored_bytes" json:"stored_bytes"`
	CreatedBy       string     `gorm:"size:160;not null;column:created_by" json:"created_by"`
	ReviewedBy      string     `gorm:"size:160;column:reviewed_by" json:"reviewed_by"`
	ReviewNote      string     `gorm:"type:text;column:review_note" json:"review_note"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	CreatedAt       time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// SolutionStandard is the stable identity for a governed reusable solution.
type SolutionStandard struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Status            string    `gorm:"index;size:32;not null" json:"status"`
	Sequence          int       `gorm:"not null;default:0" json:"sequence"`
	CurrentRevisionID uint      `gorm:"index;column:current_revision_id" json:"current_revision_id"`
	Title             string    `gorm:"size:512;not null" json:"title"`
	Summary           string    `gorm:"type:text" json:"summary"`
	CreatedBy         string    `gorm:"size:160;not null;column:created_by" json:"created_by"`
	CreatedAt         time.Time `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time `gorm:"index" json:"updated_at"`
}

// SolutionStandardRevision is immutable. A proposal can create a new standard
// or advance an existing one without rewriting earlier accepted content.
type SolutionStandardRevision struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	StandardID       uint      `gorm:"uniqueIndex:idx_solution_standard_version,priority:1;index;not null;column:standard_id" json:"standard_id"`
	Version          int       `gorm:"uniqueIndex:idx_solution_standard_version,priority:2;not null" json:"version"`
	ParentRevisionID uint      `gorm:"index;column:parent_revision_id" json:"parent_revision_id"`
	ProposalID       uint      `gorm:"uniqueIndex;not null;column:proposal_id" json:"proposal_id"`
	Content          []byte    `gorm:"not null" json:"-"`
	ContentEncoding  string    `gorm:"size:16;not null;column:content_encoding" json:"content_encoding"`
	ContentHash      string    `gorm:"index;size:64;not null;column:content_hash" json:"content_hash"`
	ContentBytes     int       `gorm:"not null;column:content_bytes" json:"content_bytes"`
	StoredBytes      int       `gorm:"not null;column:stored_bytes" json:"stored_bytes"`
	CreatedBy        string    `gorm:"size:160;not null;column:created_by" json:"created_by"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
}
