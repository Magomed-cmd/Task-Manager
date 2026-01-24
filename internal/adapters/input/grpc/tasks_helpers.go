package grpc

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"time"

	"task-manager/internal/core/domain/entities"
	tasksv1 "task-manager/pkg/grpc/gen/tasks"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var errStreamIdleTimeout = errors.New("stream idle timeout")
var streamSeq uint64

type streamRecvResult struct {
	req *tasksv1.StreamEventsRequest
	err error
}

func nextStreamID() uint64 {
	return atomic.AddUint64(&streamSeq, 1)
}

func startRecvLoop(stream tasksv1.TaskService_StreamEventsServer) <-chan streamRecvResult {
	ch := make(chan streamRecvResult, 1)

	go func() {
		defer close(ch)
		for {
			req, err := stream.Recv()
			if err != nil {
				ch <- streamRecvResult{err: err}
				return
			}
			select {
			case ch <- streamRecvResult{req: req}:
			case <-stream.Context().Done():
				return
			}
		}
	}()

	return ch
}

func (s *TaskServer) finishStream(
	err error,
	stream tasksv1.TaskService_StreamEventsServer,
	accepted, rejected, batches, events int32,
	startedAt time.Time,
	streamID uint64,
	remote string,
) error {
	fields := []zap.Field{
		zap.Uint64("stream_id", streamID),
		zap.String("remote", remote),
		zap.Int32("batches", batches),
		zap.Int32("events", events),
		zap.Int32("accepted", accepted),
		zap.Int32("rejected", rejected),
		zap.Duration("elapsed", time.Since(startedAt)),
	}

	switch {
	case err == nil:
		return nil
	case err == errStreamIdleTimeout:
		s.log.Warn("grpc: stream events idle timeout", fields...)
		return status.Error(codes.DeadlineExceeded, err.Error())
	case err == io.EOF:
		s.log.Info("grpc: stream events done", fields...)
		return stream.SendAndClose(&tasksv1.StreamEventsResponse{
			Accepted: accepted,
			Rejected: rejected,
		})
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	default:
		fields = append(fields, zap.Error(err))
		s.log.Error("grpc: stream events recv failed", fields...)
		return status.Error(codes.Internal, err.Error())
	}
}

func (s *TaskServer) processBatch(ctx context.Context, events []*entities.TaskEvent) (int32, int32, error) {
	batchCtx, cancel := context.WithTimeout(ctx, s.streamEventsBatchTimeout)
	defer cancel()

	return s.service.ProcessEvents(batchCtx, events)
}

func resetTimer(timer *time.Timer, d time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)
}
