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
)

func TestRoomService_CreateRoom(t *testing.T) {
	t.Run("success public room", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		svc := service.NewRoomService(roomRepo)
		creatorID := uuid.New()

		req := domain.CreateRoomRequest{
			Name:        "General",
			Description: "General chat",
			Type:        domain.RoomTypePublic,
		}

		createdRoom := &domain.Room{
			ID:        uuid.New(),
			Name:      req.Name,
			CreatedBy: creatorID,
			Type:      domain.RoomTypePublic,
		}

		roomRepo.On("Create", mock.MatchedByRoom(req.Name)).Return(nil)
		roomRepo.On("AddMember", mock.AnyRoomMember()).Return(nil)
		roomRepo.On("FindByID", mock.AnythingUUID()).Return(createdRoom, nil)

		room, err := svc.CreateRoom(req, creatorID)

		require.NoError(t, err)
		assert.Equal(t, req.Name, room.Name)
		assert.Equal(t, creatorID, room.CreatedBy)
		roomRepo.AssertExpectations(t)
	})

	t.Run("db error", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		svc := service.NewRoomService(roomRepo)

		roomRepo.On("Create", mock.MatchedByRoom("TestRoom")).Return(errors.New("db error"))

		_, err := svc.CreateRoom(domain.CreateRoomRequest{Name: "TestRoom"}, uuid.New())

		assert.EqualError(t, err, "failed to create room")
		roomRepo.AssertExpectations(t)
	})
}

func TestRoomService_JoinRoom(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		svc := service.NewRoomService(roomRepo)
		roomID := uuid.New()
		userID := uuid.New()

		room := &domain.Room{ID: roomID, Type: domain.RoomTypePublic}
		roomRepo.On("FindByID", roomID).Return(room, nil)
		roomRepo.On("IsMember", roomID, userID).Return(false, nil)
		roomRepo.On("AddMember", mock.AnyRoomMember()).Return(nil)

		err := svc.JoinRoom(roomID, userID)
		require.NoError(t, err)
		roomRepo.AssertExpectations(t)
	})

	t.Run("already a member", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		svc := service.NewRoomService(roomRepo)
		roomID := uuid.New()
		userID := uuid.New()

		room := &domain.Room{ID: roomID, Type: domain.RoomTypePublic}
		roomRepo.On("FindByID", roomID).Return(room, nil)
		roomRepo.On("IsMember", roomID, userID).Return(true, nil)

		err := svc.JoinRoom(roomID, userID)
		assert.EqualError(t, err, "already a member")
		roomRepo.AssertExpectations(t)
	})

	t.Run("private room rejected", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		svc := service.NewRoomService(roomRepo)
		roomID := uuid.New()

		room := &domain.Room{ID: roomID, Type: domain.RoomTypePrivate}
		roomRepo.On("FindByID", roomID).Return(room, nil)

		err := svc.JoinRoom(roomID, uuid.New())
		assert.EqualError(t, err, "this is a private room, you need an invitation")
		roomRepo.AssertExpectations(t)
	})
}

func TestRoomService_DeleteRoom(t *testing.T) {
	t.Run("owner can delete", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		svc := service.NewRoomService(roomRepo)
		ownerID := uuid.New()
		roomID := uuid.New()

		room := &domain.Room{ID: roomID, CreatedBy: ownerID}
		roomRepo.On("FindByID", roomID).Return(room, nil)
		roomRepo.On("Delete", roomID).Return(nil)

		err := svc.DeleteRoom(roomID, ownerID)
		require.NoError(t, err)
		roomRepo.AssertExpectations(t)
	})

	t.Run("non-owner cannot delete", func(t *testing.T) {
		roomRepo := new(mock.RoomRepository)
		svc := service.NewRoomService(roomRepo)
		ownerID := uuid.New()
		otherUserID := uuid.New()
		roomID := uuid.New()

		room := &domain.Room{ID: roomID, CreatedBy: ownerID}
		roomRepo.On("FindByID", roomID).Return(room, nil)

		err := svc.DeleteRoom(roomID, otherUserID)
		assert.EqualError(t, err, "only room creator can delete this room")
		roomRepo.AssertExpectations(t)
	})
}
