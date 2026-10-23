package main

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestBillingDeliveryOutlivesTheHttpContext(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(billingConfig)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)
		host.CallOnHttpRequestHeaders([][2]string{{":authority", "example.com"}, {":path", "/v1/chat/completions"}, {":method", "POST"}, {"x-request-id", "lifetime-test"}, {"x-mse-tenant", "tenant-a"}, {"x-mse-consumer", "consumer-a"}})
		host.CallOnHttpRequestBody([]byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`))
		host.CallOnHttpResponseHeaders([][2]string{{":status", "200"}, {"content-type", "application/json"}})
		host.CallOnHttpResponseBody([]byte(`{"model":"gpt-4","usage":{"prompt_tokens":4,"completion_tokens":6,"total_tokens":10}}`))
		require.Empty(t, host.GetRedisCalloutAttributes(), "delivery must be owned by the plugin, not the ending HTTP context")
		host.CompleteHttp()
		host.Tick()
		attrs := host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID)
		require.Len(t, attrs, 1)
		original := append([]byte(nil), attrs[0].Query...)
		host.CallOnRedisCallResponse(attrs[0].CalloutID, 1, nil)
		attrs = host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID)
		require.Len(t, attrs, 1, "one bounded transport retry survives HTTP deletion")
		require.Equal(t, original, attrs[0].Query, "event identity and payload stay immutable")
		host.CallOnRedisCallResponse(attrs[0].CalloutID, 1, nil)
		require.Empty(t, host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID))
		require.NotEmpty(t, host.GetWarnLogs(), "terminal delivery failure remains observable")
	})
}
