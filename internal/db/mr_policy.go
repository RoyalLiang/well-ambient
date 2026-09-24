package db

import "time"

// MRPolicyRun records a decision and the at-most-once notification attempt.
type MRPolicyRun struct {
	Key       string `gorm:"primaryKey;size:64"`
	Project   string
	MRIID     int
	HeadSHA   string
	Status    string
	Reason    string
	CreatedAt time.Time
	UpdatedAt time.Time
}
