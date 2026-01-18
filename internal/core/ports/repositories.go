package ports

import (
	"context"
	"task-manager/internal/core/domain/entities"
)

type TaskRepository interface {
	GetByID(ctx context.Context, id string) (*entities.Task, error)
	ListActive(ctx context.Context) ([]*entities.Task, error)
}

type ProgressRepository interface {
	Get(ctx context.Context, userID, taskID string) (*entities.TaskProgress, error)
	Save(ctx context.Context, progress *entities.TaskProgress) error
}

type EventRepository interface {
	IsProcessed(ctx context.Context, eventID string) (bool, error)
	MarkProcessed(ctx context.Context, event *entities.TaskEvent) error
}
