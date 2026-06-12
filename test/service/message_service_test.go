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

func TestMessageService_GetMessages(t *testing.T) {
	t.Run("success when member", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		roomID := uuid.New()
		userID := uuid.New()

		messages := []domain.Message{
			{ID: uuid.New(), RoomID: roomID, Content: "Hello"},
			{ID: uuid.New(), RoomID: roomID, Content: "World"},
		}

		roomRepo.On("IsMember", roomID, userID).Return(true, nil)
		msgRepo.On("FindByRoomID", roomID, 1, 50).Return(messages, int64(2), nil)

		result, err := svc.GetMessages(roomID, userID, 1, 50)

		require.NoError(t, err)
		assert.Equal(t, int64(2), result.Total)
		assert.Equal(t, 1, result.TotalPages)
		roomRepo.AssertExpectations(t)
		msgRepo.AssertExpectations(t)
	})

	t.Run("rejected when not a member", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		roomID := uuid.New()
		userID := uuid.New()

		roomRepo.On("IsMember", roomID, userID).Return(false, nil)

		_, err := svc.GetMessages(roomID, userID, 1, 50)

		assert.EqualError(t, err, "you are not a member of this room")
		roomRepo.AssertExpectations(t)
		msgRepo.AssertNotCalled(t, "FindByRoomID")
	})

	t.Run("pagination calculates total pages correctly", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		roomID := uuid.New()
		userID := uuid.New()

		roomRepo.On("IsMember", roomID, userID).Return(true, nil)
		// 105 total messages, limit 50 -> 3 pages
		msgRepo.On("FindByRoomID", roomID, 1, 50).Return([]domain.Message{}, int64(105), nil)

		result, err := svc.GetMessages(roomID, userID, 1, 50)

		require.NoError(t, err)
		assert.Equal(t, 3, result.TotalPages)
	})
}

func TestMessageService_EditMessage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		userID := uuid.New()

		existing := &domain.Message{
			ID:       msgID,
			SenderID: userID,
			Content:  "old content",
		}

		msgRepo.On("FindByID", msgID).Return(existing, nil)
		msgRepo.On("Update", existing).Return(nil)

		result, err := svc.EditMessage(msgID, userID, domain.EditMessageRequest{Content: "new content"})

		require.NoError(t, err)
		assert.Equal(t, "new content", result.Content)
		assert.True(t, result.IsEdited)
		msgRepo.AssertExpectations(t)
	})

	t.Run("rejects editing others' messages", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		ownerID := uuid.New()
		otherID := uuid.New()

		existing := &domain.Message{ID: msgID, SenderID: ownerID, Content: "original"}
		msgRepo.On("FindByID", msgID).Return(existing, nil)

		_, err := svc.EditMessage(msgID, otherID, domain.EditMessageRequest{Content: "hacked"})

		assert.EqualError(t, err, "you can only edit your own messages")
		msgRepo.AssertNotCalled(t, "Update")
	})

	t.Run("rejects editing deleted messages", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		userID := uuid.New()

		existing := &domain.Message{ID: msgID, SenderID: userID, IsDeleted: true}
		msgRepo.On("FindByID", msgID).Return(existing, nil)

		_, err := svc.EditMessage(msgID, userID, domain.EditMessageRequest{Content: "new"})

		assert.EqualError(t, err, "cannot edit a deleted message")
	})

	t.Run("message not found", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		msgRepo.On("FindByID", msgID).Return(nil, errors.New("not found"))

		_, err := svc.EditMessage(msgID, uuid.New(), domain.EditMessageRequest{Content: "x"})

		assert.EqualError(t, err, "message not found")
	})
}

func TestMessageService_DeleteMessage(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		userID := uuid.New()

		existing := &domain.Message{ID: msgID, SenderID: userID}
		msgRepo.On("FindByID", msgID).Return(existing, nil)
		msgRepo.On("SoftDelete", msgID).Return(nil)

		err := svc.DeleteMessage(msgID, userID)

		require.NoError(t, err)
		msgRepo.AssertExpectations(t)
	})

	t.Run("rejects deleting others' messages", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		ownerID := uuid.New()
		otherID := uuid.New()

		existing := &domain.Message{ID: msgID, SenderID: ownerID}
		msgRepo.On("FindByID", msgID).Return(existing, nil)

		err := svc.DeleteMessage(msgID, otherID)

		assert.EqualError(t, err, "you can only delete your own messages")
		msgRepo.AssertNotCalled(t, "SoftDelete")
	})
}
