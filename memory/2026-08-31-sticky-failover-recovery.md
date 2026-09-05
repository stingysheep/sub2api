# 调度调查：故障切换后不回高优先级账号

- **症状**：分组内高优先级账号短暂失败，切到后备账号；高优先级账号恢复后，后续请求仍长期使用后备账号。
- **根因假设（已由代码路径支持）**：故障账号排除集合仅存在于当前请求。切换后的后备账号在选号成功后写入 session -> account 粘性绑定；标准粘性 TTL 为 1 小时，命中后续期。普通调度和 OpenAI 非 sticky-weighted 调度在粘性账号可用时直接返回，不比较更高优先级候选，因此恢复账号不会被重新评估。
- **证据**：`backend/internal/service/gateway_service.go` 与 `backend/internal/service/openai_gateway_service.go` 定义 1 小时粘性 TTL；`gateway_scheduling.go`、`openai_gateway_scheduling.go` 的粘性层先于优先级排序；各 HTTP/WS failover handler 的失败账号集合为请求局部变量。
- **风险**：仅缩短 TTL 或每次强制清粘性会破坏会话连续性/提示缓存；先进的 sticky-weighted 模式只提供加权偏好，不能保证恢复后立即回切。
- **推荐方向**：增加“故障切换后粘性回切”策略：仅对曾经发生故障切换的会话，在下一次新请求中检查当前健康候选的最高优先级；高优先级账号恢复且通过完整准入检查时回切并更新绑定，否则继续后备账号。长连接在当前连接内不强制换号，下一 turn/重连时评估。
- **状态**：待用户确认实施策略；尚未修改源码、尚未发布生产。
