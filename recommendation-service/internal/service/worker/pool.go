package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

var (
	poolJobsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "dating_recommendation_worker_jobs_processed_total",
		Help: "Total number of worker pool jobs processed by job type.",
	}, []string{"job_type"})

	poolJobsDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "dating_recommendation_worker_jobs_dropped_total",
		Help: "Total number of worker pool jobs dropped due to full queue or dedup.",
	})

	poolQueueLength = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "dating_recommendation_worker_queue_length",
		Help: "Current number of jobs waiting in the worker pool queue.",
	})
)

// JobType identifies what kind of recalculation a Job requests.
type JobType uint8

const (
	// JobPrimaryRecalc triggers a primary-rating recalculation for a single user.
	// Fired when a profile.updated event arrives.
	JobPrimaryRecalc JobType = iota

	// JobBehavioralRecalc triggers a behavioral-rating recalculation for a single user.
	// Fired when an interaction.liked event arrives (for the target user).
	JobBehavioralRecalc

	// JobFullRecalc triggers a full (primary + behavioral + combined) recalculation
	// for a single user. Fired on TriggerRecalculation RPC or the periodic ticker.
	JobFullRecalc

	// JobGlobalRecalc recalculates all users — fired by the periodic ticker.
	// The handler fetches every user_id from Postgres and fans out JobFullRecalc jobs.
	JobGlobalRecalc
)

// String returns a human-readable label used in Prometheus metric labels.
func (t JobType) String() string {
	switch t {
	case JobPrimaryRecalc:
		return "primary"
	case JobBehavioralRecalc:
		return "behavioral"
	case JobFullRecalc:
		return "full"
	case JobGlobalRecalc:
		return "global"
	default:
		return "unknown"
	}
}

// Job is a unit of work placed on the pool's queue.
type Job struct {
	Type   JobType
	UserID int64 // 0 for JobGlobalRecalc
}

// Handler is the function the pool calls for every dequeued Job.
// It must be safe to call from multiple goroutines concurrently.
type Handler func(ctx context.Context, job Job)

// Pool is a bounded goroutine pool that drains a buffered Job channel.
// Design decisions:
//   - Fixed worker count prevents unbounded goroutine growth under load.
//   - Buffered channel decouples producers (event handlers / RPC) from workers.
//   - Duplicate suppression via a sharded in-flight set avoids hammering Postgres
//     when the same user receives many rapid events.
//   - Graceful shutdown: Stop() drains in-flight jobs before returning.
type Pool struct {
	handler    Handler
	handlerMu  sync.RWMutex
	jobs       chan Job
	workerN    int
	wg         sync.WaitGroup
	once       sync.Once
	cancelFunc context.CancelFunc

	// Metrics — read with atomic, written inside workers.
	processed atomic.Int64
	dropped   atomic.Int64

	// in-flight dedup: protects against scheduling the same (type, userID) pair
	// multiple times when the queue is already backed up.
	inflight sync.Map // key: inflight key string → struct{}{}
}

// New creates a Pool with workerN workers and a job queue of queueSize.
// handler may be nil at construction time and set later via SetHandler before
// Start is called.
func New(workerN, queueSize int, handler Handler) *Pool {
	if workerN <= 0 {
		workerN = 4
	}
	if queueSize <= 0 {
		queueSize = 256
	}
	return &Pool{
		handler: handler,
		jobs:    make(chan Job, queueSize),
		workerN: workerN,
	}
}

// SetHandler registers (or replaces) the job handler.
// Must be called before Start; safe to call from a single goroutine during
// initialisation without holding the lock.
func (p *Pool) SetHandler(h Handler) {
	p.handlerMu.Lock()
	p.handler = h
	p.handlerMu.Unlock()
}

// Start launches workerN goroutines and binds their lifetime to ctx.
// Calling Start more than once is a no-op.
func (p *Pool) Start(ctx context.Context) {
	p.once.Do(func() {
		childCtx, cancel := context.WithCancel(ctx)
		p.cancelFunc = cancel

		for i := 0; i < p.workerN; i++ {
			p.wg.Add(1)
			go p.run(childCtx)
		}

		logrus.WithField("workers", p.workerN).
			WithField("queue_size", cap(p.jobs)).
			Info("worker pool started")
	})
}

// Stop signals all workers to stop and blocks until every in-flight job has
// finished. Jobs that are still in the queue but not yet picked up are discarded.
func (p *Pool) Stop() {
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
	p.wg.Wait()
	logrus.WithField("processed", p.processed.Load()).
		WithField("dropped", p.dropped.Load()).
		Info("worker pool stopped")
}

// Enqueue attempts to add job to the queue without blocking.
// If the queue is full the job is dropped (counted in Dropped()) and the caller
// is not blocked — this prevents a slow Postgres from stalling the gRPC path.
// Duplicate jobs (same type + userID already queued) are also silently dropped.
func (p *Pool) Enqueue(job Job) bool {
	// Deduplicate: skip if an identical job is already in-flight.
	key := inflightKey(job)
	if key != "" {
		if _, loaded := p.inflight.LoadOrStore(key, struct{}{}); loaded {
			return false // already queued, no need to schedule again
		}
	}

	select {
	case p.jobs <- job:
		poolQueueLength.Inc()
		return true
	default:
		// Queue full — remove the in-flight marker we just set.
		if key != "" {
			p.inflight.Delete(key)
		}
		p.dropped.Add(1)
		poolJobsDropped.Inc()
		logrus.WithField("job_type", job.Type).
			WithField("user_id", job.UserID).
			Warn("worker pool queue full — job dropped")
		return false
	}
}

// EnqueueWait is like Enqueue but blocks until there is room in the queue or
// ctx is cancelled.  Use this for high-priority paths (e.g. TriggerRecalculation RPC).
func (p *Pool) EnqueueWait(ctx context.Context, job Job) error {
	key := inflightKey(job)
	if key != "" {
		if _, loaded := p.inflight.LoadOrStore(key, struct{}{}); loaded {
			return nil // already queued
		}
	}

	select {
	case p.jobs <- job:
		poolQueueLength.Inc()
		return nil
	case <-ctx.Done():
		if key != "" {
			p.inflight.Delete(key)
		}
		return ctx.Err()
	}
}

// Processed returns the total number of jobs successfully handled.
func (p *Pool) Processed() int64 { return p.processed.Load() }

// Dropped returns the total number of jobs dropped due to a full queue.
func (p *Pool) Dropped() int64 { return p.dropped.Load() }

// QueueLen returns the current number of jobs waiting to be picked up.
func (p *Pool) QueueLen() int { return len(p.jobs) }

// run is the worker goroutine body.
func (p *Pool) run(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case job, ok := <-p.jobs:
			if !ok {
				return
			}
			p.execute(ctx, job)

		case <-ctx.Done():
			// Drain whatever is left in the channel before exiting so that
			// jobs enqueued just before Stop() are not silently abandoned.
			for {
				select {
				case job, ok := <-p.jobs:
					if !ok {
						return
					}
					p.execute(ctx, job)
				default:
					return
				}
			}
		}
	}
}

// execute calls the handler and handles panics + timing.
func (p *Pool) execute(ctx context.Context, job Job) {
	p.handlerMu.RLock()
	h := p.handler
	p.handlerMu.RUnlock()
	if h == nil {
		logrus.WithField("job_type", job.Type).Warn("worker pool has no handler — job skipped")
		return
	}
	key := inflightKey(job)
	defer func() {
		// Always release the dedup marker so the same job can be scheduled again
		// once this run is complete.
		if key != "" {
			p.inflight.Delete(key)
		}

		if r := recover(); r != nil {
			logrus.WithField("job_type", job.Type).
				WithField("user_id", job.UserID).
				WithField("panic", r).
				Error("worker recovered from panic")
		}
	}()

	poolQueueLength.Dec()
	start := time.Now()
	h(ctx, job)
	elapsed := time.Since(start)

	p.processed.Add(1)
	poolJobsProcessed.WithLabelValues(job.Type.String()).Inc()
	logrus.WithField("job_type", job.Type).
		WithField("user_id", job.UserID).
		WithField("elapsed_ms", elapsed.Milliseconds()).
		Debug("job processed")
}

// inflightKey builds a dedup key for a job.
// Returns "" for jobs that should never be deduplicated (e.g. JobGlobalRecalc).
func inflightKey(job Job) string {
	if job.Type == JobGlobalRecalc {
		return "" // allow multiple global recalcs to queue up
	}
	// e.g. "1:42" means JobPrimaryRecalc for user 42
	return string(rune('0'+job.Type)) + ":" + int64ToString(job.UserID)
}

// int64ToString is a zero-allocation int64 → string conversion.
func int64ToString(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
