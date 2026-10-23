package wrapper

import "errors"

// BackgroundQueue runs bounded event deliveries on the plugin's tick context.
// Unlike HTTP-context callouts, these callbacks survive completion of a response.
// This is an in-memory queue, not a durable outbox; plugin shutdown can lose work.
type BackgroundQueue struct {
	pending []func()
	bytes   int
	count   int
}

const backgroundQueueMaxBytes = 256 * 1024 * 1024
const backgroundQueueMaxCount = 1024

// NewBackgroundQueue must be called while parsing plugin configuration.
func NewBackgroundQueue() *BackgroundQueue {
	queue := &BackgroundQueue{}
	RegisterTickFunc(0, queue.flush)
	return queue
}

// Enqueue accounts for queued and in-flight bytes until dispatch calls done.
// Dispatch and its completion must not access the originating HTTP context.
func (queue *BackgroundQueue) Enqueue(size int, dispatch func(done func())) error {
	if size < 0 || size > backgroundQueueMaxBytes-queue.bytes || queue.count >= backgroundQueueMaxCount {
		return errors.New("background event delivery capacity exhausted")
	}
	queue.bytes += size
	queue.count++
	queue.pending = append(queue.pending, func() {
		completed := false
		dispatch(func() {
			if !completed {
				completed = true
				queue.bytes -= size
				queue.count--
			}
		})
	})
	return nil
}

func (queue *BackgroundQueue) flush() {
	tasks := queue.pending
	queue.pending = nil
	for _, task := range tasks {
		task()
	}
}
