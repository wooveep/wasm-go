package wrapper

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackgroundQueueBoundsPendingAndInFlightWork(t *testing.T) {
	queue := &BackgroundQueue{}
	var complete func()
	require.NoError(t, queue.Enqueue(backgroundQueueMaxBytes, func(done func()) { complete = done }))
	require.Error(t, queue.Enqueue(1, func(func()) { t.Fatal("capacity must be enforced") }))
	queue.flush()
	require.Error(t, queue.Enqueue(1, func(func()) { t.Fatal("in-flight capacity must remain reserved") }))
	complete()
	complete()
	require.Zero(t, queue.count)
	require.Zero(t, queue.bytes)
	require.NoError(t, queue.Enqueue(1, func(done func()) { done() }))
	queue.flush()
	for i := 0; i < backgroundQueueMaxCount; i++ {
		require.NoError(t, queue.Enqueue(1, func(func()) {}))
	}
	require.Error(t, queue.Enqueue(0, func(func()) {}))
}
