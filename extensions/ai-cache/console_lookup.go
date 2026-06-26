package main

import (
	"encoding/json"
	"net/http"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-cache/config"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	logs "github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

type consoleLookupRequest struct {
	Tenant             string `json:"tenant"`
	Consumer           string `json:"consumer,omitempty"`
	Session            string `json:"session,omitempty"`
	Route              string `json:"route"`
	Model              string `json:"model"`
	CacheScope         string `json:"cache_scope"`
	CachePolicyVersion string `json:"cache_policy_version"`
	RequestDigest      string `json:"request_digest"`
}

func shouldUseConsoleLookup(c config.PluginConfig) bool {
	return c.ConsoleLookup.Enabled && c.RoutePolicy.EnableConsoleLookup && c.RoutePolicy.EnableReplay
}

func lookupMaterializedRecordFromConsole(material ScopedCacheKeyMaterial, ctx wrapper.HttpContext, c config.PluginConfig, log logs.Log, stream bool) error {
	requestBody, err := buildConsoleLookupRequestBody(material, ctx)
	if err != nil {
		return err
	}

	client := wrapper.NewClusterClient(wrapper.FQDNCluster{
		FQDN: c.ConsoleLookup.ServiceName,
		Port: int64(c.ConsoleLookup.ServicePort),
	})
	headers := [][2]string{{"content-type", "application/json"}}
	return client.Post(c.ConsoleLookup.Path, headers, requestBody, func(statusCode int, _ http.Header, responseBody []byte) {
		handleConsoleLookupResponse(material, statusCode, responseBody, ctx, log, stream)
	}, uint32(c.ConsoleLookup.Timeout))
}

func buildConsoleLookupRequestBody(material ScopedCacheKeyMaterial, ctx wrapper.HttpContext) ([]byte, error) {
	return json.Marshal(consoleLookupRequest{
		Tenant:             material.Tenant,
		Consumer:           material.Consumer,
		Session:            ctx.GetStringContext(CACHE_SESSION_CONTEXT_KEY, ""),
		Route:              material.Route,
		Model:              material.Model,
		CacheScope:         material.CacheScope,
		CachePolicyVersion: material.CachePolicyVersion,
		RequestDigest:      material.RequestDigest,
	})
}

func handleConsoleLookupResponse(material ScopedCacheKeyMaterial, statusCode int, responseBody []byte, ctx wrapper.HttpContext, log logs.Log, stream bool) {
	if statusCode != http.StatusOK {
		log.Warnf("[%s] [handleConsoleLookupResponse] Console lookup returned status %d for key: %s", PLUGIN_NAME, statusCode, material.RedisKey)
		proxywasm.ResumeHttpRequest()
		return
	}

	record, err := LoadAndValidateMaterializedRecord(string(responseBody), material, stream)
	if err != nil {
		log.Warnf("[%s] [handleConsoleLookupResponse] Console lookup record rejected for key: %s, reason: %v", PLUGIN_NAME, material.RedisKey, err)
		proxywasm.ResumeHttpRequest()
		return
	}
	if err := replayMaterializedRecord(record, stream, ctx, log); err != nil {
		log.Warnf("[%s] [handleConsoleLookupResponse] Console lookup replay failed for key: %s, error: %v", PLUGIN_NAME, material.RedisKey, err)
		proxywasm.ResumeHttpRequest()
		return
	}
}
