package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"task-manager/internal/config"
	"task-manager/internal/infrastructure/db"
	"task-manager/internal/logger"

	"go.uber.org/zap"
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

	db, err := db.ConnectToDB(cfg.GetDSN(), log)
	if err != nil {
		log.Error("failed to connect to db", zap.Error(err))
		return
	}
	defer db.Close()

	log.Info("server is starting", zap.String("env", cfg.Logger.Env))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-quit:
		log.Info("shutting down server", zap.String("signal", s.String()))
	}

	log.Info("server stopped")
}
