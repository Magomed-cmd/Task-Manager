package db

import (
	"context"
	"fmt"

	"task-manager/internal/core/ports"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type RepoFactory func(q Querier) ports.Repositories

type UnitOfWorkManager struct {
	pool    *pgxpool.Pool
	log     *zap.Logger
	factory RepoFactory
}

func NewUnitOfWorkManager(pool *pgxpool.Pool, log *zap.Logger, factory RepoFactory) *UnitOfWorkManager {
	if pool == nil {
		log.Fatal("database pool is nil")
	}
	if log == nil {
		panic("logger is nil")
	}
	if factory == nil {
		log.Fatal("repository factory is nil")
	}
	return &UnitOfWorkManager{
		pool:    pool,
		log:     log,
		factory: factory,
	}
}

func (m *UnitOfWorkManager) Begin(ctx context.Context) (ports.UnitOfWork, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	return &unitOfWork{
		tx:    tx,
		repos: m.factory(tx),
	}, nil
}

func (m *UnitOfWorkManager) Do(ctx context.Context, fn func(uow ports.UnitOfWork) error) (err error) {
	uow, err := m.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			_ = uow.Rollback(ctx)
			panic(r)
		}
		if err != nil {
			if rbErr := uow.Rollback(ctx); rbErr != nil {
				err = fmt.Errorf("%w; rollback failed: %v", err, rbErr)
			}
		}
	}()

	if err = fn(uow); err != nil {
		return err
	}

	if err = uow.Commit(ctx); err != nil {
		return err
	}
	return nil
}

type unitOfWork struct {
	tx      pgx.Tx
	repos   ports.Repositories
	closed  bool
}

func (u *unitOfWork) Repositories() ports.Repositories {
	return u.repos
}

func (u *unitOfWork) Commit(ctx context.Context) error {
	if u.closed {
		return fmt.Errorf("unit of work already closed")
	}
	u.closed = true
	if err := u.tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (u *unitOfWork) Rollback(ctx context.Context) error {
	if u.closed {
		return fmt.Errorf("unit of work already closed")
	}
	u.closed = true
	return u.tx.Rollback(ctx)
}
