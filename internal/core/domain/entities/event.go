package entities

import (
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
	payload     *ProgressPayload
	createdAt   time.Time
	processedAt time.Time
}

func NewTaskEvent(eventID, userID, roomID string, eventType TaskEventType, payload *ProgressPayload, createdAt time.Time) (*TaskEvent, error) {
	event := &TaskEvent{
		eventID:   eventID,
		userID:    userID,
		roomID:    roomID,
		eventType: eventType,
		payload:   payload,
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

func (e *TaskEvent) Payload() *ProgressPayload {
	if e.payload == nil {
		return nil
	}
	return &ProgressPayload{
		TaskID: e.payload.TaskID,
		Amount: e.payload.Amount,
	}
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
	if e.requiresProgressPayload() {
		if e.payload == nil {
			return exceptions.ErrEventPayloadInvalid
		}
		if err := e.payload.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (e *TaskEvent) requiresProgressPayload() bool {
	switch e.eventType {
	case EventTypeProgressUpdate, EventTypeTaskSubscribed, EventTypeTaskStepCounted:
		return true
	default:
		return false
	}
}
