package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PostgresDB       string `yaml:"POSTGRES_DB"`
	PostgresUser     string `yaml:"POSTGRES_USER"`
	PostgresPassword string `yaml:"POSTGRES_PASSWORD"`
	PostgresPort     int    `yaml:"POSTGRES_PORT"`
	LoggerEnv        string `yaml:"LOGGER_ENV"`
}

func LoadConfig() (Config, error) {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		return Config{}, fmt.Errorf("read config.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config.yaml: %w", err)
	}

	var missing []string
	if cfg.PostgresDB == "" {
		missing = append(missing, "POSTGRES_DB")
	}
	if cfg.PostgresUser == "" {
		missing = append(missing, "POSTGRES_USER")
	}
	if cfg.PostgresPassword == "" {
		missing = append(missing, "POSTGRES_PASSWORD")
	}
	if cfg.PostgresPort == 0 {
		missing = append(missing, "POSTGRES_PORT")
	}
	if cfg.LoggerEnv == "" {
		missing = append(missing, "LOGGER_ENV")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}
