package repository

import (
	"context"

	"ai-service-platform/backend/internal/domain/entity"
)

type VisionRepository interface {
	CreateTask(ctx context.Context, task *entity.VisionTask) error
	GetTaskByID(ctx context.Context, taskID, userID uint) (*entity.VisionTask, error)
	GetTaskByIDAnyUser(ctx context.Context, taskID uint) (*entity.VisionTask, error)
	UpdateTask(ctx context.Context, task *entity.VisionTask) error
}
