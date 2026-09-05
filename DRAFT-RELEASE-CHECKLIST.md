# 发布前检查清单

状态：仅完成公开仓库与分支整理；未发布生产。
日期：2026-09-05

## 分支

- `main`：官方上游 v0.2.1 基线。
- `custom`：在官方 v0.2.1 基础上承载本地定制。
- 生产环境未修改、未重启、未发布。

## 发布前必须完成

- [x] 推送 `custom` 最新提交到 GitHub。
- [ ] 完成发布前人工审阅 `main..custom` 差异，重点确认不包含密钥、令牌、生产环境文件、数据库备份或临时运维文件。
- [ ] 本地 Go 测试与构建。当前环境未安装 Go。
- [ ] 前端类型检查与定向测试。
- [ ] 非生产环境构建镜像并做回归验证。
- [ ] 形成发布说明与回滚方案。
- [ ] 用户明确确认后，才进入生产发布。

## 当前已验证

- 公开 Fork：`https://github.com/stingysheep/sub2api`。
- GitHub `main`：`ab99d56e9626e6cd731592dae8553c9758a0efa2`。
- GitHub `custom`：`ba00ef5fdc323a6d618691979b16d0d4c1ab88bc`。
- `pnpm --dir frontend exec vue-tsc --noEmit`：通过。
- 账号/分组/公共界面相关定向 Vitest：通过。
- `CreateAccountModal.spec.ts`：31 个测试通过。
- 工作区未跟踪的运维备份、截图、tar 和临时目录没有纳入提交。
