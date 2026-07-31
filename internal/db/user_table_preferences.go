package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserTablePreference stores a compact JSON column list for one user and table.
type UserTablePreference struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Username    string    `gorm:"uniqueIndex:idx_user_table_preference;index;size:160;not null" json:"username"`
	TableKey    string    `gorm:"uniqueIndex:idx_user_table_preference;index;size:96;not null" json:"table_key"`
	ColumnsJSON string    `gorm:"type:text;not null" json:"columns_json"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func normalizeTableColumns(columns []string) []string {
	seen := make(map[string]struct{}, len(columns))
	normalized := make([]string, 0, len(columns))
	for _, column := range columns {
		column = strings.TrimSpace(column)
		if column == "" {
			continue
		}
		if _, exists := seen[column]; exists {
			continue
		}
		seen[column] = struct{}{}
		normalized = append(normalized, column)
	}
	return normalized
}

func LoadUserTablePreference(conn *gorm.DB, username, tableKey string) ([]string, bool, error) {
	username = strings.TrimSpace(username)
	tableKey = strings.TrimSpace(tableKey)
	if conn == nil || username == "" || tableKey == "" {
		return nil, false, nil
	}
	if !conn.Migrator().HasTable(&UserTablePreference{}) {
		return nil, false, nil
	}

	var preference UserTablePreference
	if err := conn.Where("username = ? AND table_key = ?", username, tableKey).First(&preference).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	var columns []string
	if err := json.Unmarshal([]byte(preference.ColumnsJSON), &columns); err != nil {
		return nil, false, fmt.Errorf("decode table preference: %w", err)
	}
	return normalizeTableColumns(columns), true, nil
}

func SaveUserTablePreference(conn *gorm.DB, username, tableKey string, columns []string) error {
	username = strings.TrimSpace(username)
	tableKey = strings.TrimSpace(tableKey)
	if conn == nil {
		return fmt.Errorf("database is not initialized")
	}
	if username == "" || tableKey == "" {
		return fmt.Errorf("username and table key are required")
	}
	columnsJSON, err := json.Marshal(normalizeTableColumns(columns))
	if err != nil {
		return err
	}
	preference := UserTablePreference{
		Username:    username,
		TableKey:    tableKey,
		ColumnsJSON: string(columnsJSON),
		UpdatedAt:   time.Now(),
	}
	return conn.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "username"}, {Name: "table_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"columns_json", "updated_at"}),
	}).Create(&preference).Error
}
