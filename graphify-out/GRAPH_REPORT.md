# Graph Report - .  (2026-05-13)

## Corpus Check
- 86 files · ~67,542 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 732 nodes · 1042 edges · 50 communities (26 shown, 24 thin omitted)
- Extraction: 88% EXTRACTED · 12% INFERRED · 0% AMBIGUOUS · INFERRED: 125 edges (avg confidence: 0.79)
- Token cost: 11,067 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Plugin Context Runtime|Plugin Context Runtime]]
- [[_COMMUNITY_Redis Client API|Redis Client API]]
- [[_COMMUNITY_HTTP Example Tests|HTTP Example Tests]]
- [[_COMMUNITY_AI Gateway Plugins|AI Gateway Plugins]]
- [[_COMMUNITY_HTTP Context Methods|HTTP Context Methods]]
- [[_COMMUNITY_HTTP Call Examples|HTTP Call Examples]]
- [[_COMMUNITY_Test Framework|Test Framework]]
- [[_COMMUNITY_Cluster Wrapper Types|Cluster Wrapper Types]]
- [[_COMMUNITY_Complex HTTP Example|Complex HTTP Example]]
- [[_COMMUNITY_Plugin Context Config|Plugin Context Config]]
- [[_COMMUNITY_Test Host Callbacks|Test Host Callbacks]]
- [[_COMMUNITY_Auth Traffic Plugins|Auth Traffic Plugins]]
- [[_COMMUNITY_Logging API|Logging API]]
- [[_COMMUNITY_Rule Matcher|Rule Matcher]]
- [[_COMMUNITY_Token Usage Parsing|Token Usage Parsing]]
- [[_COMMUNITY_Log Wrapper|Log Wrapper]]
- [[_COMMUNITY_HTTP Cluster Client|HTTP Cluster Client]]
- [[_COMMUNITY_Plugin Leadership Context|Plugin Leadership Context]]
- [[_COMMUNITY_Redis Wrapper Options|Redis Wrapper Options]]
- [[_COMMUNITY_Encoded Data Proto|Encoded Data Proto]]
- [[_COMMUNITY_Redis Test Helpers|Redis Test Helpers]]
- [[_COMMUNITY_WasmPlugin Configuration|WasmPlugin Configuration]]
- [[_COMMUNITY_Context Interfaces|Context Interfaces]]
- [[_COMMUNITY_VM Context Options|VM Context Options]]
- [[_COMMUNITY_Security Blocking Plugins|Security Blocking Plugins]]
- [[_COMMUNITY_Plugin Startup Hooks|Plugin Startup Hooks]]
- [[_COMMUNITY_Prompt Decoration|Prompt Decoration]]
- [[_COMMUNITY_AI Security Masking|AI Security Masking]]
- [[_COMMUNITY_Load Test Script|Load Test Script]]
- [[_COMMUNITY_Parse Config Option|Parse Config Option]]
- [[_COMMUNITY_Override Config Option|Override Config Option]]
- [[_COMMUNITY_Request Headers Option|Request Headers Option]]
- [[_COMMUNITY_Request Body Option|Request Body Option]]
- [[_COMMUNITY_Streaming Request Option|Streaming Request Option]]
- [[_COMMUNITY_Response Headers Option|Response Headers Option]]
- [[_COMMUNITY_Response Body Option|Response Body Option]]
- [[_COMMUNITY_Streaming Response Option|Streaming Response Option]]
- [[_COMMUNITY_Stream Done Option|Stream Done Option]]
- [[_COMMUNITY_Rebuild Option|Rebuild Option]]
- [[_COMMUNITY_Rebuild Memory Option|Rebuild Memory Option]]
- [[_COMMUNITY_Cycle Limit Option|Cycle Limit Option]]
- [[_COMMUNITY_VM Plugin Factory|VM Plugin Factory]]
- [[_COMMUNITY_Local Wasm Build|Local Wasm Build]]
- [[_COMMUNITY_Request Validation|Request Validation]]
- [[_COMMUNITY_GraphQL Transformation|GraphQL Transformation]]
- [[_COMMUNITY_Body Parameter Routing|Body Parameter Routing]]
- [[_COMMUNITY_TinyGo WebAssembly|TinyGo WebAssembly]]
- [[_COMMUNITY_Envoy OCI Delivery|Envoy OCI Delivery]]

## God Nodes (most connected - your core abstractions)
1. `redisCallInternal()` - 61 edges
2. `RedisClusterClient[C]` - 61 edges
3. `respString()` - 60 edges
4. `CommonHttpCtx[PluginConfig]` - 41 edges
5. `TestHost` - 27 edges
6. `NewTestHost()` - 19 edges
7. `RunTest()` - 19 edges
8. `DefaultLog` - 16 edges
9. `testLogger` - 14 edges
10. `HttpCall()` - 14 edges

## Surprising Connections (you probably didn't know these)
- `Request Block parseConfig` --conceptually_related_to--> `RE2 Host and Path Patterns`  [INFERRED]
  examples/request-block/main.go → docs/plugins/transformation/transformer.md
- `PluginContext Interface` --conceptually_related_to--> `CommonPluginCtx`  [INFERRED]
  pkg/iface/context.go → docs/plugins/wasm-dev/wasm16.md
- `HttpContext Interface` --conceptually_related_to--> `CommonHttpCtx`  [INFERRED]
  pkg/iface/context.go → docs/plugins/wasm-dev/wasm16.md
- `HttpContext Interface` --implements--> `HTTP Body Buffering and Streaming`  [EXTRACTED]
  pkg/iface/context.go → docs/plugins/wasm-dev/wasm16.md
- `Safe Log HTTP Call Example Plugin` --calls--> `Higress HTTP Client`  [EXTRACTED]
  examples/safe-log-http-call/main.go → docs/plugins/wasm-dev/wasm17.md

## Hyperedges (group relationships)
- **Wasm Plugin Development And Delivery** — readme_wasm_go_sdk, wasm_go_go124_native_wasm, wasm_image_spec_oci_image, readme_wasmplugin_api, custom_custom_plugin_workflow [EXTRACTED 1.00]
- **AI Request Context Enrichment** — geo_ip_geo_location_properties, ai_prompt_decorator_prompt_insertion, ai_prompt_template_request_templates, ai_history_conversation_memory, ai_intent_intent_category_property [INFERRED 0.82]
- **AI Provider Gateway Stack** — ai_proxy_openai_compatible_proxy, ai_cache_llm_response_cache, ai_statistics_ai_observability, ai_security_guard_content_moderation, ai_quota_quota_management [INFERRED 0.84]
- **Consumer-Centric Authentication Plugins** — hmac_auth_apisix_plugin, hmac_auth_plugin, jwt_auth_plugin, key_auth_plugin, oauth_plugin, auth_consumer_identity [EXTRACTED 1.00]
- **Request Filtering and Blocking Plugins** — ext_auth_plugin, oidc_plugin, bot_detect_plugin, ip_restriction_plugin, request_block_plugin, waf_plugin, match_rule_filtering, request_blocking_rules [INFERRED 0.82]
- **Traffic Control and Progressive Release Plugins** — cluster_key_rate_limit_plugin, key_rate_limit_plugin, traffic_tag_plugin, frontend_gray_plugin, request_key_extraction, traffic_tagging_strategy, frontend_gray_release [INFERRED 0.86]
- **Higress Wasm Configuration Scope Pipeline** — wasm14_wasmplugin_crd, wasm14_wasmplugin_config_scopes, wasm16_parse_config_hooks, wasm19_plugin_effective_scope [EXTRACTED 1.00]
- **External Call Examples** — main_http_call_plugin, main_complex_http_call_plugin, main_safe_log_http_call_plugin, wasm17_http_client [INFERRED 0.86]
- **Proxy-Wasm Context Model** — wasm15_plugin_contexts, wasm16_common_vm_ctx, wasm16_common_plugin_ctx, wasm16_common_http_ctx, context_http_context_interface [EXTRACTED 1.00]
- **Rule Matching and Isolation** — rule_matcher_rule_matcher, rule_matcher_rule_config, rule_matcher_match_keys, rule_matcher_rule_level_isolation, rule_matcher_rule_backup_store [EXTRACTED 1.00]
- **Plugin Runtime Lifecycle** — plugin_wrapper_common_vm_ctx, plugin_wrapper_common_plugin_ctx, plugin_wrapper_common_http_ctx, plugin_wrapper_context_options, rule_matcher_rule_matcher [EXTRACTED 1.00]
- **Test Framework Surface** — README_test_framework, host_test_host, test_test_runners, redis_redis_resp_builder, utils_header_utilities [EXTRACTED 1.00]

## Communities (50 total, 24 thin omitted)

### Community 0 - "Plugin Context Runtime"
Cohesion: 0.03
Nodes (42): CommonHttpCtx, CommonPluginCtx, CommonVmCtx, CtxOption, HttpContext, Lease, Log, logOption (+34 more)

### Community 1 - "Redis Client API"
Cohesion: 0.09
Nodes (3): redisCallInternal(), respString(), RedisClusterClient[C]

### Community 2 - "HTTP Example Tests"
Cohesion: 0.09
Nodes (37): TestComplexHttpCall(), TestComplexHttpCallWithDifferentLoops(), TestHttpCall(), TestRebuildExample(), TestRebuildMultipleRequests(), TestAllowValidRequest(), TestBlockByBody(), TestBlockByHeaders() (+29 more)

### Community 3 - "AI Gateway Plugins"
Cohesion: 0.06
Nodes (41): OpenAPI Tool Definitions, Configurable ReAct API Agent, LLM Response Cache, Semantic Cache, String Match Cache, AI Conversation History Memory, AI History Query API, Intent Category Request Property (+33 more)

### Community 5 - "HTTP Call Examples"
Cohesion: 0.06
Nodes (40): HttpContext Interface, HTTPExecutionPhase, PluginContext Interface, Complex HTTP Call k6 Load Test, Complex HTTP Call busyLoop, Complex HTTP Call Example Plugin, HTTP Call onHttpRequestHeaders, HTTP Call parseConfig (+32 more)

### Community 6 - "Test Framework"
Cohesion: 0.07
Nodes (40): Go and Wasm Test Modes, Test Framework Documentation, Cluster Abstraction, Outbound Cluster Name Formats, Cluster Wrapper Tests, Proxy Wasm Host Emulator, Test Host, HTTP Cluster Client (+32 more)

### Community 7 - "Cluster Wrapper Types"
Cohesion: 0.05
Nodes (12): Cluster, ConsulCluster, DnsCluster, FQDNCluster, K8sCluster, NacosCluster, GetRequestHost(), GetRequestPath() (+4 more)

### Community 8 - "Complex HTTP Example"
Cohesion: 0.07
Nodes (24): HttpCallConfig, busyLoop(), init(), onHttpRequestHeaders(), parseConfig(), TestBusyLoopLinearity(), HttpCallConfig, init() (+16 more)

### Community 9 - "Plugin Context Config"
Cohesion: 0.07
Nodes (9): completeConfig, customConfig, mockPluginContext, parseConfig(), parseConfigWithError(), TestParseOverrideConfig(), TestParseRuleConfig(), TestRuleLevelConfigIsolation() (+1 more)

### Community 11 - "Auth Traffic Plugins"
Cohesion: 0.11
Nodes (27): Consumer Identity and Authorization, Cache Control Plugin, Cluster Key Rate Limit Plugin, CORS Plugin, Custom Response Plugin, Ext Auth Plugin, External Policy Service, Frontend Gray Plugin (+19 more)

### Community 12 - "Logging API"
Cohesion: 0.09
Nodes (12): Log, Debug(), Debugf(), Info(), Infof(), SetPluginLog(), SetSafeLogEnabled(), UnsafeInfo() (+4 more)

### Community 13 - "Rule Matcher"
Cohesion: 0.13
Nodes (9): Category, HostMatcher, MatchType, stripPortFromHost(), RuleConfig, RuleConfig[PluginConfig], RuleMatcher, RuleMatcher[PluginConfig] (+1 more)

### Community 14 - "Token Usage Parsing"
Cohesion: 0.19
Nodes (15): TokenUsage, ExtractChatId(), ExtractInputTokenDetails(), ExtractInputTokens(), ExtractModel(), ExtractOutputTokenDetails(), ExtractOutputTokens(), ExtractTotalTokens() (+7 more)

### Community 16 - "HTTP Cluster Client"
Cohesion: 0.19
Nodes (5): ClusterClient, ClusterClient[C], HttpCall(), HttpClient, ResponseCallback

### Community 18 - "Redis Wrapper Options"
Cohesion: 0.17
Nodes (6): optionFunc, RedisCall(), RedisClient, RedisClusterClient, redisOption, RedisResponseCallback

### Community 19 - "Encoded Data Proto"
Cohesion: 0.22
Nodes (4): file_inject_encoded_data_proto_init(), file_inject_encoded_data_proto_rawDescGZIP(), init(), InjectEncodedDataToFilterChainArguments

### Community 20 - "Redis Test Helpers"
Cohesion: 0.35
Nodes (10): boolToInt(), CreateRedisResp(), CreateRedisRespArray(), CreateRedisRespBool(), CreateRedisRespError(), CreateRedisRespFloat(), CreateRedisRespInt(), CreateRedisRespNull() (+2 more)

### Community 21 - "WasmPlugin Configuration"
Cohesion: 0.4
Nodes (6): Custom Response Plugin, WasmPlugin Configuration Scopes, Higress WasmPlugin CRD, ParseConfig and ParseRuleConfig Hooks, Wasm Plugin Phase and Priority, Wasm Plugin Effective Scope

### Community 22 - "Context Interfaces"
Cohesion: 0.4
Nodes (4): HttpContext, HTTPExecutionPhase, PluginContext, RouteResponseCallback

### Community 23 - "VM Context Options"
Cohesion: 0.4
Nodes (4): NewCommonVmCtx(), NewCommonVmCtxWithOptions(), SetCtxWithOptions(), prePluginOption[PluginConfig]

### Community 24 - "Security Blocking Plugins"
Cohesion: 0.5
Nodes (5): Bot Detect Plugin, IP Restriction Plugin, Request Block Plugin, Request Blocking Rules, WAF Plugin

### Community 25 - "Plugin Startup Hooks"
Cohesion: 0.5
Nodes (3): ParseConfig(), PrePluginStartOrReload(), setGlobalMaxRequestsPerIoCycle()

### Community 26 - "Prompt Decoration"
Cohesion: 0.5
Nodes (4): AI Prompt Decorator Prompt Insertion, AI Request Prompt Templates, Geo IP Request Properties, Geo-Aware Prompt Decoration

### Community 27 - "AI Security Masking"
Cohesion: 0.5
Nodes (4): AI Sensitive Data Replacement, AI Sensitive Word Interception, AI Content Moderation, AI Security Metrics And Trace Attributes

## Knowledge Gaps
- **150 isolated node(s):** `options`, `headers`, `HttpCallConfig`, `HttpCallConfig`, `RebuildConfig` (+145 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **24 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `recoverFunc()` connect `HTTP Context Methods` to `Plugin Context Runtime`, `HTTP Example Tests`?**
  _High betweenness centrality (0.124) - this node is a cross-community bridge._
- **Why does `UnsafeInfof()` connect `Logging API` to `HTTP Cluster Client`?**
  _High betweenness centrality (0.099) - this node is a cross-community bridge._
- **What connects `options`, `headers`, `HttpCallConfig` to the rest of the system?**
  _150 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Plugin Context Runtime` be split into smaller, more focused modules?**
  _Cohesion score 0.03 - nodes in this community are weakly interconnected._
- **Should `Redis Client API` be split into smaller, more focused modules?**
  _Cohesion score 0.09 - nodes in this community are weakly interconnected._
- **Should `HTTP Example Tests` be split into smaller, more focused modules?**
  _Cohesion score 0.09 - nodes in this community are weakly interconnected._
- **Should `AI Gateway Plugins` be split into smaller, more focused modules?**
  _Cohesion score 0.06 - nodes in this community are weakly interconnected._