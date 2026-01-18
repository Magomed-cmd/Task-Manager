package entities

import (
	"task-manager/internal/core/domain/exceptions"
	"time"
)

func (p *TaskProgress) AddProgress(amount int, target int) bool {
	if p.Completed {
		return false
	}

	p.Progress += amount
	if p.Progress >= target {
		p.Progress = target
		p.Completed = true
		return true
	}
	return false
}

func (p *TaskProgress) CanClaim() error {
	if !p.Completed {
		return exceptions.ErrTaskNotCompleted
	}
	if p.Claimed {
		return exceptions.ErrRewardAlreadyClaimed
	}
	return nil
}

func (p *TaskProgress) MarkClaimed() error {
	if err := p.CanClaim(); err != nil {
		return err
	}
	p.Claimed = true
	p.UpdatedAt = time.Now()
	return nil
}

type TaskProgress struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	UserID    string    `json:"user_id"`
	Progress  int       `json:"progress"`
	Completed bool      `json:"completed"`
	Claimed   bool      `json:"claimed"`
	UpdatedAt time.Time `json:"updated_at"`
}
