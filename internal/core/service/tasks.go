package usecases

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"task-manager/internal/core/domain/entities"
	"task-manager/internal/core/domain/exceptions"
	"task-manager/internal/core/ports"
)

type TaskService struct {
	tasks    ports.TaskRepository
	progress ports.ProgressRepository
	events   ports.EventRepository
	uow      ports.UnitOfWorkManager
	now      func() time.Time
}

func NewTaskService(
	tasks ports.TaskRepository,
	progress ports.ProgressRepository,
	events ports.EventRepository,
	uow ports.UnitOfWorkManager,
) (*TaskService, error) {
	if uow == nil {
		return nil, errors.New("unit of work manager is nil")
	}
	return &TaskService{
		tasks:    tasks,
		progress: progress,
		events:   events,
		uow:      uow,
		now:      time.Now,
	}, nil
}

func (s *TaskService) GetTasksWithProgress(ctx context.Context, userID string) ([]*entities.Task, []*entities.TaskProgress, error) {
	tasks, err := s.tasks.ListActive(ctx)
	if err != nil {
		return nil, nil, err
	}

	progressList := make([]*entities.TaskProgress, 0, len(tasks))
	for _, task := range tasks {
		progress, err := s.progress.Get(ctx, userID, task.ID())
		if err != nil {
			if !errors.Is(err, exceptions.ErrProgressNotFound) {
				return nil, nil, err
			}
			progress = entities.NewTaskProgress(task.ID(), userID)
		}
		progressList = append(progressList, progress)
	}

	return tasks, progressList, nil
}

func (s *TaskService) GetTask(ctx context.Context, taskID string) (*entities.Task, error) {
	return s.tasks.GetByID(ctx, taskID)
}

func (s *TaskService) ProcessEvent(ctx context.Context, event *entities.TaskEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}

	return s.uow.Do(ctx, func(uow ports.UnitOfWork) error {
		repos := uow.Repositories()

		processed, err := repos.Events.IsProcessed(ctx, event.EventID())
		if err != nil {
			return err
		}
		if processed {
			return nil
		}

	switch event.Type() {
	case entities.EventTypeProgressUpdate,
		entities.EventTypeTaskSubscribed,
		entities.EventTypeTaskStepCounted:
		// TODO: Обсудить, как дальше обрабатывать все типы событий.
		payload, err := parseProgressPayload(event.Payload())
		if err != nil {
			return err
		}
		if err := s.applyProgressUpdate(ctx, repos, event.UserID(), payload.TaskID, payload.Amount); err != nil {
			return err
		}
	default:
		return exceptions.ErrUnsupportedEventType
	}

		if event.ProcessedAt().IsZero() {
			event.SetProcessedAt(s.now())
		}
		return repos.Events.MarkProcessed(ctx, event)
	})
}

func (s *TaskService) ClaimReward(ctx context.Context, userID string, taskID string) error {
	return s.uow.Do(ctx, func(uow ports.UnitOfWork) error {
		repos := uow.Repositories()

		if _, err := repos.Tasks.GetByID(ctx, taskID); err != nil {
			return err
		}

		progress, err := repos.Progress.Get(ctx, userID, taskID)
		if err != nil {
			return err
		}
		if err := progress.MarkClaimed(); err != nil {
			return err
		}

		return repos.Progress.Update(ctx, progress)
	})
}

func (s *TaskService) applyProgressUpdate(ctx context.Context, repos ports.Repositories, userID string, taskID string, amount int) error {
	task, err := repos.Tasks.GetByID(ctx, taskID)
	if err != nil {
		return err
	}

	if !task.IsActive() {
		return nil
	}

	now := s.now()
	progress, err := repos.Progress.Get(ctx, userID, task.ID())
	if err != nil {
		if !errors.Is(err, exceptions.ErrProgressNotFound) {
			return err
		}
		progress = entities.NewTaskProgress(task.ID(), userID)
		progress.AddProgress(amount, task.Target())
		progress.SetUpdatedAt(now)
		return repos.Progress.Create(ctx, progress)
	}

	if progress.Completed() {
		return nil
	}

	progress.AddProgress(amount, task.Target())
	progress.SetUpdatedAt(now)
	return repos.Progress.Update(ctx, progress)
}

type progressPayload struct {
	TaskID string `json:"task_id"`
	Amount int    `json:"amount"`
}

func parseProgressPayload(payload json.RawMessage) (*progressPayload, error) {
	if len(payload) == 0 {
		return nil, exceptions.ErrEventPayloadInvalid
	}
	var parsed progressPayload
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return nil, exceptions.ErrEventPayloadInvalid
	}
	if parsed.TaskID == "" {
		return nil, exceptions.ErrEventTaskIDRequired
	}
	if parsed.Amount <= 0 {
		return nil, exceptions.ErrEventAmountInvalid
	}
	return &parsed, nil
}
