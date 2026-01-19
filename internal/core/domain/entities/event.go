package entities

import (
	"encoding/json"
	"time"

	"task-manager/internal/core/domain/exceptions"
)

type TaskEventType string

const (
	EventTypeProgressUpdate TaskEventType = "progress_update"
	EventTypeTaskSubscribed TaskEventType = "task_subscribed"
	EventTypeTaskStepCounted TaskEventType = "task_step_counted"
	EventTypeClaimReward    TaskEventType = "claim_reward"
)

type TaskEvent struct {
	eventID     string
	userID      string
	roomID      string
	eventType   TaskEventType
	payload     json.RawMessage
	createdAt   time.Time
	processedAt time.Time
}

func NewTaskEvent(eventID, userID, roomID string, eventType TaskEventType, payload json.RawMessage, createdAt time.Time) (*TaskEvent, error) {
	var payloadCopy json.RawMessage
	if len(payload) > 0 {
		payloadCopy = append(json.RawMessage(nil), payload...)
	}
	event := &TaskEvent{
		eventID:   eventID,
		userID:    userID,
		roomID:    roomID,
		eventType: eventType,
		payload:   payloadCopy,
		createdAt: createdAt,
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return event, nil
}

func (e *TaskEvent) EventID() string {
	return e.eventID
}

func (e *TaskEvent) UserID() string {
	return e.userID
}

func (e *TaskEvent) RoomID() string {
	return e.roomID
}

func (e *TaskEvent) Type() TaskEventType {
	return e.eventType
}

func (e *TaskEvent) Payload() json.RawMessage {
	if len(e.payload) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), e.payload...)
}

func (e *TaskEvent) CreatedAt() time.Time {
	return e.createdAt
}

func (e *TaskEvent) ProcessedAt() time.Time {
	return e.processedAt
}

func (e *TaskEvent) SetProcessedAt(at time.Time) {
	e.processedAt = at
}

func (e *TaskEvent) Validate() error {
	if e == nil {
		return exceptions.ErrEventNil
	}
	if e.eventID == "" {
		return exceptions.ErrEventIDRequired
	}
	if e.userID == "" {
		return exceptions.ErrEventUserIDRequired
	}
	if e.eventType == "" {
		return exceptions.ErrEventTypeRequired
	}
	return nil
}