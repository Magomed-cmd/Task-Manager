package postgres

import (
	"context"
	"time"

	"task-manager/internal/core/domain/entities"
	"task-manager/internal/infrastructure/db"

	"go.uber.org/zap"
)

type EventRepository struct {
	db  db.Querier
	log *zap.Logger
}

func NewEventRepository(db db.Querier, log *zap.Logger) *EventRepository {
	if db == nil {
		log.Fatal("database querier is nil")
	}
	if log == nil {
		log.Fatal("logger is nil")
	}
	return &EventRepository{
		db:  db,
		log: log,
	}
}

func (r *EventRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM task_events WHERE event_id = $1)`
	var exists bool
	if err := r.db.QueryRow(ctx, query, eventID).Scan(&exists); err != nil {
		r.log.Error("failed to check event processed", zap.Error(err))
		return false, err
	}
	return exists, nil
}

func (r *EventRepository) MarkProcessed(ctx context.Context, event *entities.TaskEvent) error {
	query := `INSERT INTO task_events (event_id, user_id, type, processed_at)
		VALUES ($1, $2, $3, COALESCE($4, NOW()))
		ON CONFLICT (event_id) DO NOTHING`

	processedAt := any(event.ProcessedAt)
	if event.ProcessedAt.IsZero() {
		processedAt = (*time.Time)(nil)
	}

	if _, err := r.db.Exec(
		ctx,
		query,
		event.EventID,
		event.UserID,
		event.Type,
		processedAt,
	); err != nil {
		r.log.Error("failed to mark event processed", zap.Error(err))
		return err
	}
	return nil
}
