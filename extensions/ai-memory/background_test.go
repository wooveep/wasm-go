package main

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

// Recent lookups belong to HTTP; source-event delivery belongs to the plugin.
func memoryRedisCallouts(host test.TestHost) []proxytest.RedisCalloutAttribute {
	host.Tick()
	return append(host.GetRedisCalloutAttributes(), host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID)...)
}

func memoryRedisRespond(host test.TestHost, status int32, response []byte) {
	attrs := memoryRedisCallouts(host)
	host.CallOnRedisCallResponse(attrs[0].CalloutID, status, response)
}

func TestMemoryEventDeliveryOutlivesResponseAndRetriesTheSamePayloadOnce(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host := startMemoryEventRequestWithBody(t, true, memoryEventRequestBody(t, "safe lifecycle fact", nil))
		callMemoryEventResponse(t, host, 200, memoryEventResponseBody(t, "recorded", "stop", 5, 4, 9))
		require.Empty(t, host.GetRedisCalloutAttributes())
		host.CompleteHttp()
		host.Tick()
		attrs := host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID)
		require.Len(t, attrs, 1)
		original := append([]byte(nil), attrs[0].Query...)
		host.CallOnRedisCallResponse(attrs[0].CalloutID, 1, nil)
		attrs = host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID)
		require.Len(t, attrs, 1)
		require.Equal(t, original, attrs[0].Query)
		host.CallOnRedisCallResponse(attrs[0].CalloutID, 0, test.CreateRedisRespString("1700000000000-1"))
		require.Empty(t, host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID))
	})
}
