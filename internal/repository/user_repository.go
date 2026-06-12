package repository

import (
	"chatapp/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(user *domain.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) FindByID(id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(email string) (*domain.User, error) {
	var user domain.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByUsername(username string) (*domain.User, error) {
	var user domain.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) Update(user *domain.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) SetOnlineStatus(id uuid.UUID, isOnline bool) error {
	return r.db.Model(&domain.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_online": isOnline,
			"last_seen": gorm.Expr("NOW()"),
		}).Error
}

func (r *userRepository) GetOnlineUsers() ([]domain.User, error) {
	var users []domain.User
	err := r.db.Where("is_online = ?", true).Find(&users).Error
	return users, err
}

func (r *userRepository) SearchUsers(query string, excludeID uuid.UUID, limit int) ([]domain.User, error) {
	var users []domain.User
	err := r.db.
		Where("username ILIKE ? AND id != ?", "%"+query+"%", excludeID).
		Limit(limit).
		Find(&users).Error
	return users, err
}
