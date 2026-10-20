package db

import "time"

// CapabilityInstallation is evidence of an explicit server-side installation.
// Built-in registration and client downloads never create these receipts.
type CapabilityInstallation struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	CapabilityKey       string    `gorm:"index;size:128;not null" json:"capability_key"`
	CapabilityVersionID uint      `gorm:"uniqueIndex;not null" json:"capability_version_id"`
	Version             int       `json:"version"`
	Method              string    `gorm:"size:32;not null" json:"method"`
	Source              string    `gorm:"size:1024" json:"source"`
	Location            string    `gorm:"size:256;not null" json:"location"`
	Digest              string    `gorm:"size:128;not null" json:"digest"`
	ArtifactSHA256      string    `gorm:"size:64;not null" json:"artifact_sha256"`
	EvidenceSource      string    `gorm:"size:128;not null" json:"evidence_source"`
	InstalledAt         time.Time `json:"installed_at"`
}
