package mapper

import (
	"errors"
	"time"

	"task-manager/internal/core/domain/entities"
	"task-manager/internal/core/domain/exceptions"
	tasksv1 "task-manager/pkg/grpc/gen/tasks"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Task(task *entities.Task) *tasksv1.Task {
	if task == nil {
		return nil
	}
	return &tasksv1.Task{
		Id:          task.ID(),
		Title:       task.Title(),
		Description: task.Description(),
		Type:        string(task.Type()),
		Target:      int32(task.Target()),
		RewardJson:  task.Reward(),
		IsActive:    task.IsActive(),
		CreatedAt:   timestamp(task.CreatedAt()),
	}
}

func Progress(progress *entities.TaskProgress) *tasksv1.TaskProgress {
	if progress == nil {
		return nil
	}
	return &tasksv1.TaskProgress{
		Id:        progress.ID(),
		TaskId:    progress.TaskID(),
		UserId:    progress.UserID(),
		Progress:  int32(progress.Progress()),
		Completed: progress.Completed(),
		Claimed:   progress.Claimed(),
		UpdatedAt: timestamp(progress.UpdatedAt()),
	}
}

func TasksWithProgress(tasks []*entities.Task, progress []*entities.TaskProgress) *tasksv1.GetTasksWithProgressResponse {
	resp := &tasksv1.GetTasksWithProgressResponse{
		Tasks:    make([]*tasksv1.Task, 0, len(tasks)),
		Progress: make([]*tasksv1.TaskProgress, 0, len(progress)),
	}
	for _, task := range tasks {
		resp.Tasks = append(resp.Tasks, Task(task))
	}
	for _, entry := range progress {
		resp.Progress = append(resp.Progress, Progress(entry))
	}
	return resp
}

func Event(event *tasksv1.TaskEvent) (*entities.TaskEvent, error) {
	if event == nil {
		return nil, exceptions.ErrEventNil
	}

	var payload *entities.ProgressPayload
	if event.Payload != nil {
		payload = &entities.ProgressPayload{
			TaskID: event.Payload.GetTaskId(),
			Amount: int(event.Payload.GetAmount()),
		}
	}

	createdAt := time.Time{}
	if event.CreatedAt != nil {
		createdAt = event.CreatedAt.AsTime()
	}

	return entities.NewTaskEvent(
		event.GetEventId(),
		event.GetUserId(),
		event.GetRoomId(),
		entities.TaskEventType(event.GetType()),
		payload,
		createdAt,
	)
}

func Error(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, exceptions.ErrTaskNotFound),
		errors.Is(err, exceptions.ErrProgressNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, exceptions.ErrTaskNotCompleted),
		errors.Is(err, exceptions.ErrRewardAlreadyClaimed),
		errors.Is(err, exceptions.ErrTaskInactive):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, exceptions.ErrEventNil),
		errors.Is(err, exceptions.ErrEventIDRequired),
		errors.Is(err, exceptions.ErrEventUserIDRequired),
		errors.Is(err, exceptions.ErrEventTypeRequired),
		errors.Is(err, exceptions.ErrEventPayloadInvalid),
		errors.Is(err, exceptions.ErrEventTaskIDRequired),
		errors.Is(err, exceptions.ErrEventAmountInvalid),
		errors.Is(err, exceptions.ErrUnsupportedEventType):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
