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
	id          string
	title       string
	description string
	taskType    TaskType
	target      int
	reward      json.RawMessage
	isActive    bool
	createdAt   time.Time
}

func NewTask(id, title, description string, taskType TaskType, target int, reward json.RawMessage, isActive bool, createdAt time.Time) *Task {
	var rewardCopy json.RawMessage
	if len(reward) > 0 {
		rewardCopy = append(json.RawMessage(nil), reward...)
	}
	return &Task{
		id:          id,
		title:       title,
		description: description,
		taskType:    taskType,
		target:      target,
		reward:      rewardCopy,
		isActive:    isActive,
		createdAt:   createdAt,
	}
}

func (t *Task) ID() string {
	return t.id
}

func (t *Task) Title() string {
	return t.title
}

func (t *Task) Description() string {
	return t.description
}

func (t *Task) Type() TaskType {
	return t.taskType
}

func (t *Task) Target() int {
	return t.target
}

func (t *Task) Reward() json.RawMessage {
	if len(t.reward) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), t.reward...)
}

func (t *Task) IsActive() bool {
	return t.isActive
}

func (t *Task) CreatedAt() time.Time {
	return t.createdAt
}
