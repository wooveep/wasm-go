package main

import (
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/proxytest"
	"github.com/higress-group/wasm-go/pkg/test"
)

// Advance the real plugin tick and inspect the context that owns event delivery.
func billingRedisCallouts(host test.TestHost) []proxytest.RedisCalloutAttribute {
	host.Tick()
	return host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID)
}

func billingRedisRespond(host test.TestHost, status int32, response []byte) {
	attrs := host.GetRedisCalloutAttributesFromContext(proxytest.PluginContextID)
	host.CallOnRedisCallResponse(attrs[0].CalloutID, status, response)
}
