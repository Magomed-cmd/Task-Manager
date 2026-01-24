package ports

import (
	"context"
	"task-manager/internal/core/domain/entities"
)

type TaskUseCases interface {
	GetTasksWithProgress(ctx context.Context, userID string) ([]*entities.Task, []*entities.TaskProgress, error)
	GetTask(ctx context.Context, taskID string) (*entities.Task, error)
	ProcessEvent(ctx context.Context, event *entities.TaskEvent) error
	ProcessEvents(ctx context.Context, events []*entities.TaskEvent) (int32, int32, error)
	ClaimReward(ctx context.Context, userID string, taskID string) error
}
