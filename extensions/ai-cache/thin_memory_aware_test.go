package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

const thinMemoryDigestHeader = "x-mse-memory-digest"

func TestThinMemoryAwareCache(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("consumer-scoped memory route separates materialized lookup keys by consumer", func(t *testing.T) {
			keyA := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-a")
			keyB := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-b")
			requireThinMemoryAwareKeysDiffer(t, keyA, keyB, "consumer-scoped memory-aware cache keys must vary by consumer")
		})

		t.Run("memory policy version and digest affect materialized lookup key", func(t *testing.T) {
			keyA := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-a")
			keySame := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v1", "memory-digest-a", "consumer-a")
			keyDigestChanged := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v1", "memory-digest-b", "consumer-a")
			keyPolicyChanged := thinMemoryAwareLookupKey(t, "policy_digest", "memory-policy-v2", "memory-digest-a", "consumer-a")

			require.True(t, keyA == keySame, "same memory-aware inputs should produce stable materialized key")
			requireThinMemoryAwareKeysDiffer(t, keyA, keyDigestChanged, "memory digest must affect memory-aware cache lookup key")
			requireThinMemoryAwareKeysDiffer(t, keyA, keyPolicyChanged, "memory policy version must affect memory-aware cache lookup key or policy material")
		})

		t.Run("configured memory bypass skips cache lookup and continues upstream", func(t *testing.T) {
			host, action := startThinMemoryAwareRequest(t, "bypass", "memory-policy-v1", "memory-digest-a", "consumer-a")
			defer host.Reset()

			require.Equal(t, types.ActionContinue, action)
			require.Empty(t, host.GetRedisCalloutAttributes(), "memory bypass should not issue Redis cache lookup")
			require.Empty(t, host.GetHttpCalloutAttributes(), "memory bypass should not issue Console cache lookup")
			require.Nil(t, host.GetLocalResponse(), "memory bypass should not replay a local cache hit")
		})
	})
}

func thinMemoryAwareLookupKey(t *testing.T, mode, memoryPolicyVersion, memoryDigest, consumer string) string {
	t.Helper()
	host, action := startThinMemoryAwareRequest(t, mode, memoryPolicyVersion, memoryDigest, consumer)
	defer host.Reset()
	require.Equal(t, types.ActionPause, action)
	requireThinMemoryAwareNoLegacyCacheCalls(t, host)
	return requireThinMemoryAwareMaterializedGetKey(t, host)
}

func requireThinMemoryAwareKeysDiffer(t *testing.T, left, right, message string) {
	t.Helper()
	require.True(t, left != right, message)
}

func thinMemoryAwareConfig(t *testing.T, mode, memoryPolicyVersion string) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(map[string]interface{}{
		"cache": map[string]interface{}{
			"type":           "redis",
			"serviceName":    "redis.static",
			"servicePort":    6379,
			"cacheKeyPrefix": "higress-ai-cache:",
		},
		"materialized_lookup": map[string]interface{}{
			"redis": map[string]interface{}{
				"enabled":      true,
				"service_name": "redis.static",
				"service_port": 6379,
				"key_prefix":   "cache:materialized:",
				"timeout":      80,
			},
		},
		"console_lookup": map[string]interface{}{
			"enabled":      true,
			"service_name": thinConsoleService,
			"service_port": 8080,
			"path":         thinConsolePath,
			"timeout":      50,
		},
		"route_policy": map[string]interface{}{
			"enable_redis_lookup":   true,
			"enable_console_lookup": true,
			"enable_replay":         true,
			"enabled_path_suffixes": []string{"/v1/chat/completions"},
			"memory": map[string]interface{}{
				"enabled":        true,
				"cache_mode":     mode,
				"policy_version": memoryPolicyVersion,
				"digest_header":  thinMemoryDigestHeader,
			},
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"session_header":       "x-mse-session",
		"cache_scope":          "consumer",
		"cache_policy_version": "policy-v1",
	})
	require.NoError(t, err)
	return data
}

func startThinMemoryAwareRequest(t *testing.T, mode, memoryPolicyVersion, memoryDigest, consumer string) (test.TestHost, types.Action) {
	t.Helper()
	host, status := test.NewTestHost(thinMemoryAwareConfig(t, mode, memoryPolicyVersion))
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("test-route-default"))
	require.NoError(t, host.SetRequestId("request-memory-aware-1"))

	action := host.CallOnHttpRequestHeaders([][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", consumer},
		{"x-mse-session", "session-a"},
		{thinMemoryDigestHeader, memoryDigest},
	})
	require.Equal(t, types.HeaderStopIteration, action)

	action = host.CallOnHttpRequestBody([]byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "user", "content": "weather?"}
		],
		"stream": false
	}`))
	return host, action
}

func requireThinMemoryAwareMaterializedGetKey(t *testing.T, host test.TestHost) string {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 2 || !strings.EqualFold(cmd[0], "get") {
			continue
		}
		require.Truef(t, strings.HasPrefix(cmd[1], "cache:materialized:"), "memory-aware cache lookup should use materialized Redis key; redis calls=%s", thinResponseCaptureCallSummary(t, host))
		return cmd[1]
	}
	require.Failf(t, "missing memory-aware materialized Redis lookup", "redis calls=%s", thinResponseCaptureCallSummary(t, host))
	return ""
}

func requireThinMemoryAwareNoLegacyCacheCalls(t *testing.T, host test.TestHost) {
	t.Helper()
	for _, call := range host.GetRedisCalloutAttributes() {
		cmd, ok := thinResponseCaptureCommand(t, call.Query)
		if !ok || len(cmd) < 2 {
			continue
		}
		if strings.EqualFold(cmd[0], "get") || strings.EqualFold(cmd[0], "set") {
			require.Falsef(t, strings.HasPrefix(cmd[1], "higress-ai-cache:"), "memory-aware route must not use legacy cache lookup/write; redis calls=%s", thinResponseCaptureCallSummary(t, host))
		}
	}
}
