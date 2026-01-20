package app

import (
	"fmt"
	"net"

	grpcadapter "task-manager/internal/adapters/input/grpc"
	"task-manager/internal/adapters/output/postgres"
	"task-manager/internal/config"
	"task-manager/internal/core/ports"
	"task-manager/internal/core/service"
	dbinfra "task-manager/internal/infrastructure/db"
	"task-manager/internal/logger"
	tasksv1 "task-manager/pkg/grpc/gen/tasks"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type App struct {
	Config     *config.Config
	Log        *zap.Logger
	GRPCServer *grpc.Server
	Listener   net.Listener
	close      func()
}

func Init() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("config load error: %w", err)
	}

	log, err := logger.Init(cfg.Logger.Env)
	if err != nil {
		return nil, fmt.Errorf("logger init error: %w", err)
	}

	pool, err := dbinfra.ConnectToDB(cfg.GetDSN(), log)
	if err != nil {
		log.Error("failed to connect to db", zap.Error(err))
		_ = log.Sync()
		return nil, err
	}

	taskRepo := postgres.NewTaskRepository(pool, log)
	progressRepo := postgres.NewProgressRepository(pool, log)
	eventRepo := postgres.NewEventRepository(pool, log)

	repoFactory := func(q dbinfra.Querier) ports.Repositories {
		return ports.Repositories{
			Tasks:    postgres.NewTaskRepository(q, log),
			Progress: postgres.NewProgressRepository(q, log),
			Events:   postgres.NewEventRepository(q, log),
		}
	}
	uow := dbinfra.NewUnitOfWorkManager(pool, log, repoFactory)

	taskService, err := service.NewTaskService(taskRepo, progressRepo, eventRepo, uow, log)
	if err != nil {
		log.Error("failed to init task service", zap.Error(err))
		pool.Close()
		_ = log.Sync()
		return nil, err
	}

	grpcAddr := fmt.Sprintf(":%d", cfg.GRPC.Port)
	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Error("failed to listen grpc", zap.Error(err))
		pool.Close()
		_ = log.Sync()
		return nil, err
	}

	grpcServer := grpc.NewServer()
	tasksv1.RegisterTaskServiceServer(grpcServer, grpcadapter.NewTaskServer(taskService, log))
	reflection.Register(grpcServer)

	return &App{
		Config:     cfg,
		Log:        log,
		GRPCServer: grpcServer,
		Listener:   listener,
		close: func() {
			_ = listener.Close()
			pool.Close()
			_ = log.Sync()
		},
	}, nil
}

func (a *App) Close() {
	if a == nil || a.close == nil {
		return
	}
	a.close()
}
