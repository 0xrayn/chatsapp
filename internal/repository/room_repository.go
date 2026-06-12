package repository

import (
	"chatapp/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type roomRepository struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) domain.RoomRepository {
	return &roomRepository{db: db}
}

func (r *roomRepository) Create(room *domain.Room) error {
	return r.db.Create(room).Error
}

func (r *roomRepository) FindByID(id uuid.UUID) (*domain.Room, error) {
	var room domain.Room
	err := r.db.
		Preload("Creator").
		Preload("Members.User").
		Where("id = ?", id).
		First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *roomRepository) FindAll(page, limit int) ([]domain.Room, int64, error) {
	var rooms []domain.Room
	var total int64

	offset := (page - 1) * limit

	r.db.Model(&domain.Room{}).Where("type = ?", domain.RoomTypePublic).Count(&total)

	err := r.db.
		Preload("Creator").
		Where("type = ?", domain.RoomTypePublic).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&rooms).Error

	return rooms, total, err
}

func (r *roomRepository) FindByUserID(userID uuid.UUID) ([]domain.Room, error) {
	var rooms []domain.Room
	err := r.db.
		Preload("Creator").
		Joins("JOIN room_members ON room_members.room_id = rooms.id").
		Where("room_members.user_id = ?", userID).
		Order("rooms.updated_at DESC").
		Find(&rooms).Error
	return rooms, err
}

func (r *roomRepository) Update(room *domain.Room) error {
	return r.db.Save(room).Error
}

func (r *roomRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.Room{}, "id = ?", id).Error
}

func (r *roomRepository) AddMember(member *domain.RoomMember) error {
	return r.db.Create(member).Error
}

func (r *roomRepository) RemoveMember(roomID, userID uuid.UUID) error {
	return r.db.
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Delete(&domain.RoomMember{}).Error
}

func (r *roomRepository) IsMember(roomID, userID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&domain.RoomMember{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *roomRepository) GetMembers(roomID uuid.UUID) ([]domain.RoomMember, error) {
	var members []domain.RoomMember
	err := r.db.
		Preload("User").
		Where("room_id = ?", roomID).
		Find(&members).Error
	return members, err
}

func (r *roomRepository) FindDirectRoom(userAID, userBID uuid.UUID) (*domain.Room, error) {
	var room domain.Room
	err := r.db.
		Preload("Members.User").
		Joins("JOIN room_members rm1 ON rm1.room_id = rooms.id AND rm1.user_id = ?", userAID).
		Joins("JOIN room_members rm2 ON rm2.room_id = rooms.id AND rm2.user_id = ?", userBID).
		Where("rooms.type = ?", domain.RoomTypeDirect).
		First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *roomRepository) FindDirectRoomsByUserID(userID uuid.UUID) ([]domain.Room, error) {
	var rooms []domain.Room
	err := r.db.
		Preload("Members.User").
		Joins("JOIN room_members ON room_members.room_id = rooms.id").
		Where("room_members.user_id = ? AND rooms.type = ?", userID, domain.RoomTypeDirect).
		Order("rooms.updated_at DESC").
		Find(&rooms).Error
	return rooms, err
}
