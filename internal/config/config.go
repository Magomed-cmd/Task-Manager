package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"

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
	Port int
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
			Port: getEnvInt("GRPC_PORT", 50051),
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
