---
title: AI 缓存
keywords: [higress,ai cache]
description: AI 缓存插件配置参考
---

**Note**

> 若使用 tinygo 编译，则需要数据面的proxy wasm版本大于等于0.2.100，且编译时需要带上版本的tag，例如：`tinygo build -o main.wasm -scheduler=none -target=wasi -gc=custom -tags="custommalloc nottinygc_finalizer proxy_wasm_version_0_2_100" ./`

## 功能说明

LLM 结果缓存插件，默认配置方式可以直接用于 openai 协议的结果缓存，同时支持流式和非流式响应的缓存。

**提示**

携带请求头`x-higress-skip-ai-cache: on`时，当前请求将不会使用缓存中的内容，而是直接转发给后端服务，同时也不会缓存该请求返回响应的内容


## 运行属性

插件执行阶段：`认证阶段`
插件执行优先级：`10`

## 配置说明
配置分为 3 个部分：向量数据库（vector）；文本向量化接口（embedding）；缓存数据库（cache），同时也提供了细粒度的 LLM 请求/响应提取参数配置等。

## 配置说明

本插件同时支持基于向量数据库的语义化缓存和基于字符串匹配的缓存方法，如果同时配置了向量数据库和缓存数据库，优先使用缓存数据库，未命中场景下使用向量数据库能力。

*Note*: 向量数据库(vector) 和 缓存数据库(cache) 不能同时为空，否则本插件无法提供缓存服务。

| Name | Type | Requirement | Default | Description |
| --- | --- | --- | --- | --- |
| vector | object | optional | - | 向量存储服务配置，详见下文向量数据库服务配置 |
| embedding | object | optional | - | 文本向量化服务配置，详见下文文本向量化服务配置 |
| cache | object | optional | - | 缓存服务配置，详见下文缓存服务配置 |
| cacheKeyStrategy | string | optional | "lastQuestion" | 决定如何根据历史问题生成缓存键的策略。可选值: "lastQuestion" (使用最后一个问题), "allQuestions" (拼接所有问题) 或 "disabled" (禁用缓存) |
| enableSemanticCache | bool | optional | false | 是否启用语义化缓存。若不启用，则使用字符串匹配的方式来查找缓存，此时需要配置cache服务。当配置了 vector provider 时，默认自动开启 |

根据是否需要启用语义缓存，可以只配置组件的组合为:
1. `cache`: 仅启用字符串匹配缓存
2. `vector (+ embedding)`: 启用语义化缓存, 其中若 `vector` 未提供字符串表征服务，则需要自行配置 `embedding` 服务
3. `vector (+ embedding) + cache`: 启用语义化缓存并用缓存服务存储LLM响应以加速

注意若不配置相关组件，则可以忽略相应组件的`required`字段。

## 生产薄缓存路径

在 Modelfusion 生产回放场景中，`ai-cache` 可以作为薄网关插件运行。
该模式下网关不生成 embedding、不执行向量检索、不写 PostgreSQL、不计算价格、
不做账单结算、不修复或删除持久状态，也不持有上游模型凭据。上述职责由
Console 负责，Console 预先生成结构化回放记录。

薄缓存模式中网关负责：

- 为 OpenAI 兼容请求生成带作用域的请求摘要。
- 从 Redis 读取物化回放记录，并可在 Redis miss 后调用 Console
  `/internal/cache/lookup`。
- 在本地回放前严格校验回放记录。
- 只回放 OpenAI 兼容的结构化响应或流式 chunk。
- 在符合条件的上游响应结束后向 Redis Stream 写入一个 `CacheEvent` JSON。
- Redis、Console、校验或事件投递失败时默认 fail-open，继续上游请求或保持用户响应不变。

薄缓存配置示例：

```yaml
materialized_lookup:
  redis:
    enabled: true
    service_name: redis-stack-server.dns
    service_port: 6379
    key_prefix: cache:materialized:
    timeout: 80
console_lookup:
  enabled: true
  service_name: modelfusion-console.dns
  service_port: 8080
  path: /internal/cache/lookup
  timeout: 50
redis_stream:
  enabled: true
  service_name: redis-stack-server.dns
  service_port: 6379
  stream: cache:events
  field: event
  timeout: 120
route_policy:
  enable_redis_lookup: true
  enable_console_lookup: true
  enable_replay: true
  enable_bypass: false
  enabled_path_suffixes:
  - /v1/chat/completions
  memory:
    enabled: true
    cache_mode: policy_digest
    policy_version: memory-policy-v1
    digest_header: x-mse-memory-digest
tenant_header: x-mse-tenant
consumer_header: x-mse-consumer
session_header: x-mse-session
cache_scope: consumer
cache_policy_version: cache-policy-v1
fail_policy: open
```

薄缓存字段：

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| materialized_lookup.redis.enabled | bool | false | 启用 Redis 物化回放查询 |
| materialized_lookup.redis.service_name | string | - | 物化记录 Redis 服务名 |
| materialized_lookup.redis.service_port | int | 6379 | Redis 服务端口 |
| materialized_lookup.redis.key_prefix | string | - | 物化记录 key 前缀 |
| materialized_lookup.redis.timeout | int | - | Redis 查询超时时间，单位毫秒 |
| console_lookup.enabled | bool | false | Redis miss 后启用可选 Console 查询 |
| console_lookup.service_name | string | - | Console 服务名 |
| console_lookup.service_port | int | - | Console 服务端口 |
| console_lookup.path | string | `/internal/cache/lookup` | Console 查询路径 |
| console_lookup.timeout | int | - | Console 查询超时时间，单位毫秒 |
| redis_stream.enabled | bool | false | 启用 `CacheEvent` 投递 |
| redis_stream.service_name | string | - | 事件 Redis 服务名 |
| redis_stream.service_port | int | 6379 | 事件 Redis 服务端口 |
| redis_stream.stream | string | `cache:events` | Redis Stream 名称 |
| redis_stream.field | string | `event` | Redis Stream 字段名 |
| redis_stream.timeout | int | - | 事件投递超时时间，单位毫秒 |
| route_policy.enable_redis_lookup | bool | true | 允许 Redis 物化查询 |
| route_policy.enable_console_lookup | bool | console_lookup.enabled | 允许 Console fallback 查询 |
| route_policy.enable_replay | bool | true | 校验命中后允许本地回放 |
| route_policy.enable_bypass | bool | false | 对当前路由绕过薄缓存 |
| route_policy.enabled_path_suffixes | []string | nil | 允许缓存的 OpenAI 兼容路径后缀 |
| route_policy.memory.cache_mode | string | `policy_digest` | `policy_digest` 将 memory policy 和 digest 纳入缓存 policy 材料；`bypass` 跳过缓存 |
| route_policy.memory.policy_version | string | - | memory-aware 缓存 key 使用的 memory policy 版本 |
| route_policy.memory.digest_header | string | `x-mse-memory-digest` | `ai-memory` 输出的组装 memory digest 请求头 |
| tenant_header | string | `x-mse-tenant` | 租户身份请求头 |
| consumer_header | string | `x-mse-consumer` | 消费者身份请求头 |
| session_header | string | `x-openclaw-session-key` | 会话身份请求头 |
| cache_scope | string | `tenant` | `tenant` 或 `consumer`；memory-aware 路由建议使用 `consumer` |
| cache_policy_version | string | - | 启用薄缓存时必填 |
| fail_policy | string | `open` | 当前生产行为为 fail-open |

物化记录是 Console 拥有的 Redis value。记录使用
`schema_version: ai-cache.materialized.v1`，必须包含 tenant、可选
consumer、route、model、cache scope、cache policy version、request digest、
soft/hard 过期时间、`usage`、`finish_reason`，以及非流式回放的 `response`，
或流式回放的 `stream_replayable: true` 和 `stream_chunks`。插件会拒绝 schema
版本不支持、过期、scope/tenant/consumer/route/model/policy/digest 不匹配以及
payload 格式错误的记录。

`CacheEvent` 使用如下 Redis Stream 写入：

```text
XADD cache:events * event <CacheEvent JSON>
```

事件包含身份、路由、模型、请求摘要、policy、状态码、流式标记、gate 标记、
开始/结束时间和插件版本。策略允许时，事件可以包含用户内容、助手内容、usage、
finish reason 以及安全的 provider/runtime 事实。事件不得包含 Authorization
请求头、API Key、内部 bearer token、Redis 凭据、上游 provider 凭据或敏感原文。
Redis Stream 投递失败时用户响应保持不变。

Console 负责 materialization、embedding、vector search、持久回放记录、
semantic lookup、pricing、settlement、repair、deletion 和 reconciliation。
后端状态必须隔离：cache 和 memory 使用独立 PostgreSQL 表、Redis Stream、
Redis key 前缀、向量集合、payload schema、保留/删除策略、管理 API 和鉴权边界。

平台自有的 cache 和 memory 模型工作应通过 Console 管理的内部 AI Route 调用，
例如 `ai-cache.embedding`、`ai-cache.rerank`、`ai-memory.digest`、
`ai-memory.embedding`。这些调用应发出 `event_kind=internal_cost` 用于平台成本归因，
不能成为普通客户 usage 账单。

当 `ai-memory` 会影响回答时，推荐插件顺序为 `ai-memory` 先于 `ai-cache`。
推荐的缓存策略包括 consumer 级作用域、使用 `policy_digest` 将 memory policy
version 和组装 memory digest 纳入 key/policy 材料，或在 Console 尚未物化
memory-aware 回放记录前使用 `bypass`。

缓存回放时，`ai-cache` 会设置可信网关事实，例如 `upstream_invoked=false`，供
`ai-billing` 消费。真实上游 provider 调用会发出 `upstream_invoked=true`。这些事实
不会加入用户可见的 OpenAI 响应体，用户请求或响应体中的同名字段也不能控制它们。

## 兼容性路径

下方 `vector`、`embedding`、`cache` 配置描述的是旧的在线 embedding/vector 或
文本匹配兼容路径。该路径仍可用于本地或显式非生产兼容场景，但 Modelfusion 生产
缓存回放应使用上文的结构化薄缓存路径，并由 Console 负责物化。

## 向量数据库服务（vector）
| Name | Type | Requirement | Default | Description |
| --- | --- | --- | --- | --- |
| vector.type | string | required | - | 向量存储服务提供者类型，例如 dashvector、chroma、elasticsearch、weaviate、pinecone、qdrant、milvus |
| vector.serviceName | string | required | - | 向量存储服务名称 |
| vector.serviceHost | string | optional | - | 向量存储服务域名。部分 provider（如 dashvector、pinecone）要求必填 |
| vector.servicePort | int64 | optional | 443 | 向量存储服务端口 |
| vector.apiKey | string | optional | - | 向量存储服务 API Key |
| vector.topK | int | optional | 1 | 返回TopK结果 |
| vector.timeout | uint32 | optional | 10000 | 请求向量存储服务的超时时间，单位为毫秒。默认值是10000，即10秒 |
| vector.collectionID | string | optional | - | 向量存储服务 Collection ID |
| vector.threshold | float64 | optional | 1000 | 向量相似度度量阈值 |
| vector.thresholdRelation | string | optional | "lt" | 相似度度量比较方式。相似度度量方式有 `Cosine`, `DotProduct`, `Euclidean` 等，前两者值越大相似度越高，后者值越小相似度越高。对于 `Cosine` 和 `DotProduct` 选择 `gt`，对于 `Euclidean` 则选择 `lt`。所有可选值包括 `lt` (less than，小于)、`lte` (less than or equal to，小等于)、`gt` (greater than，大于)、`gte` (greater than or equal to，大等于) |
| vector.esUsername | string | optional | - | ElasticSearch 用户名，仅用于 elasticsearch 类型 |
| vector.esPassword | string | optional | - | ElasticSearch 密码，仅用于 elasticsearch 类型 |

## 文本向量化服务（embedding）
| Name | Type | Requirement | Default | Description |
| --- | --- | --- | --- | --- |
| embedding.type | string | required | - | 请求文本向量化服务类型，例如 dashscope、openai、azure、cohere、ollama、huggingface、textin、xfyun |
| embedding.serviceName | string | required | - | 请求文本向量化服务名称 |
| embedding.serviceHost | string | optional | - | 请求文本向量化服务域名 |
| embedding.servicePort | int64 | optional | 443 | 请求文本向量化服务端口。不同 provider 默认值可能不同，ollama 默认为 11434 |
| embedding.timeout | uint32 | optional | 10000 | 请求文本向量化服务的超时时间，单位为毫秒。默认值是10000，即10秒 |
| embedding.model | string | optional | - | 请求文本向量化服务的模型名称 |
| embedding.apiKey | string | optional | - | 请求文本向量化服务的 API Key |


## 缓存服务（cache）
| Name | Type | Requirement | Default | Description |
| --- | --- | --- | --- | --- |
| cache.type | string | required | - | 缓存服务类型，例如 redis |
| cache.serviceName | string | required | - | 缓存服务名称 |
| cache.serviceHost | string | optional | - | 缓存服务域名 |
| cache.servicePort | int64 | optional | 6379 | 缓存服务端口。若 serviceName 以 .static 结尾，则默认值为 80 |
| cache.username | string | optional | - | 缓存服务用户名 |
| cache.password | string | optional | - | 缓存服务密码 |
| cache.timeout | uint32 | optional | 10000 | 缓存服务的超时时间，单位为毫秒。默认值是10000，即10秒 |
| cache.cacheTTL | int | optional | 0 | 缓存过期时间，单位为秒。默认值是 0，即永不过期 |
| cache.cacheKeyPrefix | string | optional | "higress-ai-cache:" | 缓存 Key 的前缀 |
| cache.database | int | optional | 0 | 使用的数据库id，仅限redis，例如配置为1，对应`SELECT 1` |


## 其他配置
| Name | Type | Requirement | Default                                                                                                                                                                                                                                                     | Description |
| --- | --- | --- | --- | --- |
| cacheKeyFrom | string | optional | "messages.@reverse.0.content"                                                                                                                                                                                                                               | 从请求 Body 中基于 [GJSON PATH](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) 语法提取字符串 |
| cacheValueFrom | string | optional | "choices.0.message.content"                                                                                                                                                                                                                                 | 从响应 Body 中基于 [GJSON PATH](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) 语法提取字符串 |
| cacheStreamValueFrom | string | optional | "choices.0.delta.content"                                                                                                                                                                                                                                   | 从流式响应 Body 中基于 [GJSON PATH](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) 语法提取字符串 |
| cacheToolCallsFrom | string | optional | "choices.0.delta.content.tool_calls"                                                                                                                                                                                                                        | 从流式响应 Body 中基于 [GJSON PATH](https://github.com/tidwall/gjson/blob/master/SYNTAX.md) 语法提取字符串 |
| responseTemplate | string | optional | `{"id":"from-cache","choices":[{"index":0,"message":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`                       | 返回 HTTP 响应的模版，用 %s 标记需要被 cache value 替换的部分 |
| streamResponseTemplate | string | optional | `data:{"id":"from-cache","choices":[{"index":0,"delta":{"role":"assistant","content":"%s"},"finish_reason":"stop"}],"model":"from-cache","object":"chat.completion","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}\n\ndata:[DONE]\n\n`| 返回流式 HTTP 响应的模版，用 %s 标记需要被 cache value 替换的部分 |

## 文本向量化提供商特有配置

### Azure OpenAI

Azure OpenAI 所对应的 `embedding.type` 为 `azure`。它需要提前创建[Azure OpenAI 账户](https://portal.azure.com/#view/Microsoft_Azure_ProjectOxford/CognitiveServicesHub/~/overview)，然后您需要在[Azure AI Foundry](https://ai.azure.com/resource/deployments)中挑选一个模型并将其部署，点击您部署好的模型，您可以在终结点中看到目标 URI 以及密钥。请将 URI 中的 host 填入`embedding.serviceHost`，密钥填入`embedding.apiKey`。

一个完整的 URI 示例为 https://YOUR_RESOURCE_NAME.openai.azure.com/openai/deployments/YOUR_DEPLOYMENT_NAME/embeddings?api-version=2024-10-21，您需要将`YOUR_RESOURCE_NAME.openai.azure.com`填入`embedding.serviceHost`。

它特有的配置字段如下：

| 名称 | 数据类型 | 填写要求 | 默认值 | 描述 |
| ---------------------- | -------- | -------- | ------ | ------- |
| `embedding.apiVersion` | string | 必填 | - | api版本，获取到的URI中api-version的值 |

需要注意的是您必须要指定`embedding.serviceHost`，如`YOUR_RESOURCE_NAME.openai.azure.com`。模型默认使用了`text-embedding-ada-002`，如需其他模型，请在`embedding.model`中进行指定。

### Cohere

Cohere 所对应的 `embedding.type` 为 `cohere`。它并无特有的配置字段。需要提前创建 [API Key](https://docs.cohere.com/reference/embed)，并将其填入`embedding.apiKey`。

### OpenAI

OpenAI 所对应的 `embedding.type` 为 `openai`。它并无特有的配置字段。需要提前创建 [API Key](https://platform.openai.com/settings/organization/api-keys)，并将其填入`embedding.apiKey`，一个 API Key 的示例为`sk-xxxxxxx`。

### Ollama

Ollama 所对应的 `embedding.type` 为 `ollama`。它并无特有的配置字段。

### Hugging Face

Hugging Face 所对应的 `embedding.type` 为 `huggingface`。它并无特有的配置字段。需要提前创建 [hf_token](https://huggingface.co/blog/getting-started-with-embeddings)，并将其填入`embedding.apiKey`，一个 hf_token 的示例为`hf_xxxxxxx`。

`embedding.model`默认指定为`sentence-transformers/all-MiniLM-L6-v2`

### DashScope

DashScope 所对应的 `embedding.type` 为 `dashscope`。需要提前创建 [API Key](https://help.aliyun.com/document_detail/2712195.html)，并将其填入`embedding.apiKey`。

`embedding.model`默认指定为`text-embedding-v2`，还可选用`text-embedding-v1`等模型。

### TextIn

TextIn 所对应的 `embedding.type` 为 `textin`。它需要提前获取[`app-id` 和`secret-code`](https://www.textin.com/document/acge_text_embedding)。

它特有的配置字段如下：

| 名称 | 数据类型 | 填写要求 | 默认值 | 描述 |
| ------------------------------- | -------- | -------- | ------ | ------------------ |
| `embedding.textinAppId` | string | 必填 | - | 应用 ID，获取的 app-id |
| `embedding.textinSecretCode` | string | 必填 | - | 调用 API 所需 Secret，获取的 secret-code |
| `embedding.textinMatryoshkaDim` | int | 必填 | - | 返回的单个向量长度 |

### 讯飞星火

讯飞星火 所对应的 `embedding.type` 为 `xfyun`。它需要提前创建[应用](https://console.xfyun.cn/services/emb)，获取`APPID`、`APISecret`和`APIKey`，并将`APIKey`填入`embedding.apiKey`中。

它特有的配置字段如下：

| 名称 | 数据类型 | 填写要求 | 默认值 | 描述 |
| --------------------- | -------- | -------- | ------ | -------------------- |
| `embedding.appId` | string | 必填 | - | 应用 ID，获取的 APPID |
| `embedding.apiSecret` | string | 必填 | - | 调用 API 所需 Secret，获取的 APISecret |

## 向量数据库提供商特有配置

### Chroma
Chroma 所对应的 `vector.type` 为 `chroma`。它并无特有的配置字段。需要提前创建 Collection，并填写 Collection ID 至配置项 `vector.collectionID`，一个 Collection ID 的示例为 `52bbb8b3-724c-477b-a4ce-d5b578214612`。

### DashVector
DashVector 所对应的 `vector.type` 为 `dashvector`。它并无特有的配置字段。需要提前创建 Collection，并填写 `Collection 名称` 至配置项 `vector.collectionID`。

### ElasticSearch
ElasticSearch 所对应的 `vector.type` 为 `elasticsearch`。需要提前创建 Index 并填写 Index Name 至配置项 `vector.collectionID` 。

当前依赖于 [KNN](https://www.elastic.co/guide/en/elasticsearch/reference/current/knn-search.html) 方法，请保证 ES 版本支持 `KNN`，当前已在 `8.16` 版本测试。

它特有的配置字段如下：
| 名称 | 数据类型 | 填写要求 | 默认值 | 描述 |
|-------------------|----------|----------|--------|-------------------------------------------------------------------------------|
| `vector.esUsername` | string | 非必填 | - | ElasticSearch 用户名 |
| `vector.esPassword` | string | 非必填 | - | ElasticSearch 密码 |


`vector.esUsername` 和 `vector.esPassword` 用于 Basic 认证。同时也支持 Api Key 认证，当填写了 `vector.apiKey` 时，则启用 Api Key 认证，如果使用 SaaS 版本需要填写 `encoded` 的值。

### Milvus
Milvus 所对应的 `vector.type` 为 `milvus`。它并无特有的配置字段。需要提前创建 Collection，并填写 Collection Name 至配置项 `vector.collectionID`。

### Pinecone
Pinecone 所对应的 `vector.type` 为 `pinecone`。它并无特有的配置字段。需要提前创建 Index，并填写 Index 访问域名至 `vector.serviceHost`。

Pinecone 中的 `Namespace` 参数通过插件的 `vector.collectionID` 进行配置，如果不填写 `vector.collectionID`，则默认为 Default Namespace。

### Qdrant
Qdrant 所对应的 `vector.type` 为 `qdrant`。它并无特有的配置字段。需要提前创建 Collection，并填写 Collection Name 至配置项 `vector.collectionID`。

### Weaviate
Weaviate 所对应的 `vector.type` 为 `weaviate`。它并无特有的配置字段。需要提前创建 Collection，并填写 Collection Name 至配置项 `vector.collectionID`。

需要注意的是 Weaviate 会设置首字母自动大写，在填写配置 `collectionID` 的时候需要将首字母设置为大写。

如果使用 SaaS 需要填写 `vector.serviceHost` 参数。

## 配置示例
### 基础配置
```yaml
embedding:
  type: dashscope
  serviceName: my_dashscope.dns
  apiKey: [Your Key]

vector:
  type: dashvector
  serviceName: my_dashvector.dns
  collectionID: [Your Collection ID]
  serviceHost: [Your domain]
  apiKey: [Your key]

cache:
  type: redis
  serviceName: my_redis.dns
  servicePort: 6379
  timeout: 100

```

## 进阶用法
当前默认的缓存 key 是基于 GJSON PATH 的表达式：`messages.@reverse.0.content` 提取，含义是把 messages 数组反转后取第一项的 content；

GJSON PATH 支持条件判断语法，例如希望取最后一个 role 为 user 的 content 作为 key，可以写成： `messages.@reverse.#(role=="user").content`；

如果希望将所有 role 为 user 的 content 拼成一个数组作为 key，可以写成：`messages.@reverse.#(role=="user")#.content`；

还可以支持管道语法，例如希望取到数第二个 role 为 user 的 content 作为 key，可以写成：`messages.@reverse.#(role=="user")#.content|1`。

更多用法可以参考[官方文档](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)，可以使用 [GJSON Playground](https://gjson.dev/) 进行语法测试。

## 常见问题

1. 如果返回的错误为 `error status returned by host: bad argument`，请检查`serviceName`是否正确包含了服务的类型后缀(.dns等)。
