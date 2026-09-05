# 2026-09-03 Sticky Account First-Token Investigation

## Symptom

用户反馈：分组内每个账号单独测试首字很快、渠道状态为绿，但实际请求首字常等待 10-20 秒。

## Root cause

1. Anthropic `/v1/messages` 的流式处理此前把第一个任意 SSE data 帧作为 `firstTokenMs`。`message_start` 和 `content_block_start` 是元数据帧，不代表实际文本、思考或工具参数输出，因此调度器会得到虚假的低 TTFT，sticky 账号不会被慢账号逃逸逻辑识别。
2. 账号池调度在 sticky 账号槽位忙时，直接返回 sticky 等待计划（默认最长 120 秒），不会先尝试同一分组内其它可用账号。组内其它账号即使槽位空闲，也会被这条路径跳过。
3. 生产只读日志显示账号 6 曾多次在约 120 秒后收到 Cloudflare 524，账号 29 的 Cloudflare 502 也在请求开始约 20 秒后出现；生产机器资源正常，容器 healthy。渠道监控的绿色状态不能代表真实用户消息流在高并发下的首字延迟。

## Fix

- `gateway_upstream_response.go` 仅在 Anthropic `text_delta`、`thinking_delta`、非空 `input_json_delta` 或工具块开始事件时记录 TTFT。
- `gateway_service.go` 新增 Anthropic 语义输出判定函数。
- `gateway_scheduling.go` 在多账号组内 sticky 槽位忙时继续走组内负载选择；单账号组仍保留 sticky 等待计划。
- `gateway_streaming_test.go` 新增语义输出分类回归测试；`gateway_multiplatform_test.go` 覆盖多账号 sticky 忙时切换及模型路由行为。

## Verification

- `git diff --check`: passed.
- 生产只读资源检查：load average 0.12/0.20/0.23，内存可用约 2.3 GiB，根分区使用 31%，`sub2api` healthy。
- 使用仓库内 `.tmp-go27` Go 工具链，并临时 overlay 两份既有签名失配测试及 Ent runtime 初始化文件后，`TestAnthropicStreamDataStartsSemanticOutput` 与 `TestGatewayService_SelectAccountWithLoadAwareness` 通过。完整 `internal/service` 测试仍失败于既有 Ent 默认值初始化、账号复制字段断言和身份绑定测试；overlay 文件已清理，未修改原始基线文件。

## Residual risk

当前修复改善账号选择和 TTFT 统计，但 Anthropic 流式路径仍没有独立的“首字超时即切换”看门狗；若上游在返回 HTTP 头后长期不输出，仍可能等待通用流间隔超时。发布前应在非生产环境补充该场景的端到端测试，再决定是否增加可配置首字超时。
