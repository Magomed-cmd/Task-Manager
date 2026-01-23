package ports

import (
	"context"
	"task-manager/internal/core/domain/entities"
	"time"
)

type TaskRepository interface {
	GetByID(ctx context.Context, id string) (*entities.Task, error)
	ListActive(ctx context.Context) ([]*entities.Task, error)
}

type ProgressRepository interface {
	Get(ctx context.Context, userID string, taskID string) (*entities.TaskProgress, error)
	Create(ctx context.Context, progress *entities.TaskProgress) error
	Update(ctx context.Context, progress *entities.TaskProgress) error
	AddProgress(ctx context.Context, userID string, taskID string, amount int, target int, updatedAt time.Time) error
	Claim(ctx context.Context, userID string, taskID string) error
}

type EventRepository interface {
	IsProcessed(ctx context.Context, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, event *entities.TaskEvent) error
}
