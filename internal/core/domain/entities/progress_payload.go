package entities

import "task-manager/internal/core/domain/exceptions"

type ProgressPayload struct {
	TaskID string `json:"task_id"`
	Amount int    `json:"amount"`
}

func (p *ProgressPayload) Validate() error {
	if p == nil {
		return exceptions.ErrEventPayloadInvalid
	}
	if p.TaskID == "" {
		return exceptions.ErrEventTaskIDRequired
	}
	if p.Amount <= 0 {
		return exceptions.ErrEventAmountInvalid
	}
	return nil
}
