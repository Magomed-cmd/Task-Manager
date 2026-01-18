package exceptions

import "errors"

var (
	ErrTaskNotCompleted     = errors.New("task is not completed yet")
	ErrRewardAlreadyClaimed = errors.New("reward already claimed")
	ErrTaskNotFound         = errors.New("task not found")
	ErrProgressNotFound     = errors.New("progress not found")
)
