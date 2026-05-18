---
title: AI 金额配额与账单插件需求说明
keywords: [AI网关, AI配额, AI账单, ai-quota, ai-billing]
description: ai-quota 金额模式重构与 ai-billing 新增插件的需求说明
---

# AI 金额配额与账单插件需求说明

## 1. 背景

当前 `ai-quota` 以 token quota 为核心，并通过插件自身暴露的 HTTP 管理接口完成 quota 查询、刷新和增减。这种方式把管理面能力放在网关插件内，不适合后续与 console 的账户、余额、价格和账单流水体系对接。

新的目标是将 `ai-quota` 重构为金额余额准入插件，并新增独立的 `ai-billing` 插件生成账单事件。账单事件由 `ai-billing` 上报，但余额真实源不是单个事件本身，而是 billing-service DB 中的账单流水和账户余额投影。网关插件只在请求路径上读取 Redis 热数据并上报事件。

## 2. 目标

- `ai-quota` 直接改造为金额模式，不再支持 token quota 作为目标模型。
- `ai-quota` 不再通过 HTTP 接口管理 quota。
- `ai-quota` 不再需要 `admin_consumer`。
- `ai-quota` 使用 Higress WasmPlugin `matchRules` 绑定不同 AI 路由的 `quota_scope`。
- `ai-quota` 请求前读取 Redis 热余额，采用轻量金额准入模式。
- `ai-quota` 响应结束后基于真实 token usage 和 Redis 价格缓存计算费用，并扣减 Redis 热余额。
- 新增 `ai-billing` 插件，独立生成 billing event 并投递到 console billing-service。
- `ai-statistics`、`ai-quota`、`ai-billing` 三个插件独立启用，不共享插件运行时状态。
- billing-service 作为账单事件、账单流水、余额投影、价格策略和幂等处理的事实源。
- 支持多租户价格策略，不同租户可以对同一 provider/model 使用不同折扣或覆盖价格。

## 3. 非目标

- `ai-quota` v1 不做预冻结和严格防透支。
- `ai-quota` v1 不直接访问数据库。
- `ai-quota` v1 不通过 HTTP callout 查询余额或价格。
- `ai-billing` 不直接扣减账户余额。
- `ai-statistics` 不作为计费事实来源。
- 本需求不包含 console UI 的详细页面设计。

## 4. 整体架构

```text
             ┌─────────────────────────────┐
             │          Console             │
             │ tenant / consumer / route     │
             │ balance / price / statement   │
             └──────────────┬──────────────┘
                            │ 管理与刷新
                            ▼
             ┌─────────────────────────────┐
             │      billing-service         │
             │ DB 事实源 / 幂等 / 结算 / 补偿 │
             └───────┬──────────────┬──────┘
                     │              │
              刷新 Redis       接收 billing event
                     │              ▲
                     ▼              │
┌────────────────────────────────────────────────────────┐
│                     Higress Gateway                    │
│                                                        │
│ key-auth / jwt-auth                                    │
│   └─ 写入 x-mse-tenant / x-mse-consumer                 │
│                                                        │
│ ai-quota                                               │
│   ├─ matchRules 命中 route/domain/service 配置          │
│   ├─ 读取 quota_scope                                  │
│   ├─ Redis 热余额准入                                  │
│   ├─ Redis 租户生效价格计算费用                         │
│   └─ 响应结束后轻量扣减金额余额                         │
│                                                        │
│ ai-billing                                             │
│   ├─ 独立解析 token usage                              │
│   └─ 投递 billing event 到 billing-service              │
│                                                        │
│ ai-statistics                                          │
│   ├─ 独立解析 token usage                              │
│   └─ 写日志、metric、span                               │
└────────────────────────────────────────────────────────┘
```

## 5. 插件职责边界

| 组件 | 职责 | 不应承担 |
| --- | --- | --- |
| `ai-quota` | 金额余额准入、响应后按金额扣减 Redis 热余额 | 管理 quota、管理 consumer、管理租户价格策略、写账单流水、访问 DB |
| `ai-billing` | 生成请求级账单事件并投递 billing-service | 扣余额、管理价格、管理账户 |
| `ai-statistics` | 可观测：日志、指标、span、token 统计 | 配额判断、账单事实源、扣费 |
| billing-service | 事件入库、价格策略、流水、余额投影、幂等、补偿、Redis 刷新 | 网关请求路径内同步阻塞 |
| console | 管理 tenant、consumer、quota_scope、价格、折扣、余额和账单查询 | 网关侧实时扣费 |

三个 AI 插件必须共享 `pkg/tokenusage` 解析库和字段约定，但不能共享同一请求上下文中的插件私有状态。

```text
tokenusage.GetTokenUsage
        ▲
        │
 ┌──────┼─────────────┐
 │      │             │
quota  billing    statistics
```

## 6. 请求处理流程

### 6.1 请求头阶段

```text
1. key-auth / jwt-auth / 其他认证插件校验请求身份。
2. 认证插件写入 `x-mse-tenant` 和 `x-mse-consumer`。
3. ai-quota 根据 Higress WasmPlugin matchRules 获取当前路由配置。
4. ai-quota 从配置读取 quota_scope、provider 等信息。
5. ai-quota 拼接余额 Redis key。
6. ai-quota GET balance。
7. balance > 0 时放行。
8. balance <= 0 或余额缺失时按 missing_balance_policy 处理，默认拒绝。
```

### 6.2 响应体阶段

```text
1. ai-quota 调用 tokenusage.GetTokenUsage 解析 input/output/total tokens 和 model。
2. ai-quota 在 endOfStream 或非流式响应结束时获取最终 usage。
3. ai-quota 从 Redis 读取 tenant/provider/model 对应输入、输出生效价格。
4. ai-quota 计算本次请求费用。
5. ai-quota 扣减 billing:balance:{tenant}:{quota_scope}:{consumer}。
6. ai-billing 独立解析 usage 并投递 billing event。
7. ai-statistics 独立解析 usage 并写观测数据。
```

## 7. ai-quota 重构需求

### 7.1 移除能力

- 移除 HTTP quota 管理接口：
  - `/quota`
  - `/quota/refresh`
  - `/quota/delta`
- 移除 `admin_consumer` 配置。
- 移除 `admin_path` 配置。
- 移除管理员身份判断。
- 移除通过请求 body 调整 quota 的逻辑。
- 移除 `ChatModeAdmin`、`AdminMode*` 相关逻辑。
- 删除或改写文档中通过 HTTP 管理 quota 的说明。

### 7.2 新配置

```yaml
redis:
  service_name: redis-service.default.svc.cluster.local
  service_port: 6379
  username: ""
  password: ""
  timeout: 1000
  database: 0

quota_scope: "global"
provider: ""
tenant_header: "x-mse-tenant"
consumer_header: "x-mse-consumer"
balance_key_template: "billing:balance:{tenant}:{quota_scope}:{consumer}"
price_key_template: "billing:effective_price:{tenant}:{provider}:{model}:{token_type}"
amount_scale: 1000000
price_unit_tokens: 1000000
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages

missing_balance_policy: "deny"
missing_price_policy: "skip"
missing_usage_policy: "skip"
```

配置说明：

| 名称 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `redis` | object | 无 | Redis 连接配置 |
| `quota_scope` | string | `global` | 当前配置对应的额度作用域 |
| `provider` | string | 空 | AI 提供商标识，用于价格 key |
| `tenant_header` | string | `x-mse-tenant` | 租户身份请求头 |
| `consumer_header` | string | `x-mse-consumer` | consumer 身份请求头 |
| `balance_key_template` | string | `billing:balance:{tenant}:{quota_scope}:{consumer}` | 余额 key 模板 |
| `price_key_template` | string | `billing:effective_price:{tenant}:{provider}:{model}:{token_type}` | 租户生效价格 key 模板 |
| `amount_scale` | int | `1000000` | 金额缩放比例，例如 1 元 = 1000000 micro_yuan |
| `price_unit_tokens` | int | `1000000` | 价格单位 token 数，默认每 100 万 token 价格 |
| `enable_path_suffixes` | []string | `/v1/chat/completions`, `/v1/messages` | 生效路径后缀 |
| `missing_balance_policy` | string | `deny` | 余额缺失策略：`deny` 或 `allow` |
| `missing_price_policy` | string | `skip` | 价格缺失策略：`skip` 或 `deny` |
| `missing_usage_policy` | string | `skip` | usage 缺失策略：v1 默认 `skip` |

### 7.3 路由级配置

不同 AI 路由通过 Higress WasmPlugin `matchRules` 指定不同 `quota_scope`。

```yaml
apiVersion: extensions.higress.io/v1alpha1
kind: WasmPlugin
metadata:
  name: ai-quota
  namespace: higress-system
spec:
  defaultConfig:
    redis:
      service_name: redis-service.default.svc.cluster.local
      service_port: 6379
    quota_scope: global
    balance_key_template: "billing:balance:{tenant}:{quota_scope}:{consumer}"
    price_key_template: "billing:effective_price:{tenant}:{provider}:{model}:{token_type}"
    amount_scale: 1000000
    price_unit_tokens: 1000000
  matchRules:
    - ingress:
        - ai/qwen-route
      config:
        quota_scope: "route:qwen"
        provider: "dashscope"
    - ingress:
        - ai/deepseek-route
      config:
        quota_scope: "route:deepseek"
        provider: "deepseek"
  phase: UNSPECIFIED_PHASE
  priority: 280
```

### 7.4 Redis key 约定

```text
billing:balance:{tenant}:{quota_scope}:{consumer}
billing:effective_price:{tenant}:{provider}:{model}:input
billing:effective_price:{tenant}:{provider}:{model}:output
billing:price_version:{tenant}
billing:idempotency:{request_id}
```

后续预冻结扩展可增加：

```text
billing:reservation:{tenant}:{quota_scope}:{consumer}:{request_id}
```

### 7.5 金额单位和价格计算

余额和价格必须使用整数，避免浮点误差。推荐使用 micro currency：

```text
1 元 = 1,000,000 micro_yuan
```

价格建议存储为每 `price_unit_tokens` token 的 micro 金额：

```text
billing:effective_price:tenant-a:dashscope:qwen-turbo:input  = 800000
billing:effective_price:tenant-a:dashscope:qwen-turbo:output = 2000000
```

计算公式：

```text
input_cost = ceil(input_tokens * input_price / price_unit_tokens)
output_cost = ceil(output_tokens * output_price / price_unit_tokens)
total_cost = input_cost + output_cost
```

### 7.6 扣减策略

v1 使用轻量金额模式：

```text
请求前：
  balance > 0 放行

响应结束：
  usage 存在且 price 命中：
    计算 cost
    扣减 balance
  usage 缺失：
    默认 skip，不扣费，记录日志或指标
  price 缺失：
    默认 skip，不扣费，记录日志或指标
```

扣减建议使用 Redis Lua，将读取价格、计算金额、扣减余额放在一次 Redis 操作内，减少单次请求内的不一致窗口。

### 7.7 多租户价格策略

不同租户可以对同一 provider/model 使用不同价格。价格规则不在 `ai-quota` 内实现，必须由 billing-service 在 DB 中管理并计算租户生效价格。

```text
基础价格：
  provider + model + token_type -> base_price

租户价格策略：
  tenant + provider + model -> discount 或 override_price

Redis 生效价格：
  billing:effective_price:{tenant}:{provider}:{model}:{token_type}
```

`ai-quota` 只读取 Redis 中的生效价格，不理解折扣规则。Redis 重启或缓存丢失后，billing-service 必须从 DB 中的基础价格和租户价格策略重新生成生效价格 key。

## 8. ai-billing 新增插件需求

### 8.1 职责

- 读取 `x-mse-consumer`。
- 读取 `x-mse-tenant`。
- 获取 `quota_scope`、provider、model、route、cluster、request path、status code。
- 记录 `request_id`、`started_at`、`ended_at`。
- 独立调用 `tokenusage.GetTokenUsage`。
- 生成 billing event。
- 通过 HTTP callout 投递到 billing-service。
- 默认 fail-open，不阻断用户响应。
- callout 超时建议 100 到 500 ms。

### 8.2 配置

```yaml
billing_service:
  service_name: billing-service.default.svc.cluster.local
  service_port: 8080
  path: /v1/billing/events
  timeout: 300

quota_scope: "global"
provider: ""
tenant_header: "x-mse-tenant"
consumer_header: "x-mse-consumer"
enable_path_suffixes:
  - /v1/chat/completions
  - /v1/messages
fail_policy: "open"
```

### 8.3 billing event

```json
{
  "request_id": "req-001",
  "idempotency_key": "consumer-a:route:qwen:req-001",
  "tenant": "tenant-a",
  "consumer": "consumer-a",
  "quota_scope": "route:qwen",
  "provider": "dashscope",
  "model": "qwen-turbo",
  "route": "ai/qwen-route",
  "cluster": "outbound|443||dashscope.example.com",
  "input_tokens": 10,
  "output_tokens": 100,
  "total_tokens": 110,
  "stream": true,
  "status_code": 200,
  "started_at": 1730000000000,
  "ended_at": 1730000001200,
  "usage_missing": false,
  "price_version": "2026-05-01",
  "gateway_calculated_cost": 220
}
```

说明：

- `gateway_calculated_cost` 可选，用于 billing-service 与网关侧扣减结果对账。
- billing-service 必须以服务端租户价格策略、账单流水和 DB 事务为最终账务事实。
- 如果 usage 缺失，`usage_missing` 为 `true`，token 字段为 0 或缺省。

### 8.4 失败处理

- 默认 `fail_policy: open`。
- billing-service 超时、返回 5xx、网络失败时，不阻断用户响应。
- 插件记录错误日志和失败指标。
- 后续可选通过 Redis Stream 投递补偿事件。

## 9. ai-statistics 协作要求

- `ai-statistics` 继续只负责日志、metric、span。
- `ai-statistics` 独立调用 `tokenusage.GetTokenUsage`。
- `ai-statistics` 关闭时，`ai-quota` 和 `ai-billing` 必须仍可工作。
- 文档中不得描述 `ai-quota` 依赖 `ai-statistics` 获取 token usage。

## 10. console 与 billing-service 协作要求

console 负责管理：

- consumer 账户。
- tenant 与 consumer 归属关系。
- `quota_scope`。
- AI 路由与 `quota_scope` 的绑定。
- consumer 在不同 `quota_scope` 下的金额余额。
- provider/model 基础价格表。
- tenant 级折扣和覆盖价格。
- 账单流水查询。
- 充值、扣减、调整记录。
- usage 缺失、价格缺失等异常事件处理。

billing-service 负责：

- DB 事实源。
- billing event 入库。
- 账单流水写入。
- 账户余额投影表更新。
- Redis 热余额刷新。
- Redis 租户生效价格刷新。
- billing event 幂等处理。
- 余额校准和补偿。

余额真实源要求：

```text
billing event -> billing-service 结算 -> billing_statements 流水 -> account_balances 余额投影 -> Redis 热余额
```

`account_balances` 可以保存当前余额以便快速查询，但必须由账单流水、充值流水、调整流水在同一事务内更新。Redis 只作为热缓存，不能作为真实源。

Redis 重启恢复要求：

```text
1. billing-service 从 account_balances 重建 billing:balance:*。
2. billing-service 从基础价格和租户价格策略重建 billing:effective_price:*。
3. billing-service 写入 billing:price_version:{tenant}。
4. 重建完成前，ai-quota 遇到缺失余额默认 deny，遇到缺失价格默认 skip 或按配置 deny。
```

## 11. 验收标准

- `ai-quota` 不再暴露 quota 管理 HTTP 接口。
- `ai-quota` 不再要求 `admin_consumer`。
- `ai-quota` 余额 key 存储金额余额，不存 token quota。
- 同一个 tenant 下的同一个 consumer 可以在不同 `quota_scope` 下拥有不同金额余额。
- 不同 tenant 可以对同一 provider/model 使用不同生效价格。
- `matchRules` 命中不同 AI 路由时，`ai-quota` 使用不同 `quota_scope`。
- 请求前 Redis 热余额大于 0 时放行。
- 请求前 Redis 热余额小于等于 0 时默认拒绝。
- 响应结束后有 usage 且价格命中时，`ai-quota` 按金额扣减余额。
- usage 缺失时默认不扣费，不误扣。
- price 缺失时默认不扣费，不误扣。
- `ai-billing` 可以独立上报 billing event。
- Redis 重启后，billing-service 可以从 DB 重建余额和租户生效价格缓存。
- `ai-statistics`、`ai-quota`、`ai-billing` 任意一个关闭，不影响另外两个插件的基本职责。

## 12. 后续增强

- 预冻结和解冻，支持严格不透支。
- Redis Stream 失败补偿队列。
- billing-service 与网关侧扣减差异自动校准。
- 按固定费用、图片数量、缓存 token、推理 token 等扩展计费维度。
- price version 灰度和回滚。
