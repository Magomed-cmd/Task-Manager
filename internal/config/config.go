package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Logger   LoggerConfig
	Database DatabaseConfig
	GRPC     GRPCConfig
}

type LoggerConfig struct {
	Env string
}

type DatabaseConfig struct {
	Name     string
	User     string
	Password string
	Port     int
}

type GRPCConfig struct {
	Port                       int
	StreamEventsIdleTimeout    time.Duration
	StreamEventsBatchTimeout   time.Duration
	SubscribeProgressInterval  time.Duration
	SubscribeProgressMaxPeriod time.Duration
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("error loading .env file: %w", err)
	}

	return &Config{
		Logger: LoggerConfig{
			Env: getEnv("LOGGER_ENV", "development"),
		},
		Database: DatabaseConfig{
			Name:     getEnv("POSTGRES_DB", "task_manager"),
			User:     getEnv("POSTGRES_USER", "task_manager"),
			Password: getEnv("POSTGRES_PASSWORD", "task_manager"),
			Port:     getEnvInt("POSTGRES_PORT", 5432),
		},
		GRPC: GRPCConfig{
			Port:                       getEnvInt("GRPC_PORT", 50051),
			StreamEventsIdleTimeout:    getEnvDuration("GRPC_STREAM_EVENTS_IDLE_TIMEOUT", 30*time.Second),
			StreamEventsBatchTimeout:   getEnvDuration("GRPC_STREAM_EVENTS_BATCH_TIMEOUT", 5*time.Second),
			SubscribeProgressInterval:  getEnvDuration("GRPC_SUBSCRIBE_PROGRESS_INTERVAL", 2*time.Second),
			SubscribeProgressMaxPeriod: getEnvDuration("GRPC_SUBSCRIBE_PROGRESS_MAX_PERIOD", 5*time.Minute),
		},
	}, nil
}

func (c *Config) GetDSN() string {
	return fmt.Sprintf("postgres://%s:%s@localhost:%d/%s?sslmode=disable",
		c.Database.User,
		c.Database.Password,
		c.Database.Port,
		c.Database.Name,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
