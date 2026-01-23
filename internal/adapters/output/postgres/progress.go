package postgres

import (
	"context"
	"errors"
	"task-manager/internal/core/domain/entities"
	"task-manager/internal/core/domain/exceptions"
	"task-manager/internal/infrastructure/db"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type ProgressRepository struct {
	db  db.Querier
	log *zap.Logger
}

func NewProgressRepository(db db.Querier, log *zap.Logger) *ProgressRepository {
	if db == nil {
		log.Fatal("database querier is nil")
	}
	if log == nil {
		log.Fatal("logger is nil")
	}
	return &ProgressRepository{
		db:  db,
		log: log,
	}
}

func (r *ProgressRepository) Get(ctx context.Context, userID string, taskID string) (*entities.TaskProgress, error) {
	query := `SELECT id, task_id, user_id, progress, completed, claimed, updated_at
		FROM task_progress WHERE user_id = $1 AND task_id = $2`

	var (
		progressID string
		taskIDVal  string
		userIDVal  string
		value      int
		completed  bool
		claimed    bool
		updatedAt  time.Time
	)
	err := r.db.QueryRow(ctx, query, userID, taskID).Scan(
		&progressID,
		&taskIDVal,
		&userIDVal,
		&value,
		&completed,
		&claimed,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, exceptions.ErrProgressNotFound
		}
		r.log.Error("failed to get task progress", zap.Error(err))
		return nil, err
	}
	return entities.NewTaskProgressFromData(progressID, taskIDVal, userIDVal, value, completed, claimed, updatedAt), nil
}

func (r *ProgressRepository) Create(ctx context.Context, progress *entities.TaskProgress) error {
	query := `INSERT INTO task_progress (task_id, user_id, progress, completed, claimed, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	var id string
	if err := r.db.QueryRow(
		ctx,
		query,
		progress.TaskID(),
		progress.UserID(),
		progress.Progress(),
		progress.Completed(),
		progress.Claimed(),
		progress.UpdatedAt(),
	).Scan(&id); err != nil {
		r.log.Error("failed to create task progress", zap.Error(err))
		return err
	}
	progress.SetID(id)
	return nil
}

func (r *ProgressRepository) Update(ctx context.Context, progress *entities.TaskProgress) error {
	query := `UPDATE task_progress
		SET progress = $3, completed = $4, claimed = $5, updated_at = $6
		WHERE user_id = $1 AND task_id = $2
		RETURNING id`

	var id string
	if err := r.db.QueryRow(
		ctx,
		query,
		progress.UserID(),
		progress.TaskID(),
		progress.Progress(),
		progress.Completed(),
		progress.Claimed(),
		progress.UpdatedAt(),
	).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exceptions.ErrProgressNotFound
		}
		r.log.Error("failed to update task progress", zap.Error(err))
		return err
	}
	progress.SetID(id)
	return nil
}

func (r *ProgressRepository) AddProgress(ctx context.Context, userID string, taskID string, amount int, target int, updatedAt time.Time) error {
	query := `INSERT INTO task_progress (task_id, user_id, progress, completed, claimed, updated_at)
		VALUES ($1, $2, LEAST($3::int, $4::int), $3::int >= $4::int, false, COALESCE($5, NOW()))
		ON CONFLICT (user_id, task_id) DO UPDATE
		SET progress = LEAST(task_progress.progress + EXCLUDED.progress, $4::int),
			completed = task_progress.completed OR (task_progress.progress + EXCLUDED.progress >= $4::int),
			updated_at = EXCLUDED.updated_at
		WHERE task_progress.completed = false`

	updatedAtValue := any(updatedAt)
	if updatedAt.IsZero() {
		updatedAtValue = nil
	}

	if _, err := r.db.Exec(
		ctx,
		query,
		taskID,
		userID,
		amount,
		target,
		updatedAtValue,
	); err != nil {
		r.log.Error("failed to add task progress", zap.Error(err))
		return err
	}
	return nil
}

func (r *ProgressRepository) Claim(ctx context.Context, userID string, taskID string) error {
	query := `UPDATE task_progress
		SET claimed = true, updated_at = NOW()
		WHERE user_id = $1 AND task_id = $2 AND claimed = false AND completed = true
		RETURNING id`

	var id string
	if err := r.db.QueryRow(ctx, query, userID, taskID).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return r.claimStateError(ctx, userID, taskID)
		}
		r.log.Error("failed to claim task reward", zap.Error(err))
		return err
	}
	return nil
}

func (r *ProgressRepository) claimStateError(ctx context.Context, userID string, taskID string) error {
	query := `SELECT completed, claimed FROM task_progress WHERE user_id = $1 AND task_id = $2`

	var completed, claimed bool
	if err := r.db.QueryRow(ctx, query, userID, taskID).Scan(&completed, &claimed); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return exceptions.ErrProgressNotFound
		}
		r.log.Error("failed to check claim state", zap.Error(err))
		return err
	}
	if !completed {
		return exceptions.ErrTaskNotCompleted
	}
	if claimed {
		return exceptions.ErrRewardAlreadyClaimed
	}
	return errors.New("claim reward failed")
}
