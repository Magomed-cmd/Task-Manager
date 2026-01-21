package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"task-manager/internal/infrastructure/app"

	"go.uber.org/zap"
)

func main() {
	application, err := app.Init()
	if err != nil {
		fmt.Printf("app init error: %v\n", err)
		return
	}
	defer application.Close()

	go func() {
		application.Log.Info("grpc server started", zap.String("addr", application.Listener.Addr().String()))
		if err := application.GRPCServer.Serve(application.Listener); err != nil {
			application.Log.Error("grpc server stopped", zap.Error(err))
		}
	}()

	application.Log.Info("server is starting", zap.String("env", application.Config.Logger.Env))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-quit:
		application.Log.Info("shutting down server", zap.String("signal", s.String()))
	}

	application.GRPCServer.GracefulStop()
	application.Log.Info("server stopped")
}
