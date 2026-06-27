package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

func TestMemoryConsoleAssemble(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("off mode skips Console assemble and memory behavior", func(t *testing.T) {
			host, status := newMemoryConfigTestHost(memoryConsoleAssembleConfig(t, "off"))
			t.Cleanup(host.Reset)
			require.Equal(t, types.OnPluginStartStatusOK, status)
			require.NoError(t, host.SetRouteName("memory-route"))

			headerAction := host.CallOnHttpRequestHeaders(memoryConsoleAssembleHeaders())
			bodyAction := host.CallOnHttpRequestBody(memoryConsoleAssembleRequestBody())

			require.Equal(t, types.ActionContinue, headerAction)
			require.Equal(t, types.ActionContinue, bodyAction)
			require.Empty(t, host.GetRedisCalloutAttributes())
			require.Empty(t, host.GetHttpCalloutAttributes())
		})

		t.Run("recent-only mode skips Console assemble and injects Redis recent messages", func(t *testing.T) {
			host := startMemoryConsoleAssembleRequest(t, "recent-only")
			host.CallOnRedisCall(0, test.CreateRedisRespString(validMemoryRecentRecord(t, nil)))

			require.Empty(t, host.GetHttpCalloutAttributes(), "recent-only mode must not call Console assemble")
			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			messages := requireMemoryRequestMessages(t, host)
			require.Len(t, messages, 3)
			requireMemoryMessage(t, messages[0], "user", "recent safe user")
			requireMemoryMessage(t, messages[1], "assistant", "recent safe assistant")
			requireMemoryMessage(t, messages[2], "user", "current question")
		})

		for _, mode := range []string{"digest", "semantic"} {
			t.Run(mode+" mode calls Console assemble after Redis miss", func(t *testing.T) {
				host := startMemoryConsoleAssembleRequest(t, mode)
				host.CallOnRedisCall(0, test.CreateRedisRespNull())

				facts := requireMemoryAssembleCall(t, host)
				require.Equal(t, mode, facts["memory_mode"])
				require.Equal(t, "current question", facts["current_question"])
			})
		}

		tests := []struct {
			name             string
			decision         string
			memoryMessage    map[string]interface{}
			responseRecent   []map[string]interface{}
			expectedMessages []memoryExpectedMessage
		}{
			{
				name:     "inject decision inserts memory and response recent messages",
				decision: "inject",
				memoryMessage: map[string]interface{}{
					"role":    "system",
					"content": "Console memory context",
				},
				responseRecent: []map[string]interface{}{
					{"role": "user", "content": "console recent user"},
					{"role": "assistant", "content": "console recent assistant"},
				},
				expectedMessages: []memoryExpectedMessage{
					{role: "system", content: "Console memory context"},
					{role: "user", content: "console recent user"},
					{role: "assistant", content: "console recent assistant"},
					{role: "user", content: "current question"},
				},
			},
			{
				name:     "recent_only decision uses Console recent messages without memory message",
				decision: "recent_only",
				responseRecent: []map[string]interface{}{
					{"role": "user", "content": "console recent user"},
					{"role": "assistant", "content": "console recent assistant"},
				},
				expectedMessages: []memoryExpectedMessage{
					{role: "user", content: "console recent user"},
					{role: "assistant", content: "console recent assistant"},
					{role: "user", content: "current question"},
				},
			},
			{
				name:     "skip decision keeps current request messages only",
				decision: "skip",
				expectedMessages: []memoryExpectedMessage{
					{role: "user", content: "current question"},
				},
			},
			{
				name:     "bypass decision keeps current request messages only",
				decision: "bypass",
				expectedMessages: []memoryExpectedMessage{
					{role: "user", content: "current question"},
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				host := startMemoryConsoleAssembleRequest(t, "semantic")
				host.CallOnRedisCall(0, test.CreateRedisRespString(validMemoryRecentRecord(t, nil)))
				requireMemoryAssembleCall(t, host)

				host.CallOnHttpCall(memoryAssembleHeaders(), memoryAssembleResponse(t, tt.decision, tt.memoryMessage, tt.responseRecent))

				require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				requireMemoryExpectedMessages(t, requireMemoryRequestMessages(t, host), tt.expectedMessages)
				requireMemoryConsoleSafeLogs(t,
					host,
					"Console memory context",
					"console recent user",
					"console recent assistant",
					"Console trace raw value",
					"Console diagnostic raw value",
				)
			})
		}

		t.Run("timeout falls back to Redis recent memory", func(t *testing.T) {
			host := startMemoryConsoleAssembleRequest(t, "semantic")
			host.CallOnRedisCall(0, test.CreateRedisRespString(validMemoryRecentRecord(t, nil)))
			requireMemoryAssembleCall(t, host)

			host.CallOnHttpCall(nil, nil)

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			requireMemoryExpectedMessages(t, requireMemoryRequestMessages(t, host), []memoryExpectedMessage{
				{role: "user", content: "recent safe user"},
				{role: "assistant", content: "recent safe assistant"},
				{role: "user", content: "current question"},
			})
		})

		t.Run("invalid response falls back to Redis recent memory and does not leak raw memory into logs", func(t *testing.T) {
			host := startMemoryConsoleAssembleRequest(t, "semantic")
			host.CallOnRedisCall(0, test.CreateRedisRespString(validMemoryRecentRecord(t, nil)))
			requireMemoryAssembleCall(t, host)

			host.CallOnHttpCall(memoryAssembleHeaders(), []byte(`{"schema_version":1,"decision":"inject","memory_message":{"role":"system","content":"raw Console memory must not be logged"}`))

			require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
			requireMemoryExpectedMessages(t, requireMemoryRequestMessages(t, host), []memoryExpectedMessage{
				{role: "user", content: "recent safe user"},
				{role: "assistant", content: "recent safe assistant"},
				{role: "user", content: "current question"},
			})
			requireMemoryConsoleSafeLogs(t, host, "raw Console memory must not be logged", "recent safe user", "recent safe assistant")
		})
	})
}

type memoryExpectedMessage struct {
	role    string
	content string
}

func memoryConsoleAssembleConfig(t *testing.T, memoryMode string) json.RawMessage {
	t.Helper()
	return mustMemoryConfig(t, map[string]interface{}{
		"redis_stream": map[string]interface{}{
			"service_name": "redis.memory.svc.cluster.local",
		},
		"recent_cache": map[string]interface{}{
			"service_name": "redis.recent.svc.cluster.local",
		},
		"console_internal": map[string]interface{}{
			"service_name":  memoryConsoleService,
			"service_port":  8080,
			"assemble_path": memoryConsolePath,
			"timeout_ms":    120,
		},
		"tenant_header":        "x-mse-tenant",
		"consumer_header":      "x-mse-consumer",
		"session_header":       "x-mse-session",
		"request_id_header":    "x-request-id",
		"enable_path_suffixes": []string{"/v1/chat/completions"},
		"fail_policy":          "open",
		"_rules_": []map[string]interface{}{
			{
				"_match_route_":       []string{"memory-route"},
				"memory_mode":         memoryMode,
				"recent_window_turns": 6,
				"memory_token_budget": 1500,
				"assemble_timeout_ms": 80,
				"inject_role":         "system",
				"semantic_top_k":      3,
				"capture_response":    true,
				"policy_version":      "memory-policy-v1",
			},
		},
	})
}

func startMemoryConsoleAssembleRequest(t *testing.T, memoryMode string) test.TestHost {
	t.Helper()
	host, status := newMemoryConfigTestHost(memoryConsoleAssembleConfig(t, memoryMode))
	t.Cleanup(host.Reset)
	require.Equal(t, types.OnPluginStartStatusOK, status)
	require.NoError(t, host.SetRouteName("memory-route"))
	require.NoError(t, host.SetRequestId("property-request-id"))

	headerAction := host.CallOnHttpRequestHeaders(memoryConsoleAssembleHeaders())
	require.Equal(t, types.HeaderStopIteration, headerAction)

	bodyAction := host.CallOnHttpRequestBody(memoryConsoleAssembleRequestBody())
	require.Equal(t, types.ActionPause, bodyAction)
	requireMemoryRecentRedisLookup(t, host)
	return host
}

func memoryConsoleAssembleHeaders() [][2]string {
	return [][2]string{
		{":authority", "example.com"},
		{":path", "/v1/chat/completions"},
		{":method", "POST"},
		{"content-type", "application/json"},
		{"x-mse-tenant", "tenant-a"},
		{"x-mse-consumer", "consumer-a"},
		{"x-mse-session", "session-a"},
		{"x-request-id", "request-console-assemble-1"},
	}
}

func memoryConsoleAssembleRequestBody() []byte {
	return []byte(`{
		"model": "qwen-turbo",
		"messages": [
			{"role": "user", "content": "current question"}
		],
		"stream": false
	}`)
}

func requireMemoryExpectedMessages(t *testing.T, messages []map[string]interface{}, expected []memoryExpectedMessage) {
	t.Helper()
	require.Len(t, messages, len(expected))
	for i, want := range expected {
		requireMemoryMessage(t, messages[i], want.role, want.content)
	}
}

func requireMemoryConsoleSafeLogs(t *testing.T, host test.TestHost, forbidden ...string) {
	t.Helper()
	logs := strings.ToLower(strings.Join(append(append(append(host.GetDebugLogs(), host.GetInfoLogs()...), host.GetWarnLogs()...), host.GetErrorLogs()...), "\n"))
	for _, value := range forbidden {
		require.NotContains(t, logs, strings.ToLower(value))
	}
}
