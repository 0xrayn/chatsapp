package mock

import (
	"chatapp/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// AnyUser returns a testify matcher that accepts any *domain.User argument
func AnyUser() interface{} {
	return mock.MatchedBy(func(u *domain.User) bool {
		return u != nil && u.Username != "" && u.Email != ""
	})
}

// AnyRoomMember returns a matcher that accepts any *domain.RoomMember
func AnyRoomMember() interface{} {
	return mock.MatchedBy(func(m *domain.RoomMember) bool {
		return m != nil
	})
}

// MatchedByRoom matches a *domain.Room by name
func MatchedByRoom(name string) interface{} {
	return mock.MatchedBy(func(r *domain.Room) bool {
		return r != nil && r.Name == name
	})
}

// AnythingUUID matches any uuid.UUID
func AnythingUUID() interface{} {
	return mock.MatchedBy(func(id uuid.UUID) bool {
		return id != uuid.Nil
	})
}

// AnythingRoomPtr matches any non-nil *domain.Room
func AnythingRoomPtr() interface{} {
	return mock.MatchedBy(func(r *domain.Room) bool {
		return r != nil
	})
}
