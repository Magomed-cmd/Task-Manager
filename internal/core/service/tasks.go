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

		if err := s.processEventWithRepos(ctx, repos, event); err != nil {
			s.log.Warn("usecase: process event failed", zap.Error(err))
			return err
		}
		s.log.Info("usecase: process event done", zap.String("event_id", event.EventID()))
		return nil
	})
}

func (s *TaskService) ProcessEvents(ctx context.Context, events []*entities.TaskEvent) (int32, int32, error) {
	if len(events) == 0 {
		return 0, 0, nil
	}

	var rejected int32
	valid := make([]*entities.TaskEvent, 0, len(events))
	for _, event := range events {
		if event == nil {
			rejected++
			continue
		}
		if err := event.Validate(); err != nil {
			rejected++
			continue
		}
		valid = append(valid, event)
	}
	if len(valid) == 0 {
		return 0, rejected, nil
	}

	var accepted int32
	err := s.uow.Do(ctx, func(uow ports.UnitOfWork) error {
		repos := uow.Repositories()
		for _, event := range valid {
			if err := s.processEventWithRepos(ctx, repos, event); err != nil {
				if isNonFatalEventError(err) {
					rejected++
					continue
				}
				return err
			}
			accepted++
		}
		return nil
	})
	if err != nil {
		return accepted, rejected, err
	}
	return accepted, rejected, nil
}

func (s *TaskService) ClaimReward(ctx context.Context, userID string, taskID string) error {
	s.log.Info("usecase: claim reward", zap.String("user_id", userID), zap.String("task_id", taskID))
	err := s.uow.Do(ctx, func(uow ports.UnitOfWork) error {
		repos := uow.Repositories()

		if _, err := repos.Tasks.GetByID(ctx, taskID); err != nil {
			return err
		}

		if err := repos.Progress.Claim(ctx, userID, taskID); err != nil {
			if errors.Is(err, exceptions.ErrRewardAlreadyClaimed) {
				return nil
			}
			return err
		}

		return nil
	})
	if err != nil {
		s.log.Warn("usecase: claim reward failed", zap.Error(err))
		return err
	}
	s.log.Info("usecase: claim reward done", zap.String("user_id", userID), zap.String("task_id", taskID))
	return nil
}

func (s *TaskService) processEventWithRepos(ctx context.Context, repos ports.Repositories, event *entities.TaskEvent) error {
	processed, err := repos.Events.IsProcessed(ctx, event.EventID())
	if err != nil {
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
			return err
		}
	default:
		return exceptions.ErrUnsupportedEventType
	}

	if event.ProcessedAt().IsZero() {
		event.SetProcessedAt(s.now())
	}
	return repos.Events.MarkProcessed(ctx, event)
}

func isNonFatalEventError(err error) bool {
	return errors.Is(err, exceptions.ErrUnsupportedEventType) ||
		errors.Is(err, exceptions.ErrTaskNotFound) ||
		errors.Is(err, exceptions.ErrTaskInactive) ||
		errors.Is(err, exceptions.ErrProgressNotFound)
}

func (s *TaskService) applyProgressUpdate(ctx context.Context, repos ports.Repositories, userID string, taskID string, amount int) error {
	task, err := repos.Tasks.GetByID(ctx, taskID)
	if err != nil {
		return err
	}

	if !task.IsActive() {
		return exceptions.ErrTaskInactive
	}

	return repos.Progress.AddProgress(ctx, userID, task.ID(), amount, task.Target(), s.now())
}
