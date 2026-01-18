package entities

import (
	"encoding/json"
	"time"
)

type TaskEventType string

const (
	EventTypeProgressUpdate TaskEventType = "progress_update"
	EventTypeClaimReward    TaskEventType = "claim_reward"
)

type TaskEvent struct {
	EventID     string          `json:"event_id"`
	UserID      string          `json:"user_id"`
	RoomID      string          `json:"room_id"`
	Type        TaskEventType   `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	ProcessedAt time.Time       `json:"processed_at"`
}
