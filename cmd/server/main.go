package main

import (
	"fmt"

	"task-manager/internal/config"
	"task-manager/internal/infrastructure"
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

	db, err := infrastructure.ConnectToDB(cfg.GetDSN(), log)
	if err != nil {
		log.Error("failed to connect to db", zap.Error(err))
		return
	}
	defer db.Close()

	log.Info("connected to database successfully")
}
