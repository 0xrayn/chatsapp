package service_test

import (
	"errors"
	"testing"

	"chatapp/internal/domain"
	"chatapp/internal/service"
	"chatapp/test/mock"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestAuthService_Register(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := new(mock.UserRepository)
		svc := service.NewAuthService(userRepo)

		req := domain.RegisterRequest{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "password123",
		}

		userRepo.On("FindByEmail", req.Email).Return(nil, gorm.ErrRecordNotFound)
		userRepo.On("FindByUsername", req.Username).Return(nil, gorm.ErrRecordNotFound)
		userRepo.On("Create", mock.AnyUser()).Return(nil)

		resp, err := svc.Register(req)

		require.NoError(t, err)
		assert.NotEmpty(t, resp.Token)
		assert.Equal(t, req.Username, resp.User.Username)
		assert.Equal(t, req.Email, resp.User.Email)
		assert.NotEmpty(t, resp.User.Password) // hashed password set on struct; hidden via json:"-" tag
		userRepo.AssertExpectations(t)
	})

	t.Run("duplicate email", func(t *testing.T) {
		userRepo := new(mock.UserRepository)
		svc := service.NewAuthService(userRepo)

		existingUser := &domain.User{ID: uuid.New(), Email: "test@example.com"}
		userRepo.On("FindByEmail", "test@example.com").Return(existingUser, nil)

		_, err := svc.Register(domain.RegisterRequest{
			Username: "newuser",
			Email:    "test@example.com",
			Password: "password123",
		})

		assert.EqualError(t, err, "email already registered")
		userRepo.AssertExpectations(t)
	})

	t.Run("duplicate username", func(t *testing.T) {
		userRepo := new(mock.UserRepository)
		svc := service.NewAuthService(userRepo)

		userRepo.On("FindByEmail", "new@example.com").Return(nil, gorm.ErrRecordNotFound)
		existingUser := &domain.User{ID: uuid.New(), Username: "takenuser"}
		userRepo.On("FindByUsername", "takenuser").Return(existingUser, nil)

		_, err := svc.Register(domain.RegisterRequest{
			Username: "takenuser",
			Email:    "new@example.com",
			Password: "password123",
		})

		assert.EqualError(t, err, "username already taken")
		userRepo.AssertExpectations(t)
	})

	t.Run("db error on create", func(t *testing.T) {
		userRepo := new(mock.UserRepository)
		svc := service.NewAuthService(userRepo)

		userRepo.On("FindByEmail", "test@example.com").Return(nil, gorm.ErrRecordNotFound)
		userRepo.On("FindByUsername", "testuser").Return(nil, gorm.ErrRecordNotFound)
		userRepo.On("Create", mock.AnyUser()).Return(errors.New("db error"))

		_, err := svc.Register(domain.RegisterRequest{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "password123",
		})

		assert.EqualError(t, err, "failed to create user")
		userRepo.AssertExpectations(t)
	})
}

func TestAuthService_Login(t *testing.T) {
	hashedPw, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)

	existingUser := &domain.User{
		ID:       uuid.New(),
		Username: "testuser",
		Email:    "test@example.com",
		Password: string(hashedPw),
	}

	t.Run("success", func(t *testing.T) {
		userRepo := new(mock.UserRepository)
		svc := service.NewAuthService(userRepo)

		userRepo.On("FindByEmail", "test@example.com").Return(existingUser, nil)
		userRepo.On("SetOnlineStatus", existingUser.ID, true).Return(nil)

		resp, err := svc.Login(domain.LoginRequest{
			Email:    "test@example.com",
			Password: "correctpassword",
		})

		require.NoError(t, err)
		assert.NotEmpty(t, resp.Token)
		assert.Equal(t, existingUser.ID, resp.User.ID)
		userRepo.AssertExpectations(t)
	})

	t.Run("wrong password", func(t *testing.T) {
		userRepo := new(mock.UserRepository)
		svc := service.NewAuthService(userRepo)

		userRepo.On("FindByEmail", "test@example.com").Return(existingUser, nil)

		_, err := svc.Login(domain.LoginRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		})

		assert.EqualError(t, err, "invalid email or password")
		userRepo.AssertExpectations(t)
	})

	t.Run("user not found", func(t *testing.T) {
		userRepo := new(mock.UserRepository)
		svc := service.NewAuthService(userRepo)

		userRepo.On("FindByEmail", "notfound@example.com").Return(nil, gorm.ErrRecordNotFound)

		_, err := svc.Login(domain.LoginRequest{
			Email:    "notfound@example.com",
			Password: "password",
		})

		assert.EqualError(t, err, "invalid email or password")
		userRepo.AssertExpectations(t)
	})
}
