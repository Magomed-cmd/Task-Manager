package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"task-manager/internal/adapters/output/postgres"
	"task-manager/internal/core/domain/entities"
	"task-manager/internal/core/domain/exceptions"
	"task-manager/internal/infrastructure/db"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func runRepoSmokeTest(ctx context.Context, log *zap.Logger, q db.Querier) {
	taskRepo := postgres.NewTaskRepository(q, log)
	progressRepo := postgres.NewProgressRepository(q, log)
	eventRepo := postgres.NewEventRepository(q, log)

	log.Info("smoke test: creating task")
	smokeTaskID, err := createSmokeTask(ctx, q)
	if err != nil {
		log.Error("smoke test: failed to create task", zap.Error(err))
		return
	}
	log.Info("smoke test: task created", zap.String("task_id", smokeTaskID))
	defer cleanupSmokeTask(ctx, log, q, smokeTaskID)

	log.Info("smoke test: listing active tasks")
	tasks, err := taskRepo.ListActive(ctx)
	if err != nil {
		log.Error("smoke test: failed to list tasks", zap.Error(err))
		return
	}
	if len(tasks) == 0 {
		log.Error("smoke test: active tasks list is empty")
		return
	}

	log.Info("smoke test: getting task by id", zap.String("task_id", smokeTaskID))
	if _, err := taskRepo.GetByID(ctx, smokeTaskID); err != nil {
		log.Error("smoke test: failed to get task", zap.Error(err))
		return
	}

	userID := fmt.Sprintf("smoke-user-%d", time.Now().UnixNano())
	log.Info("smoke test: getting progress", zap.String("task_id", smokeTaskID), zap.String("user_id", userID))
	progress, err := progressRepo.Get(ctx, userID, smokeTaskID)
	if err != nil {
		if !errors.Is(err, exceptions.ErrProgressNotFound) {
			log.Error("smoke test: failed to get progress", zap.Error(err))
			return
		}
	}

	progress = &entities.TaskProgress{
		TaskID:    smokeTaskID,
		UserID:    userID,
		Progress:  1,
		Completed: false,
		Claimed:   false,
		UpdatedAt: time.Now(),
	}
	log.Info("smoke test: creating progress", zap.String("task_id", smokeTaskID), zap.String("user_id", userID))
	if err := progressRepo.Create(ctx, progress); err != nil {
		log.Error("smoke test: failed to create progress", zap.Error(err))
		return
	}

	progress.Progress++
	progress.UpdatedAt = time.Now()
	log.Info("smoke test: updating progress", zap.String("progress_id", progress.ID))
	if err := progressRepo.Update(ctx, progress); err != nil {
		log.Error("smoke test: failed to update progress", zap.Error(err))
		return
	}

	log.Info(
		"smoke test: progress created and updated",
		zap.String("progress_id", progress.ID),
		zap.Int("progress", progress.Progress),
	)

	eventID := uuid.NewString()
	event := &entities.TaskEvent{
		EventID:     eventID,
		UserID:      userID,
		Type:        entities.EventTypeProgressUpdate,
		ProcessedAt: time.Now(),
	}

	log.Info("smoke test: checking event processed", zap.String("event_id", eventID))
	processed, err := eventRepo.IsProcessed(ctx, eventID)
	if err != nil {
		log.Error("smoke test: failed to check event processed", zap.Error(err))
		return
	}
	if processed {
		log.Warn("smoke test: event already processed", zap.String("event_id", eventID))
	}

	log.Info("smoke test: marking event processed", zap.String("event_id", eventID))
	if err := eventRepo.MarkProcessed(ctx, event); err != nil {
		log.Error("smoke test: failed to mark event processed", zap.Error(err))
		return
	}

	log.Info("smoke test: rechecking event processed", zap.String("event_id", eventID))
	processed, err = eventRepo.IsProcessed(ctx, eventID)
	if err != nil {
		log.Error("smoke test: failed to recheck event processed", zap.Error(err))
		return
	}
	if !processed {
		log.Error("smoke test: event should be processed", zap.String("event_id", eventID))
		return
	}
	log.Info("smoke test: event marked processed", zap.String("event_id", eventID))
}

func createSmokeTask(ctx context.Context, q db.Querier) (string, error) {
	query := `INSERT INTO tasks (title, description, type, target, reward, is_active)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
		RETURNING id`
	var id string
	err := q.QueryRow(
		ctx,
		query,
		"Smoke Test Task",
		"Temporary task for repository smoke test",
		"daily",
		1,
		"{}",
		true,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func cleanupSmokeTask(ctx context.Context, log *zap.Logger, q db.Querier, id string) {
	if id == "" {
		return
	}
	if _, err := q.Exec(ctx, "DELETE FROM tasks WHERE id = $1", id); err != nil {
		log.Error("smoke test: failed to cleanup task", zap.Error(err))
	}
}
