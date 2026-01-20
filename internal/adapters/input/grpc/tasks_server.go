package grpc

import (
	"context"
	"io"
	"time"

	"task-manager/internal/core/ports"
	"task-manager/internal/mapper"
	tasksv1 "task-manager/pkg/grpc/gen/tasks"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var progressStreamInterval = 2 * time.Second

type TaskServer struct {
	tasksv1.UnimplementedTaskServiceServer
	service ports.TaskUseCases
	log     *zap.Logger
}

func NewTaskServer(service ports.TaskUseCases, log *zap.Logger) *TaskServer {
	if service == nil {
		log.Fatal("task service is nil")
	}
	if log == nil {
		panic("logger is nil")
	}
	return &TaskServer{
		service: service,
		log:     log,
	}
}

func (s *TaskServer) GetTasksWithProgress(ctx context.Context, req *tasksv1.GetTasksWithProgressRequest) (*tasksv1.GetTasksWithProgressResponse, error) {
	s.log.Info("grpc: get tasks with progress", zap.String("user_id", req.GetUserId()))
	if err := req.ValidateAll(); err != nil {
		s.log.Warn("grpc: get tasks with progress validation failed", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	tasks, progress, err := s.service.GetTasksWithProgress(ctx, req.GetUserId())
	if err != nil {
		s.log.Error("grpc: get tasks with progress failed", zap.Error(err))
		return nil, mapper.Error(err)
	}

	resp := mapper.TasksWithProgress(tasks, progress)
	s.log.Info("grpc: get tasks with progress done", zap.Int("tasks", len(resp.Tasks)), zap.Int("progress", len(resp.Progress)))
	return resp, nil
}

func (s *TaskServer) GetTask(ctx context.Context, req *tasksv1.GetTaskRequest) (*tasksv1.GetTaskResponse, error) {
	s.log.Info("grpc: get task", zap.String("task_id", req.GetTaskId()))
	if err := req.ValidateAll(); err != nil {
		s.log.Warn("grpc: get task validation failed", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	task, err := s.service.GetTask(ctx, req.GetTaskId())
	if err != nil {
		s.log.Error("grpc: get task failed", zap.Error(err))
		return nil, mapper.Error(err)
	}

	s.log.Info("grpc: get task done", zap.String("task_id", req.GetTaskId()))
	return &tasksv1.GetTaskResponse{
		Task: mapper.Task(task),
	}, nil
}

func (s *TaskServer) ProcessEvent(ctx context.Context, req *tasksv1.ProcessEventRequest) (*tasksv1.ProcessEventResponse, error) {
	s.log.Info("grpc: process event", zap.String("event_id", req.GetEvent().GetEventId()), zap.String("event_type", req.GetEvent().GetType()))
	if err := req.ValidateAll(); err != nil {
		s.log.Warn("grpc: process event validation failed", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	event, err := mapper.Event(req.GetEvent())
	if err != nil {
		s.log.Warn("grpc: process event mapping failed", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.service.ProcessEvent(ctx, event); err != nil {
		s.log.Error("grpc: process event failed", zap.Error(err))
		return nil, mapper.Error(err)
	}

	s.log.Info("grpc: process event done", zap.String("event_id", event.EventID()))
	return &tasksv1.ProcessEventResponse{
		Accepted: true,
	}, nil
}

func (s *TaskServer) StreamEvents(stream tasksv1.TaskService_StreamEventsServer) error {
	s.log.Info("grpc: stream events started")
	var accepted int32
	var rejected int32

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			s.log.Info("grpc: stream events done", zap.Int32("accepted", accepted), zap.Int32("rejected", rejected))
			return stream.SendAndClose(&tasksv1.StreamEventsResponse{
				Accepted: accepted,
				Rejected: rejected,
			})
		}
		if err != nil {
			s.log.Error("grpc: stream events recv failed", zap.Error(err))
			return status.Error(codes.Internal, err.Error())
		}

		if len(req.GetEvents()) == 0 {
			s.log.Warn("grpc: stream events validation failed", zap.Error(status.Error(codes.InvalidArgument, "events are required")))
			continue
		}

		for _, event := range req.GetEvents() {
			if event == nil {
				rejected++
				s.log.Warn("grpc: stream events validation failed", zap.Error(status.Error(codes.InvalidArgument, "event is required")))
				continue
			}
			if err := event.ValidateAll(); err != nil {
				rejected++
				s.log.Warn("grpc: stream events validation failed", zap.Error(err))
				continue
			}

			domainEvent, err := mapper.Event(event)
			if err != nil {
				rejected++
				s.log.Warn("grpc: stream events mapping failed", zap.Error(err))
				continue
			}

			if err := s.service.ProcessEvent(stream.Context(), domainEvent); err != nil {
				rejected++
				s.log.Warn("grpc: stream events processing failed", zap.Error(err))
				continue
			}

			accepted++
		}
	}
}

func (s *TaskServer) SubscribeProgress(req *tasksv1.SubscribeProgressRequest, stream tasksv1.TaskService_SubscribeProgressServer) error {
	
	s.log.Info("grpc: subscribe progress", zap.String("user_id", req.GetUserId()))
	if err := req.ValidateAll(); err != nil {
		s.log.Warn("grpc: subscribe progress validation failed", zap.Error(err))
		return status.Error(codes.InvalidArgument, err.Error())
	}

	send := func() error {
		tasks, progress, err := s.service.GetTasksWithProgress(stream.Context(), req.GetUserId())
		if err != nil {
			s.log.Error("grpc: subscribe progress failed", zap.Error(err))
			return mapper.Error(err)
		}
		if err := stream.Send(mapper.TasksWithProgress(tasks, progress)); err != nil {
			return err
		}
		return nil
	}

	if err := send(); err != nil {
		return err
	}

	ticker := time.NewTicker(progressStreamInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-ticker.C:
			if err := send(); err != nil {
				return err
			}
		}
	}
}

func (s *TaskServer) ClaimReward(ctx context.Context, req *tasksv1.ClaimRewardRequest) (*tasksv1.ClaimRewardResponse, error) {
	s.log.Info("grpc: claim reward", zap.String("user_id", req.GetUserId()), zap.String("task_id", req.GetTaskId()))
	if err := req.ValidateAll(); err != nil {
		s.log.Warn("grpc: claim reward validation failed", zap.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := s.service.ClaimReward(ctx, req.GetUserId(), req.GetTaskId()); err != nil {
		s.log.Error("grpc: claim reward failed", zap.Error(err))
		return nil, mapper.Error(err)
	}

	s.log.Info("grpc: claim reward done", zap.String("user_id", req.GetUserId()), zap.String("task_id", req.GetTaskId()))
	return &tasksv1.ClaimRewardResponse{}, nil
}
