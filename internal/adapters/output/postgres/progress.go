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

// type ProgressRepository interface {
// 	Get(ctx context.Context, userID string, taskID string) (*entities.TaskProgress, error)
// 	Create(ctx context.Context, progress *entities.TaskProgress) error
// 	Update(ctx context.Context, progress *entities.TaskProgress) error
// }

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
