// Package scheduler runs the work that must not happen inside an upload
// request: AVIF transcoding (an order of magnitude slower than WebP) and
// replication to a backup bucket. Jobs are dispatched to handlers registered
// by kind, so the queue itself stays free of image/storage dependencies.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	maxAttempts = 3
	jobTimeout  = 5 * time.Minute
	queueBuffer = 1024
	retryDelay  = 5 * time.Second
)

type JobKind string

const (
	// JobTranscode fills in derivatives the upload path deferred (AVIF).
	JobTranscode JobKind = "transcode"
	// JobBackup replicates an image's objects to the backup profile.
	JobBackup JobKind = "backup"
	// JobPurge removes objects belonging to a soft-deleted image.
	JobPurge JobKind = "purge"
)

type Job struct {
	Kind    JobKind
	ImageID uuid.UUID
	Attempt int
}

// Handler processes one job. Returning an error schedules a retry until
// maxAttempts is reached.
type Handler func(ctx context.Context, job Job) error

type Queue struct {
	db   *gorm.DB
	jobs chan Job

	mu       sync.RWMutex
	handlers map[JobKind]Handler
}

func New(db *gorm.DB) *Queue {
	return &Queue{
		db:       db,
		jobs:     make(chan Job, queueBuffer),
		handlers: map[JobKind]Handler{},
	}
}

// Register binds a handler to a job kind. Call before Start.
func (q *Queue) Register(kind JobKind, h Handler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[kind] = h
}

// Submit enqueues a job. The queue is lossy by design: if it's full we log and
// drop rather than block an HTTP handler. Dropped work is recoverable — the
// reconciler at startup re-enqueues anything left pending in the DB.
func (q *Queue) Submit(kind JobKind, imageID uuid.UUID) {
	select {
	case q.jobs <- Job{Kind: kind, ImageID: imageID}:
	default:
		log.Printf("scheduler: queue full, dropped %s job for image %s", kind, imageID)
	}
}

// SubmitWait enqueues without dropping, blocking until there is room or ctx
// ends. Submit's drop-on-full behaviour is right for a single upload — losing
// one AVIF transcode is survivable — but a bulk delete that drops purge jobs
// leaks the objects forever, since nothing sweeps for them afterwards.
func (q *Queue) SubmitWait(ctx context.Context, kind JobKind, imageID uuid.UUID) error {
	select {
	case q.jobs <- Job{Kind: kind, ImageID: imageID}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *Queue) Start(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		go q.worker(ctx, i)
	}
	q.mu.RLock()
	kinds := len(q.handlers)
	q.mu.RUnlock()
	log.Printf("scheduler: %d worker(s) started, %d job kind(s) registered", workers, kinds)
}

func (q *Queue) worker(ctx context.Context, idx int) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-q.jobs:
			if err := q.process(ctx, job); err != nil {
				log.Printf("scheduler: worker[%d] %s job for image %s failed: %v", idx, job.Kind, job.ImageID, err)
			}
		}
	}
}

func (q *Queue) process(ctx context.Context, job Job) error {
	q.mu.RLock()
	h, ok := q.handlers[job.Kind]
	q.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no handler registered for job kind %q", job.Kind)
	}

	jobCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	err := h(jobCtx, job)
	if err == nil {
		return nil
	}
	if job.Attempt+1 < maxAttempts {
		job.Attempt++
		go func() {
			select {
			case <-ctx.Done():
			case <-time.After(retryDelay):
				select {
				case q.jobs <- job:
				default:
					log.Printf("scheduler: queue full, dropped retry of %s for %s", job.Kind, job.ImageID)
				}
			}
		}()
		return nil
	}
	return fmt.Errorf("gave up after %d attempts: %w", maxAttempts, err)
}

// Depth reports how many jobs are waiting — surfaced on the admin dashboard so
// a backlog is visible before it turns into a complaint.
func (q *Queue) Depth() int { return len(q.jobs) }
