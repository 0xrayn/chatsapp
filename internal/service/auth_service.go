package service

import (
	"errors"
	"fmt"
	"time"

	"chatapp/internal/domain"
	"chatapp/internal/middleware"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	userRepo domain.UserRepository
}

func NewAuthService(userRepo domain.UserRepository) *AuthService {
	return &AuthService{userRepo: userRepo}
}

func (s *AuthService) Register(req domain.RegisterRequest) (*domain.AuthResponse, error) {
	// Manual validation: username format & password strength
	if !middleware.ValidateUsername(req.Username) {
		return nil, errors.New("username must be 3-30 characters and contain only letters, numbers, and underscores")
	}
	if !middleware.ValidatePasswordStrength(req.Password) {
		return nil, errors.New("password must contain at least one letter and one number")
	}

	// Check if email exists
	if _, err := s.userRepo.FindByEmail(req.Email); err == nil {
		return nil, errors.New("email already registered")
	}

	// Check if username exists
	if _, err := s.userRepo.FindByUsername(req.Username); err == nil {
		return nil, errors.New("username already taken")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	user := &domain.User{
		ID:       uuid.New(),
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashedPassword),
		LastSeen: time.Now(),
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, errors.New("failed to create user")
	}

	token, err := middleware.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &domain.AuthResponse{Token: token, User: *user}, nil
}

func (s *AuthService) Login(req domain.LoginRequest) (*domain.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(req.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid email or password")
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	token, err := middleware.GenerateToken(user.ID, user.Username)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	// Update online status
	s.userRepo.SetOnlineStatus(user.ID, true)

	return &domain.AuthResponse{Token: token, User: *user}, nil
}

func (s *AuthService) GetProfile(userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.FindByID(userID)
}

// SearchUsers finds users by username (excluding self), used for starting new DMs
func (s *AuthService) SearchUsers(query string, excludeID uuid.UUID) ([]domain.User, error) {
	if len(query) < 1 {
		return []domain.User{}, nil
	}
	return s.userRepo.SearchUsers(query, excludeID, 20)
}

// UpdateProfile updates the user's avatar and/or status message
func (s *AuthService) UpdateProfile(userID uuid.UUID, req domain.UpdateProfileRequest) (*domain.User, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}
	if req.Status != "" {
		user.Status = req.Status
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, errors.New("failed to update profile")
	}

	return user, nil
}

const UsernameCooldownHours = 6

func (s *AuthService) UpdateUsername(userID uuid.UUID, req domain.UpdateUsernameRequest) (*domain.User, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Enforce cooldown
	if user.UsernameChangedAt != nil {
		elapsed := time.Since(*user.UsernameChangedAt)
		if elapsed < time.Duration(UsernameCooldownHours)*time.Hour {
			remaining := time.Duration(UsernameCooldownHours)*time.Hour - elapsed
			hours := int(remaining.Hours())
			mins := int(remaining.Minutes()) % 60
			return nil, fmt.Errorf("username can only be changed every %d hours. Try again in %dh %dm", UsernameCooldownHours, hours, mins)
		}
	}

	if !middleware.ValidateUsername(req.Username) {
		return nil, errors.New("username must be 3-30 characters: letters, numbers, underscores only")
	}

	// Check uniqueness
	if existing, err := s.userRepo.FindByUsername(req.Username); err == nil && existing.ID != userID {
		return nil, errors.New("username already taken")
	}

	now := time.Now()
	user.Username = req.Username
	user.UsernameChangedAt = &now

	if err := s.userRepo.Update(user); err != nil {
		return nil, errors.New("failed to update username")
	}

	return user, nil
}

func (s *AuthService) UpdateEmail(userID uuid.UUID, req domain.UpdateEmailRequest) (*domain.User, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return nil, errors.New("incorrect current password")
	}

	if existing, err := s.userRepo.FindByEmail(req.Email); err == nil && existing.ID != userID {
		return nil, errors.New("email already in use")
	}

	user.Email = req.Email
	if err := s.userRepo.Update(user); err != nil {
		return nil, errors.New("failed to update email")
	}

	return user, nil
}

func (s *AuthService) UpdatePassword(userID uuid.UUID, req domain.UpdatePasswordRequest) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return errors.New("incorrect current password")
	}

	if !middleware.ValidatePasswordStrength(req.NewPassword) {
		return errors.New("new password must contain at least one letter and one number")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("failed to hash password")
	}

	user.Password = string(hashed)
	if err := s.userRepo.Update(user); err != nil {
		return errors.New("failed to update password")
	}

	return nil
}
