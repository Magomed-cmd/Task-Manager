package postgres

import (
	"context"
	"errors"
	"task-manager/internal/core/domain/entities"
	"task-manager/internal/core/domain/exceptions"
	"task-manager/internal/infrastructure/db"

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

	task := entities.Task{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&task.Type,
		&task.Target,
		&task.Reward,
		&task.IsActive,
		&task.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, exceptions.ErrTaskNotFound
		}
		return nil, err
	}
	return &task, nil
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
		task := entities.Task{}
		if err := rows.Scan(
			&task.ID,
			&task.Title,
			&task.Description,
			&task.Type,
			&task.Target,
			&task.Reward,
			&task.IsActive,
			&task.CreatedAt,
		); err != nil {
			r.log.Error("failed to scan task row", zap.Error(err))
			return nil, err
		}
		tasks = append(tasks, &task)
	}

	if err := rows.Err(); err != nil {
		r.log.Error("failed to iterate task rows", zap.Error(err))
		return nil, err
	}

	return tasks, nil
}
