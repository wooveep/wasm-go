package config

import (
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

// DispatchEvent preserves the event payload across one transport retry and runs
// outside the ending HTTP context. Redis server errors are never retried here.
func (c PluginConfig) DispatchEvent(command []string, callback wrapper.RedisResponseCallback) error {
	dispatch := func(done func()) {
		var attempt func(bool)
		attempt = func(retryTransport bool) {
			err := c.eventRedis.Command(command, func(value resp.Value) {
				if value.Error() != nil && retryTransport && value.Error().Error() == "memory event Redis call failed" {
					attempt(false)
					return
				}
				callback(value)
				done()
			})
			if err != nil {
				// Keep failure details independent of command contents and credentials.
				callback(resp.ErrorValue(err))
				done()
			}
		}
		attempt(true)
	}
	if c.eventQueue == nil {
		dispatch(func() {})
		return nil
	}
	size := 0
	for _, value := range command {
		size += len(value)
	}
	return c.eventQueue.Enqueue(size, dispatch)
}
