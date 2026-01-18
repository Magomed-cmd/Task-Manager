package entities

import (
	"encoding/json"
	"time"
)

type TaskType string

const (
	TaskTypeSocial TaskType = "social"
	TaskTypeDaily  TaskType = "daily"
	TaskTypeGame   TaskType = "game"
)

type TaskStatus string

const (
	TaskStatusActive   TaskStatus = "active"
	TaskStatusInactive TaskStatus = "inactive"
)

type Task struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Type        TaskType        `json:"type"`
	Target      int             `json:"target"`
	Reward      json.RawMessage `json:"reward"`
	IsActive    bool            `json:"is_active"`
	CreatedAt   time.Time       `json:"created_at"`
}
