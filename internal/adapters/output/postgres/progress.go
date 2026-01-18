package postgres

import (
	"context"
	"task-manager/internal/infrastructure/db"

	"go.uber.org/zap"
)

// type ProgressRepository interface {
// 	Get(ctx context.Context, userID string, taskID string) (*entities.TaskProgress, error)
// 	Save(ctx context.Context, progress *entities.TaskProgress) error
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

// TODO: IMPLEMENT!!!
func (r *ProgressRepository) Get(ctx context.Context, userID string, taskID string) {

}
