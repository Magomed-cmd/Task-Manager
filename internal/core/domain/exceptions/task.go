package exceptions

import "errors"

var (
	ErrTaskNotCompleted     = errors.New("task is not completed yet")
	ErrRewardAlreadyClaimed = errors.New("reward already claimed")
	ErrTaskNotFound         = errors.New("task not found")
	ErrProgressNotFound     = errors.New("progress not found")
	ErrEventNil             = errors.New("event is nil")
	ErrEventIDRequired      = errors.New("event_id is required")
	ErrEventUserIDRequired  = errors.New("user_id is required")
	ErrEventTypeRequired    = errors.New("event type is required")
	ErrUnsupportedEventType = errors.New("unsupported event type")
	ErrEventPayloadInvalid  = errors.New("event payload is invalid")
	ErrEventTaskIDRequired  = errors.New("event task_id is required")
	ErrEventAmountInvalid   = errors.New("event amount is invalid")
)
