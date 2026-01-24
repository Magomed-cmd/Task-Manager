package grpc

import (
	"context"
	"io"
	"time"

	"task-manager/internal/core/domain/entities"
	"task-manager/internal/core/ports"
	"task-manager/internal/mapper"
	tasksv1 "task-manager/pkg/grpc/gen/tasks"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type TaskServer struct {
	tasksv1.UnimplementedTaskServiceServer
	service ports.TaskUseCases
	log     *zap.Logger

	streamEventsIdleTimeout  time.Duration
	streamEventsBatchTimeout time.Duration
	subscribeInterval        time.Duration
	subscribeMaxPeriod       time.Duration
}

type streamStats struct {
	accepted int32
	rejected int32
	batches  int32
	events   int32
}

func NewTaskServer(
	service ports.TaskUseCases,
	log *zap.Logger,
	streamEventsIdleTimeout time.Duration,
	streamEventsBatchTimeout time.Duration,
	subscribeInterval time.Duration,
	subscribeMaxPeriod time.Duration,
) *TaskServer {
	if service == nil {
		log.Fatal("task service is nil")
	}
	if log == nil {
		panic("logger is nil")
	}
	server := &TaskServer{
		service:                  service,
		log:                      log,
		streamEventsIdleTimeout:  streamEventsIdleTimeout,
		streamEventsBatchTimeout: streamEventsBatchTimeout,
		subscribeInterval:        subscribeInterval,
		subscribeMaxPeriod:       subscribeMaxPeriod,
	}
	if server.streamEventsIdleTimeout <= 0 {
		log.Fatal("stream events idle timeout must be configured")
	}
	if server.streamEventsBatchTimeout <= 0 {
		log.Fatal("stream events batch timeout must be configured")
	}
	if server.subscribeInterval <= 0 {
		log.Fatal("subscribe progress interval must be configured")
	}
	if server.subscribeMaxPeriod <= 0 {
		log.Fatal("subscribe progress max period must be configured")
	}
	return server
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
	return &tasksv1.GetTaskResponse{Task: mapper.Task(task)}, nil
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
	return &tasksv1.ProcessEventResponse{Accepted: true}, nil
}

func (s *TaskServer) StreamEvents(stream tasksv1.TaskService_StreamEventsServer) error {
	streamID := nextStreamID()
	remote := ""
	if p, ok := peer.FromContext(stream.Context()); ok && p.Addr != nil {
		remote = p.Addr.String()
	}
	startedAt := time.Now()

	s.log.Info("grpc: stream events started", zap.Uint64("stream_id", streamID), zap.String("remote", remote))
	stats := streamStats{}

	idleTimer := time.NewTimer(s.streamEventsIdleTimeout)
	defer idleTimer.Stop()
	recvCh := startRecvLoop(stream)
	var recvErr error

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-idleTimer.C:
			return s.finishStream(errStreamIdleTimeout, stream, stats.accepted, stats.rejected, stats.batches, stats.events, startedAt, streamID, remote)
		case res, ok := <-recvCh:
			if !ok {
				err := recvErr
				if err == nil {
					if ctxErr := stream.Context().Err(); ctxErr != nil {
						err = ctxErr
					} else {
						err = io.EOF
					}
				}
				return s.finishStream(err, stream, stats.accepted, stats.rejected, stats.batches, stats.events, startedAt, streamID, remote)
			}
			if res.err != nil {
				recvErr = res.err
				continue
			}
			req := res.req
			resetTimer(idleTimer, s.streamEventsIdleTimeout)
			stats.batches++
			stats.events += int32(len(req.GetEvents()))

			domainEvents, rejectedInBatch, err := s.mapEvents(req)
			stats.rejected += rejectedInBatch
			if err != nil {
				return err
			}
			if len(domainEvents) == 0 {
				continue
			}

			batchAccepted, batchRejected, err := s.processBatch(stream.Context(), domainEvents)
			if err != nil {
				s.log.Warn("grpc: stream events processing failed", zap.Error(err))
				return mapper.Error(err)
			}

			stats.accepted += batchAccepted
			stats.rejected += batchRejected
		}
	}
}

func (s *TaskServer) mapEvents(req *tasksv1.StreamEventsRequest) ([]*entities.TaskEvent, int32, error) {
	if len(req.GetEvents()) == 0 {
		s.log.Warn("grpc: stream events validation failed", zap.Error(status.Error(codes.InvalidArgument, "events are required")))
		return nil, 0, status.Error(codes.InvalidArgument, "events are required")
	}

	var rejected int32
	domainEvents := make([]*entities.TaskEvent, 0, len(req.GetEvents()))

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
		domainEvents = append(domainEvents, domainEvent)
	}

	return domainEvents, rejected, nil
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

	ticker := time.NewTicker(s.subscribeInterval)
	defer ticker.Stop()
	timeout := time.NewTimer(s.subscribeMaxPeriod)
	defer timeout.Stop()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-timeout.C:
			s.log.Warn("grpc: subscribe progress timeout", zap.String("user_id", req.GetUserId()))
			return status.Error(codes.DeadlineExceeded, "stream timeout")
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
