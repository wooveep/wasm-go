package main

import (
	"net/http"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

// RedisInit in another Wasm VM can close the shared host pool mid-GET.
func TestQuotaRedisTransportRecovery(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		for _, scenario := range []struct {
			name   string
			status int32
			value  []byte
			want   int
		}{
			{"reconnected positive balance", 0, test.CreateRedisRespString("100"), 0},
			{"reconnected empty wallet", 0, test.CreateRedisRespString("0"), http.StatusForbidden},
			{"persistent connection failure", 1, nil, http.StatusServiceUnavailable},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				host, status := test.NewTestHost(monetaryConfig)
				defer host.Reset()
				require.Equal(t, types.OnPluginStartStatusOK, status)
				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"}, {":path", "/v1/chat/completions"},
					{":method", "POST"}, {"x-tenant-id", "tenant-a"}, {"x-consumer-id", "consumer-a"},
				})
				require.Equal(t, types.HeaderStopAllIterationAndWatermark, action)
				firstQuery := append([]byte(nil), host.GetRedisCalloutAttributes()[0].Query...)
				host.CallOnRedisCall(1, nil)
				require.Len(t, host.GetRedisCalloutAttributes(), 1, "one read-only retry after a closed connection")
				require.Equal(t, firstQuery, host.GetRedisCalloutAttributes()[0].Query)
				assertNoRedisMutationQueries(t, host.GetRedisCalloutAttributes())
				host.CallOnRedisCall(scenario.status, scenario.value)
				require.Empty(t, host.GetRedisCalloutAttributes(), "retry must be bounded")
				if scenario.want == 0 {
					require.Equal(t, types.ActionContinue, host.GetHttpStreamAction())
				} else {
					require.Equal(t, uint32(scenario.want), host.GetLocalResponse().StatusCode)
				}
				host.CompleteHttp()
			})
		}
	})
}
