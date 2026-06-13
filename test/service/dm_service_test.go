package service_test

import (
	"errors"
	"testing"

	"chatapp/internal/domain"
	"chatapp/internal/service"
	ws "chatapp/internal/websocket"
	"chatapp/test/mock"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDMService_GetOrCreateDM(t *testing.T) {
	t.Run("rejects DM with self", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		userRepo := new(mock.UserRepository)
		msgRepo := new(mock.MessageRepository)
		svc := service.NewDMService(roomRepo, userRepo, msgRepo, ws.NewHub())

		userID := uuid.New()

		_, err := svc.GetOrCreateDM(userID, userID)

		assert.EqualError(t, err, "cannot create DM with yourself")
	})

	t.Run("returns existing DM room", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		userRepo := new(mock.UserRepository)
		msgRepo := new(mock.MessageRepository)
		svc := service.NewDMService(roomRepo, userRepo, msgRepo, ws.NewHub())

		senderID := uuid.New()
		recipientID := uuid.New()
		roomID := uuid.New()

		recipient := &domain.User{ID: recipientID, Username: "bob"}
		existingRoom := &domain.Room{ID: roomID, Type: domain.RoomTypeDirect}

		userRepo.On("FindByID", recipientID).Return(recipient, nil)
		roomRepo.On("FindDirectRoom", senderID, recipientID).Return(existingRoom, nil)

		room, err := svc.GetOrCreateDM(senderID, recipientID)

		require.NoError(t, err)
		assert.Equal(t, roomID, room.ID)
		roomRepo.AssertNotCalled(t, "Create")
	})

	t.Run("creates new DM room when none exists", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		userRepo := new(mock.UserRepository)
		msgRepo := new(mock.MessageRepository)
		svc := service.NewDMService(roomRepo, userRepo, msgRepo, ws.NewHub())

		senderID := uuid.New()
		recipientID := uuid.New()

		sender := &domain.User{ID: senderID, Username: "alice"}
		recipient := &domain.User{ID: recipientID, Username: "bob"}

		userRepo.On("FindByID", recipientID).Return(recipient, nil)
		roomRepo.On("FindDirectRoom", senderID, recipientID).Return(nil, errors.New("not found"))
		userRepo.On("FindByID", senderID).Return(sender, nil)
		roomRepo.On("Create", mock.AnythingRoomPtr()).Return(nil)
		roomRepo.On("AddMember", mock.AnyRoomMember()).Return(nil).Twice()

		createdRoom := &domain.Room{ID: uuid.New(), Type: domain.RoomTypeDirect}
		roomRepo.On("FindByID", mock.AnythingUUID()).Return(createdRoom, nil)

		room, err := svc.GetOrCreateDM(senderID, recipientID)

		require.NoError(t, err)
		assert.Equal(t, domain.RoomTypeDirect, room.Type)
		roomRepo.AssertExpectations(t)
	})

	t.Run("recipient not found", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		userRepo := new(mock.UserRepository)
		msgRepo := new(mock.MessageRepository)
		svc := service.NewDMService(roomRepo, userRepo, msgRepo, ws.NewHub())

		senderID := uuid.New()
		recipientID := uuid.New()

		userRepo.On("FindByID", recipientID).Return(nil, errors.New("not found"))

		_, err := svc.GetOrCreateDM(senderID, recipientID)

		assert.EqualError(t, err, "recipient user not found")
	})
}
