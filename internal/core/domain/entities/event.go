package entities

import "time"

type TaskEventType string

const (
	EventTypeProgressUpdate TaskEventType = "progress_update"
	EventTypeClaimReward    TaskEventType = "claim_reward"
)

type TaskEvent struct {
	EventID     string        `json:"event_id"`
	UserID      string        `json:"user_id"`
	Type        TaskEventType `json:"type"`
	ProcessedAt time.Time     `json:"processed_at"`
}
