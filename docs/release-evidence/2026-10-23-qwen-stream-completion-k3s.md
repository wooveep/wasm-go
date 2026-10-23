# ai-proxy 1.0.3：Qwen 原生流完成标记

日期：2026-10-23。默认 `modelfusion-dev` VM K3S + Helm，namespace `higress-system`。修复和真实调用回归通过。

## 问题与修复

DashScope 原生聊天流以带 `finish_reason` 的响应结束，没有 OpenAI 的 `data: [DONE]`。ai-proxy 原实现转换出了内容、结束原因和 usage，却未生成结束标记。依赖 OpenAI SSE 完成语义的 UserWeb 模型体验因此不能可靠完成。

Qwen 原生适配器现在检查已转换的 choice 是否带明确结束原因，在该事件的转换帧和 usage 帧之后输出 `[DONE]`。兼容接口及媒体转换保持原有处理；没有结束原因的截断流不生成完成标记。GitNexus 对此接口方法报告 CRITICAL，主要来自接口动态派发的广泛调用关系，修改前已提示并检查 Qwen 分支。

新增真实转换管线回归覆盖三个输入分片位置，验证内容、`stop`、usage 和末尾唯一 `[DONE]`；另覆盖截断流保留内容、但不伪造完成。三个完成场景在修改前因缺少 DONE 失败，修复后四个场景均通过。

## 发布与验证

- ai-proxy 版本、配置注解、资源 schema、README 和相关制品示例统一到 1.0.3。最低 Higress 插件兼容要求保持 1.0.2。
- 以 Go 1.26.3 构建，Proxy-Wasm Go SDK 使用工作区提交 `662ed04`；模块替换只存在本地临时 go.work，未把本地路径写入 Git。
- Go 根项目：7 个包、157 个通过项（含子测试）、4 个既有跳过项；ai-proxy：4 个包、1,983 个通过项、1 个既有跳过项。截断流子例增加后，专项四场景再次通过。
- 合并既有 cache / history / billing 修复和发布版本信息后，完整复验再次通过：根项目 159 项（4 个既有跳过项），ai-proxy 1,984 项（1 个既有跳过项）、ai-cache 197 项、ai-history 55 项、ai-billing 295 项；均包含子测试，具体跳过计数见结构化记录。ai-proxy 显式 `go vet ./...` 通过。后续发布信息对齐没有重新上传或部署制品。
- Wasm 9,891,306 字节，SHA-256 `ff2a05b962e95665d9f1ecf23ba010871c265b8a1b678e312b7dda2ed081d686`。
- 上传新路径 `/plugins/ai-proxy/1.0.3/plugin.wasm` 返回 201；下载返回 200 且哈希一致，未覆盖旧版本路径。
- 默认 K3S 普通调用与 SSE 均 200；SSE 包含内容、usage、`stop` 和 `[DONE]`。最终原生 SSE 样例输入 13、输出 1 Token。
- UserWeb 模型体验显示 OK 与“生成完成”，网关请求 ID `7dd9a461-0ee7-4063-9fdc-0e8be7c68002` 对应一笔已结算事件及账户流水。此请求使用专用合成计价：客户 ¥0.00003、上游 ¥0.000015，不代表供应商官方报价。

结构化结果见 [脱敏证据](2026-10-23-qwen-stream-completion-k3s.json)。平台侧共享绑定保存修复、镜像与完整财务对账由 ocloudmodelfusion 的 `docs/bugs/2026-10-23-main-k3s-e2e.md` 维护；本报告只对本插件的转换、制品和真实请求作结论。凭据、JWT、原始响应体及完整 provider 配置未入库。
