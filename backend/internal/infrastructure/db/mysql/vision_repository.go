package mysql

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"ai-service-platform/backend/internal/domain/entity"
)

type VisionRepository struct {
	db *gorm.DB
}

func NewVisionRepository(db *gorm.DB) *VisionRepository {
	return &VisionRepository{db: db}
}

func (r *VisionRepository) CreateTask(ctx context.Context, task *entity.VisionTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *VisionRepository) GetTaskByID(ctx context.Context, taskID, userID uint) (*entity.VisionTask, error) {
	var task entity.VisionTask
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", taskID, userID).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *VisionRepository) GetTaskByIDAnyUser(ctx context.Context, taskID uint) (*entity.VisionTask, error) {
	var task entity.VisionTask
	err := r.db.WithContext(ctx).Where("id = ?", taskID).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *VisionRepository) UpdateTask(ctx context.Context, task *entity.VisionTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}
