package entities

import (
	"task-manager/internal/core/domain/exceptions"
	"time"
)

func (p *TaskProgress) AddProgress(amount int, target int) bool {
	if p.completed {
		return false
	}

	p.progress += amount
	if p.progress >= target {
		p.progress = target
		p.completed = true
		return true
	}
	return false
}

func (p *TaskProgress) CanClaim() error {
	if !p.completed {
		return exceptions.ErrTaskNotCompleted
	}
	if p.claimed {
		return exceptions.ErrRewardAlreadyClaimed
	}
	return nil
}

func (p *TaskProgress) MarkClaimed() error {
	if err := p.CanClaim(); err != nil {
		return err
	}
	p.claimed = true
	p.updatedAt = time.Now()
	return nil
}

type TaskProgress struct {
	id        string
	taskID    string
	userID    string
	progress  int
	completed bool
	claimed   bool
	updatedAt time.Time
}

func NewTaskProgress(taskID, userID string) *TaskProgress {
	return &TaskProgress{
		taskID: taskID,
		userID: userID,
	}
}

func NewTaskProgressFromData(id, taskID, userID string, progress int, completed, claimed bool, updatedAt time.Time) *TaskProgress {
	return &TaskProgress{
		id:        id,
		taskID:    taskID,
		userID:    userID,
		progress:  progress,
		completed: completed,
		claimed:   claimed,
		updatedAt: updatedAt,
	}
}

func (p *TaskProgress) ID() string {
	return p.id
}

func (p *TaskProgress) TaskID() string {
	return p.taskID
}

func (p *TaskProgress) UserID() string {
	return p.userID
}

func (p *TaskProgress) Progress() int {
	return p.progress
}

func (p *TaskProgress) Completed() bool {
	return p.completed
}

func (p *TaskProgress) Claimed() bool {
	return p.claimed
}

func (p *TaskProgress) UpdatedAt() time.Time {
	return p.updatedAt
}

func (p *TaskProgress) SetID(id string) {
	p.id = id
}

func (p *TaskProgress) SetUpdatedAt(at time.Time) {
	p.updatedAt = at
}
