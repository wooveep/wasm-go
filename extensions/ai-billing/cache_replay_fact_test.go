package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const trustedUpstreamInvokedFact = "upstream_invoked"

type cacheReplayBillingContext struct {
	mockBillingHttpContext
	userAttributes map[string]interface{}
}

func (c *cacheReplayBillingContext) GetUserAttribute(key string) interface{} {
	return c.userAttributes[key]
}

func (c *cacheReplayBillingContext) GetUserAttributeMap() map[string]interface{} {
	return c.userAttributes
}

func TestBuildBillingEvent_TrustedCacheReplayFacts(t *testing.T) {
	ctx := &cacheReplayBillingContext{
		mockBillingHttpContext: mockBillingHttpContext{values: map[string]interface{}{
			ctxEventID:        "event-cache-hit",
			ctxIdempotencyKey: "event-cache-hit",
			ctxRequestID:      "req-cache-hit",
			ctxRequestPath:    "/v1/chat/completions",
			ctxTenant:         "tenant-a",
			ctxConsumer:       "consumer-a",
			ctxRoute:          "route-a",
			ctxCluster:        "llm-openai.dns",
			ctxModel:          "qwen-turbo",
			ctxStatusCode:     200,
			ctxInputToken:     int64(7),
			ctxOutputToken:    int64(3),
			ctxTotalToken:     int64(10),
			ctxUsageSource:    usageSourceProvider,
		}},
		userAttributes: map[string]interface{}{
			"cache_status":             "hit",
			trustedUpstreamInvokedFact: false,
		},
	}

	body, err := json.Marshal(buildBillingEvent(ctx, BillingConfig{QuotaScope: "global"}, false))
	require.NoError(t, err)

	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &event))
	require.Contains(t, event, trustedUpstreamInvokedFact)
	require.Equal(t, false, event[trustedUpstreamInvokedFact])
}
