package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	tasksv1 "task-manager/pkg/grpc/gen/tasks"
)

type stats struct {
	sent     uint64
	ok       uint64
	errCount uint64
	errCodes map[codes.Code]uint64
	mu       sync.Mutex
}

func main() {
	addr := flag.String("addr", "127.0.0.1:50051", "gRPC address")
	userID := flag.String("user", "", "user id (uuid)")
	taskID := flag.String("task", "", "task id (uuid)")
	workers := flag.Int("workers", 2, "number of concurrent workers")
	count := flag.Int("count", 2, "total requests (ignored if -forever)")
	forever := flag.Bool("forever", false, "run until interrupted")
	delay := flag.Duration("delay", 0, "delay between requests per worker (e.g. 10ms)")
	logEvery := flag.Int("log-every", 100, "log every N successes")
	verbose := flag.Bool("verbose", false, "log every request")
	checkDB := flag.Bool("check-db", false, "poll db for updated_at changes")
	poll := flag.Duration("poll", time.Second, "db poll interval (e.g. 200ms)")
	flag.Parse()

	if *userID == "" || *taskID == "" {
		fmt.Println("usage: go run ./scripts/claim_reward.go --task <uuid> --user <uuid> [--workers 2] [--count 100|--forever] [--delay 0ms] [--log-every 100] [--check-db] [--poll 1s]")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		cancel()
	}()

	conn, err := grpc.DialContext(ctx, *addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("grpc dial failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := tasksv1.NewTaskServiceClient(conn)

	var st stats
	st.errCodes = make(map[codes.Code]uint64)

	var watcher *dbWatcher
	if *checkDB {
		watcher, err = startDBWatcher(ctx, *userID, *taskID, *poll)
		if err != nil {
			fmt.Printf("db watcher failed: %v\n", err)
			os.Exit(1)
		}
	}

	run := func(id int) {
		for {
			if !*forever {
				n := atomic.AddUint64(&st.sent, 1)
				if n > uint64(*count) {
					return
				}
			} else {
				atomic.AddUint64(&st.sent, 1)
			}

			req := &tasksv1.ClaimRewardRequest{
				UserId: *userID,
				TaskId: *taskID,
			}
			_, err := client.ClaimReward(ctx, req)
			if err != nil {
				code := status.Code(err)
				atomic.AddUint64(&st.errCount, 1)
				st.mu.Lock()
				st.errCodes[code]++
				st.mu.Unlock()
				fmt.Printf("[W%d] error code=%s msg=%s\n", id, code.String(), err.Error())
			} else {
				n := atomic.AddUint64(&st.ok, 1)
				if *verbose || (n%uint64(*logEvery) == 0) {
					fmt.Printf("[W%d] ok\n", id)
				}
			}

			if *delay > 0 {
				select {
				case <-time.After(*delay):
				case <-ctx.Done():
					return
				}
			}

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			run(id + 1)
		}(i)
	}
	wg.Wait()

	if watcher != nil {
		watcher.Stop()
	}

	st.mu.Lock()
	fmt.Printf("summary sent=%d ok=%d errors=%d error_codes=%v\n", st.sent, st.ok, st.errCount, st.errCodes)
	st.mu.Unlock()
}

type dbWatcher struct {
	pool          *pgxpool.Pool
	userID        string
	taskID        string
	lastClaimed   bool
	lastUpdatedAt time.Time
	updateCount   int
	cancel        context.CancelFunc
	done          chan struct{}
}

func startDBWatcher(ctx context.Context, userID, taskID string, poll time.Duration) (*dbWatcher, error) {
	dsn := buildDSN()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	wctx, cancel := context.WithCancel(ctx)
	w := &dbWatcher{
		pool:   pool,
		userID: userID,
		taskID: taskID,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go w.loop(wctx, poll)
	return w, nil
}

func (w *dbWatcher) loop(ctx context.Context, poll time.Duration) {
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	defer close(w.done)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var claimed bool
			var updatedAt time.Time
			err := w.pool.QueryRow(ctx,
				`SELECT claimed, updated_at FROM task_progress WHERE user_id = $1 AND task_id = $2`,
				w.userID, w.taskID,
			).Scan(&claimed, &updatedAt)
			if err != nil {
				continue
			}
			if !w.lastUpdatedAt.IsZero() && updatedAt.After(w.lastUpdatedAt) {
				w.updateCount++
				fmt.Printf("db-watch change: updated_at changed (prev=%s now=%s)\n", w.lastUpdatedAt.UTC().Format(time.RFC3339Nano), updatedAt.UTC().Format(time.RFC3339Nano))
			}
			w.lastClaimed = claimed
			w.lastUpdatedAt = updatedAt
		}
	}
}

func (w *dbWatcher) Stop() {
	w.cancel()
	<-w.done
	w.pool.Close()
	fmt.Printf("db-watch summary claimed=%v updated_at=%s updates=%d\n",
		w.lastClaimed,
		w.lastUpdatedAt.UTC().Format(time.RFC3339Nano),
		w.updateCount,
	)
}

func buildDSN() string {
	db := getEnv("POSTGRES_DB", "task_manager")
	user := getEnv("POSTGRES_USER", "task_manager")
	pass := getEnv("POSTGRES_PASSWORD", "task_manager")
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5432")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, db)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
