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

type TaskRepository struct {
	db  db.Querier
	log *zap.Logger
}

func NewTaskRepository(db db.Querier, log *zap.Logger) *TaskRepository {
	if db == nil {
		log.Fatal("database querier is nil")
	}
	if log == nil {
		log.Fatal("logger is nil")
	}
	return &TaskRepository{
		db:  db,
		log: log,
	}
}

func (r *TaskRepository) GetByID(ctx context.Context, id string) (*entities.Task, error) {
	query := `SELECT id, title, description, type, target, reward, is_active, created_at
		FROM tasks WHERE id = $1`

	var (
		taskID      string
		title       string
		description string
		taskType    entities.TaskType
		target      int
		reward      []byte
		isActive    bool
		createdAt   time.Time
	)
	err := r.db.QueryRow(ctx, query, id).Scan(
		&taskID,
		&title,
		&description,
		&taskType,
		&target,
		&reward,
		&isActive,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, exceptions.ErrTaskNotFound
		}
		return nil, err
	}
	return entities.NewTask(taskID, title, description, taskType, target, reward, isActive, createdAt), nil
}

func (r *TaskRepository) ListActive(ctx context.Context) ([]*entities.Task, error) {
	query := `SELECT id, title, description, type, target, reward, is_active, created_at
	FROM tasks WHERE is_active = true`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.log.Error("failed to list active tasks", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	tasks := make([]*entities.Task, 0)
	for rows.Next() {
		var (
			taskID      string
			title       string
			description string
			taskType    entities.TaskType
			target      int
			reward      []byte
			isActive    bool
			createdAt   time.Time
		)
		if err := rows.Scan(
			&taskID,
			&title,
			&description,
			&taskType,
			&target,
			&reward,
			&isActive,
			&createdAt,
		); err != nil {
			r.log.Error("failed to scan task row", zap.Error(err))
			return nil, err
		}
		tasks = append(tasks, entities.NewTask(taskID, title, description, taskType, target, reward, isActive, createdAt))
	}

	if err := rows.Err(); err != nil {
		r.log.Error("failed to iterate task rows", zap.Error(err))
		return nil, err
	}

	return tasks, nil
}
