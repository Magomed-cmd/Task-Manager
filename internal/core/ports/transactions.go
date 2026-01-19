package ports

import "context"

type Repositories struct {
	Tasks    TaskRepository
	Progress ProgressRepository
	Events   EventRepository
}

type UnitOfWork interface {
	Repositories() Repositories
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type UnitOfWorkManager interface {
	Begin(ctx context.Context) (UnitOfWork, error)
	Do(ctx context.Context, fn func(uow UnitOfWork) error) error
}
