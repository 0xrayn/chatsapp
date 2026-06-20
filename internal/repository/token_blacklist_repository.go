package repository

import (
	"time"

	"chatapp/internal/domain"

	"gorm.io/gorm"
)

type tokenBlacklistRepository struct {
	db *gorm.DB
}

func NewTokenBlacklistRepository(db *gorm.DB) domain.TokenBlacklistRepository {
	return &tokenBlacklistRepository{db: db}
}

func (r *tokenBlacklistRepository) Add(jti string, expiresAt time.Time) error {
	entry := &domain.TokenBlacklist{
		JTI:       jti,
		ExpiresAt: expiresAt,
	}
	return r.db.Create(entry).Error
}

func (r *tokenBlacklistRepository) IsRevoked(jti string) (bool, error) {
	var count int64
	err := r.db.Model(&domain.TokenBlacklist{}).
		Where("jti = ? AND expires_at > ?", jti, time.Now()).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteExpired removes tokens that have already passed their natural expiry.
// Called periodically by a background goroutine in main.go.
func (r *tokenBlacklistRepository) DeleteExpired() error {
	return r.db.Where("expires_at <= ?", time.Now()).
		Delete(&domain.TokenBlacklist{}).Error
}
