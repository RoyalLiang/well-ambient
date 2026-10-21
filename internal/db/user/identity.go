package user

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// ResolveIdentity accepts an exact username or a uniquely registered email.
// It never guesses a username by discarding an email's domain.
func ResolveIdentity(ctx context.Context, conn *gorm.DB, identity string) (User, error) {
	var result User
	if conn == nil {
		return result, gorm.ErrInvalidDB
	}
	identity = strings.TrimSpace(identity)
	err := conn.WithContext(ctx).Where("username = ?", identity).First(&result).Error
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) || !strings.Contains(identity, "@") {
		return result, err
	}
	var matches []User
	if err := conn.WithContext(ctx).Where("LOWER(email) = LOWER(?)", identity).Limit(2).Find(&matches).Error; err != nil {
		return result, err
	}
	if len(matches) != 1 {
		return result, gorm.ErrRecordNotFound
	}
	return matches[0], nil
}
