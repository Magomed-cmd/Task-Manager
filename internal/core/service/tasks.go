package service

import (
	"context"
	"errors"
	"time"

	"task-manager/internal/core/domain/entities"
	"task-manager/internal/core/domain/exceptions"
	"task-manager/internal/core/ports"

	"go.uber.org/zap"
)

type TaskService struct {
	tasks    ports.TaskRepository
	progress ports.ProgressRepository
	events   ports.EventRepository
	uow      ports.UnitOfWorkManager
	now      func() time.Time
	log      *zap.Logger
}

func NewTaskService(
	tasks ports.TaskRepository,
	progress ports.ProgressRepository,
	events ports.EventRepository,
	uow ports.UnitOfWorkManager,
	log *zap.Logger,
) (*TaskService, error) {
	if uow == nil {
		return nil, errors.New("unit of work manager is nil")
	}
	if log == nil {
		return nil, errors.New("logger is nil")
	}
	return &TaskService{
		tasks:    tasks,
		progress: progress,
		events:   events,
		uow:      uow,
		now:      time.Now,
		log:      log,
	}, nil
}

func (s *TaskService) GetTasksWithProgress(ctx context.Context, userID string) ([]*entities.Task, []*entities.TaskProgress, error) {
	s.log.Debug("usecase: get tasks with progress", zap.String("user_id", userID))
	tasks, err := s.tasks.ListActive(ctx)
	if err != nil {
		s.log.Warn("usecase: get tasks with progress failed", zap.Error(err))
		return nil, nil, err
	}

	progressList := make([]*entities.TaskProgress, 0, len(tasks))
	for _, task := range tasks {
		progress, err := s.progress.Get(ctx, userID, task.ID())
		if err != nil {
			if !errors.Is(err, exceptions.ErrProgressNotFound) {
				s.log.Warn("usecase: get tasks with progress failed", zap.Error(err))
				return nil, nil, err
			}
			progress = entities.NewTaskProgress(task.ID(), userID)
		}
		progressList = append(progressList, progress)
	}

	s.log.Debug("usecase: get tasks with progress done", zap.Int("tasks", len(tasks)), zap.Int("progress", len(progressList)))
	return tasks, progressList, nil
}

func (s *TaskService) GetTask(ctx context.Context, taskID string) (*entities.Task, error) {
	s.log.Debug("usecase: get task", zap.String("task_id", taskID))
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		s.log.Warn("usecase: get task failed", zap.Error(err))
		return nil, err
	}
	s.log.Debug("usecase: get task done", zap.String("task_id", taskID))
	return task, nil
}

func (s *TaskService) ProcessEvent(ctx context.Context, event *entities.TaskEvent) error {
	if err := event.Validate(); err != nil {
		s.log.Warn("usecase: process event validation failed", zap.Error(err))
		return err
	}

	s.log.Info("usecase: process event", zap.String("event_id", event.EventID()), zap.String("user_id", event.UserID()), zap.String("event_type", string(event.Type())))
	return s.uow.Do(ctx, func(uow ports.UnitOfWork) error {
		repos := uow.Repositories()

		processed, err := repos.Events.IsProcessed(ctx, event.EventID())
		if err != nil {
			s.log.Warn("usecase: process event failed", zap.Error(err))
			return err
		}
		if processed {
			s.log.Debug("usecase: event already processed", zap.String("event_id", event.EventID()))
			return nil
		}

		switch event.Type() {
		case entities.EventTypeProgressUpdate,
			entities.EventTypeTaskSubscribed,
			entities.EventTypeTaskStepCounted:
			// TODO: Обсудить, как дальше обрабатывать все типы событий.
			payload := event.Payload()
			if err := s.applyProgressUpdate(ctx, repos, event.UserID(), payload.TaskID, payload.Amount); err != nil {
				s.log.Warn("usecase: process event failed", zap.Error(err))
				return err
			}
		default:
			s.log.Warn("usecase: process event failed", zap.Error(exceptions.ErrUnsupportedEventType))
			return exceptions.ErrUnsupportedEventType
		}

		if event.ProcessedAt().IsZero() {
			event.SetProcessedAt(s.now())
		}
		if err := repos.Events.MarkProcessed(ctx, event); err != nil {
			s.log.Warn("usecase: process event failed", zap.Error(err))
			return err
		}
		s.log.Info("usecase: process event done", zap.String("event_id", event.EventID()))
		return nil
	})
}

func (s *TaskService) ClaimReward(ctx context.Context, userID string, taskID string) error {
	s.log.Info("usecase: claim reward", zap.String("user_id", userID), zap.String("task_id", taskID))
	err := s.uow.Do(ctx, func(uow ports.UnitOfWork) error {
		repos := uow.Repositories()

		if _, err := repos.Tasks.GetByID(ctx, taskID); err != nil {
			return err
		}

		progress, err := repos.Progress.Get(ctx, userID, taskID)
		if err != nil {
			return err
		}
		if err := progress.MarkClaimed(); err != nil {
			if errors.Is(err, exceptions.ErrRewardAlreadyClaimed) {
				return nil
			}
			return err
		}

		return repos.Progress.Update(ctx, progress)
	})
	if err != nil {
		s.log.Warn("usecase: claim reward failed", zap.Error(err))
		return err
	}
	s.log.Info("usecase: claim reward done", zap.String("user_id", userID), zap.String("task_id", taskID))
	return nil
}

func (s *TaskService) applyProgressUpdate(ctx context.Context, repos ports.Repositories, userID string, taskID string, amount int) error {
	task, err := repos.Tasks.GetByID(ctx, taskID)
	if err != nil {
		return err
	}

	if !task.IsActive() {
		return exceptions.ErrTaskInactive
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
