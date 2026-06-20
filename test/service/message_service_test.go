package service_test

import (
	"errors"
	"testing"
	"time"

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
	t.Run("success within window and no read receipt", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		userID := uuid.New()
		roomID := uuid.New()

		existing := &domain.Message{
			ID:        msgID,
			RoomID:    roomID,
			SenderID:  userID,
			Content:   "old content",
			CreatedAt: time.Now().Add(-30 * time.Second), // well within 3-minute window
		}

		msgRepo.On("FindByID", msgID).Return(existing, nil)
		// No other member has read yet
		roomRepo.On("GetMembers", roomID).Return([]domain.RoomMember{
			{UserID: userID},
			{UserID: uuid.New(), ReadAt: nil},
		}, nil)
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

		existing := &domain.Message{ID: msgID, SenderID: ownerID, Content: "original", CreatedAt: time.Now()}
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

		existing := &domain.Message{ID: msgID, SenderID: userID, IsDeleted: true, CreatedAt: time.Now()}
		msgRepo.On("FindByID", msgID).Return(existing, nil)

		_, err := svc.EditMessage(msgID, userID, domain.EditMessageRequest{Content: "new"})

		assert.EqualError(t, err, "cannot edit a deleted message")
	})

	t.Run("rejects editing after 3-minute window", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		userID := uuid.New()

		existing := &domain.Message{
			ID:        msgID,
			SenderID:  userID,
			Content:   "old",
			CreatedAt: time.Now().Add(-5 * time.Minute), // outside 3-minute window
		}
		msgRepo.On("FindByID", msgID).Return(existing, nil)

		_, err := svc.EditMessage(msgID, userID, domain.EditMessageRequest{Content: "new"})

		assert.EqualError(t, err, "messages can only be edited within 3 minutes of sending")
		msgRepo.AssertNotCalled(t, "Update")
	})

	t.Run("rejects editing once recipient has read it", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		userID := uuid.New()
		recipientID := uuid.New()
		roomID := uuid.New()
		sentAt := time.Now().Add(-30 * time.Second)
		readAt := time.Now().Add(-10 * time.Second) // read after message was sent

		existing := &domain.Message{
			ID:        msgID,
			RoomID:    roomID,
			SenderID:  userID,
			Content:   "old",
			CreatedAt: sentAt,
		}
		msgRepo.On("FindByID", msgID).Return(existing, nil)
		roomRepo.On("GetMembers", roomID).Return([]domain.RoomMember{
			{UserID: userID},
			{UserID: recipientID, ReadAt: &readAt},
		}, nil)

		_, err := svc.EditMessage(msgID, userID, domain.EditMessageRequest{Content: "new"})

		assert.EqualError(t, err, "cannot edit a message the recipient has already read")
		msgRepo.AssertNotCalled(t, "Update")
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
	t.Run("success within window and unread", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		userID := uuid.New()
		roomID := uuid.New()

		existing := &domain.Message{ID: msgID, RoomID: roomID, SenderID: userID, CreatedAt: time.Now().Add(-1 * time.Minute)}
		msgRepo.On("FindByID", msgID).Return(existing, nil)
		roomRepo.On("GetMembers", roomID).Return([]domain.RoomMember{
			{UserID: userID},
			{UserID: uuid.New(), ReadAt: nil},
		}, nil)
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

		existing := &domain.Message{ID: msgID, SenderID: ownerID, CreatedAt: time.Now()}
		msgRepo.On("FindByID", msgID).Return(existing, nil)

		err := svc.DeleteMessage(msgID, otherID)

		assert.EqualError(t, err, "you can only delete your own messages")
		msgRepo.AssertNotCalled(t, "SoftDelete")
	})

	t.Run("rejects deleting after 3-minute window", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		userID := uuid.New()

		existing := &domain.Message{ID: msgID, SenderID: userID, CreatedAt: time.Now().Add(-10 * time.Minute)}
		msgRepo.On("FindByID", msgID).Return(existing, nil)

		err := svc.DeleteMessage(msgID, userID)

		assert.EqualError(t, err, "messages can only be deleted within 3 minutes of sending")
		msgRepo.AssertNotCalled(t, "SoftDelete")
	})

	t.Run("rejects deleting already-deleted message", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		userID := uuid.New()

		existing := &domain.Message{ID: msgID, SenderID: userID, IsDeleted: true, CreatedAt: time.Now()}
		msgRepo.On("FindByID", msgID).Return(existing, nil)

		err := svc.DeleteMessage(msgID, userID)

		assert.EqualError(t, err, "message already deleted")
		msgRepo.AssertNotCalled(t, "SoftDelete")
	})

	t.Run("rejects deleting once recipient has read it", func(t *testing.T) {
		msgRepo := new(mock.MessageRepository)
		roomRepo := new(mock.RoomRepository)
		svc := service.NewMessageService(msgRepo, roomRepo)

		msgID := uuid.New()
		userID := uuid.New()
		recipientID := uuid.New()
		roomID := uuid.New()
		sentAt := time.Now().Add(-30 * time.Second)
		readAt := time.Now().Add(-10 * time.Second)

		existing := &domain.Message{ID: msgID, RoomID: roomID, SenderID: userID, CreatedAt: sentAt}
		msgRepo.On("FindByID", msgID).Return(existing, nil)
		roomRepo.On("GetMembers", roomID).Return([]domain.RoomMember{
			{UserID: userID},
			{UserID: recipientID, ReadAt: &readAt},
		}, nil)

		err := svc.DeleteMessage(msgID, userID)

		assert.EqualError(t, err, "cannot delete a message the recipient has already read")
		msgRepo.AssertNotCalled(t, "SoftDelete")
	})
}
