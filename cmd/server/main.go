package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

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

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("config load error: %v\n", err)
		return
	}

	log, err := logger.Init(cfg.Logger.Env)
	if err != nil {
		fmt.Printf("logger init error: %v\n", err)
		return
	}
	defer func() { _ = log.Sync() }()

	pool, err := dbinfra.ConnectToDB(cfg.GetDSN(), log)
	if err != nil {
		log.Error("failed to connect to db", zap.Error(err))
		return
	}
	defer pool.Close()

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
		return
	}

	grpcAddr := fmt.Sprintf(":%d", cfg.GRPC.Port)
	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Error("failed to listen grpc", zap.Error(err))
		return
	}

	grpcServer := grpc.NewServer()
	tasksv1.RegisterTaskServiceServer(grpcServer, grpcadapter.NewTaskServer(taskService, log))
	reflection.Register(grpcServer)

	go func() {
		log.Info("grpc server started", zap.String("addr", grpcAddr))
		if err := grpcServer.Serve(listener); err != nil {
			log.Error("grpc server stopped", zap.Error(err))
		}
	}()

	log.Info("server is starting", zap.String("env", cfg.Logger.Env))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-quit:
		log.Info("shutting down server", zap.String("signal", s.String()))
	}

	grpcServer.GracefulStop()
	log.Info("server stopped")
}
