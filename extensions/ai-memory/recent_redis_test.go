package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

const memoryRecentRedisUpstream = "outbound|6379||redis.recent.svc.cluster.local"

func TestMemoryRecentRedis(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("hit inserts safe recent messages after memory message", func(t *testing.T) {
			host := startMemoryRecentRedisRequest(t)
			callMemoryRecentRedisResponse(t, host, 0, test.CreateRedisRespString(validMemoryRecentRecord(t, nil)))

			host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{
				"role":    "system",
				"content": "Console memory context",
			}, nil))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			messages := requireMemoryRequestMessages(t, host)
			require.Len(t, messages, 4)
			requireMemoryMessage(t, messages[0], "system", "Console memory context")
			requireMemoryMessage(t, messages[1], "user", "recent safe user")
			requireMemoryMessage(t, messages[2], "assistant", "recent safe assistant")
			requireMemoryMessage(t, messages[3], "user", "current question")
		})

		t.Run("miss continues with assemble-only memory", func(t *testing.T) {
			host := startMemoryRecentRedisRequest(t)
			callMemoryRecentRedisResponse(t, host, 0, test.CreateRedisRespNull())
			requireMemoryRecentAssembleOnly(t, host)
		})

		tests := []struct {
			name      string
			status    int32
			redisResp []byte
		}{
			{
				name:      "invalid JSON",
				redisResp: test.CreateRedisRespString(`{not-json`),
			},
			{
				name:      "unsupported schema version",
				redisResp: test.CreateRedisRespString(validMemoryRecentRecord(t, map[string]interface{}{"schema_version": 2})),
			},
			{
				name:      "tenant mismatch",
				redisResp: test.CreateRedisRespString(validMemoryRecentRecord(t, map[string]interface{}{"tenant": "tenant-b"})),
			},
			{
				name:      "consumer mismatch",
				redisResp: test.CreateRedisRespString(validMemoryRecentRecord(t, map[string]interface{}{"consumer": "consumer-b"})),
			},
			{
				name:      "policy mismatch",
				redisResp: test.CreateRedisRespString(validMemoryRecentRecord(t, map[string]interface{}{"policy_version": "memory-policy-v2"})),
			},
			{
				name:      "expiration",
				redisResp: test.CreateRedisRespString(validMemoryRecentRecord(t, map[string]interface{}{"expires_at_ms": time.Now().Add(-time.Minute).UnixMilli()})),
			},
			{
				name: "unsupported roles",
				redisResp: test.CreateRedisRespString(validMemoryRecentRecord(t, map[string]interface{}{
					"messages": []map[string]interface{}{
						{"role": "user", "content": "valid recent user should be rejected with the whole record"},
						{"role": "tool", "content": "tool output must not be injected as chat memory"},
						{"role": "assistant", "content": "valid recent assistant should be rejected with the whole record"},
					},
				})),
			},
			{
				name: "oversized values",
				redisResp: test.CreateRedisRespString(validMemoryRecentRecord(t, map[string]interface{}{
					"messages": []map[string]interface{}{
						{"role": "user", "content": "valid recent user should be rejected with the whole record"},
						{"role": "user", "content": strings.Repeat("x", 70_000)},
						{"role": "assistant", "content": "valid recent assistant should be rejected with the whole record"},
					},
				})),
			},
			{
				name:      "RESP error failure",
				redisResp: test.CreateRedisRespError("temporary"),
			},
			{
				name:   "callout timeout status failure",
				status: 1,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name+" fails open without injecting recent messages", func(t *testing.T) {
				host := startMemoryRecentRedisRequest(t)
				callMemoryRecentRedisResponse(t, host, tt.status, tt.redisResp)
				requireMemoryRecentAssembleOnly(t, host)
			})
		}
	})
}

func startMemoryRecentRedisRequest(t *testing.T) test.TestHost {
	t.Helper()
	host, status := newMemoryConfigTestHost(memoryInjectionConfig(t))
	t.Cleanup(host.Reset)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetRequestId("property-request-id"))

	headerAction := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
		{"x-request-id", "request-recent-redis-1"},
	})
	require.Equal(t, types.HeaderStopIteration, headerAction)

	bodyAction := host.CallOnHttpRequestBody([]byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "user", "content": "current question"}
		],
		"stream": false
	}`))
	require.Equal(t, types.ActionPause, bodyAction)
	requireMemoryRecentRedisLookup(t, host)
	return host
}

func requireMemoryRecentRedisLookup(t *testing.T, host test.TestHost) {
	t.Helper()
	calls := host.GetRedisCalloutAttributes()
	require.NotEmpty(t, calls, "memory-enabled request should read Redis recent memory before Console assemble")
	require.Equal(t, memoryRecentRedisUpstream, calls[0].Upstream)

	cmd := requireMemoryRecentRedisCommand(t, calls[0].Query)
	require.Len(t, cmd, 2, "recent memory Redis lookup should issue GET with one key")
	require.Equal(t, "GET", strings.ToUpper(cmd[0]))
	key := cmd[1]
	require.Truef(t, strings.HasPrefix(key, "memory:recent"), "recent memory key should use configured prefix, got %q", key)
	require.Contains(t, key, "tenant-a")
	require.Contains(t, key, "consumer-a")
	require.Contains(t, key, "session-a")
}

func requireMemoryRecentRedisCommand(t *testing.T, query []byte) []string {
	t.Helper()
	value, _, err := resp.NewReader(bytes.NewReader(query)).ReadValue()
	require.NoError(t, err, "recent memory Redis call should be valid RESP")
	array := value.Array()
	require.NotEmpty(t, array, "recent memory Redis command should be an array")

	cmd := make([]string, 0, len(array))
	for _, item := range array {
		cmd = append(cmd, item.String())
	}
	return cmd
}

func callMemoryRecentRedisResponse(t *testing.T, host test.TestHost, status int32, redisResp []byte) {
	t.Helper()
	host.CallOnRedisCall(status, redisResp)
	requireMemoryAssembleCall(t, host)
}

func requireMemoryRecentAssembleOnly(t *testing.T, host test.TestHost) {
	t.Helper()
	host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, "inject", map[string]interface{}{
		"role":    "system",
		"content": "Console memory context",
	}, nil))

	require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
	messages := requireMemoryRequestMessages(t, host)
	require.Len(t, messages, 2)
	requireMemoryMessage(t, messages[0], "system", "Console memory context")
	requireMemoryMessage(t, messages[1], "user", "current question")
}

func validMemoryRecentRecord(t *testing.T, overrides map[string]interface{}) string {
	t.Helper()
	record := map[string]interface{}{
		"schema_version": 1,
		"tenant":         "tenant-a",
		"consumer":       "consumer-a",
		"policy_version": "memory-policy-v1",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "recent safe user"},
			{"role": "assistant", "content": "recent safe assistant"},
		},
		"expires_at_ms": time.Now().Add(time.Hour).UnixMilli(),
	}
	for key, value := range overrides {
		record[key] = value
	}
	body, err := json.Marshal(record)
	require.NoError(t, err)
	return string(body)
}
