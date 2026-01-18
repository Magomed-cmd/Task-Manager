package main

import (
	"fmt"

	"task-manager/internal/config"
	"task-manager/internal/logger"
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
	_ = log.Sync()
}
