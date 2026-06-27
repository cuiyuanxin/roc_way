# 登录安全加固 验收清单

> 关联 spec：[`../2026-06-25-login-security-design.md`](../2026-06-25-login-security-design.md)
> 关联 tasks：[`tasks.md`](./tasks.md)

## 1. 路由级限流

- [ ] 全局限流保留 RPS/Burst 行为不变（回归通过 `go test ./internal/pkg/middleware/...`）
- [ ] `/healthz` 路由级 20次/分钟/IP 触发 429
- [ ] `/api/auth/login` 路由级 20次/分钟/IP 触发 429
- [ ] 触发响应符合规则 18（`response.WriteErr` + `request_id`）
- [ ] 配置层 `route_limits` 可声明任意路由的 Window/Limit

## 2. 失败锁定

- [ ] 5 次连败后第 6 次返回 423（`ErrAccountLocked`）
- [ ] 10 次连败后触发长期锁定，24 小时有效
- [ ] 锁定到期后允许登录（自动解锁）
- [ ] 成功登录重置失败计数（Redis Del + DB ClearFailures）
- [ ] lock 记录不被成功登录清除（防攻击者试探）

## 3. Redis + MySQL 双存储

- [ ] Redis 故障时登录可用（DB 兜底查锁定）
- [ ] Redis 故障时锁定可写入（DB 兜底写 lock）
- [ ] DB 也失败时业务不阻断（zap error 告警）
- [ ] Redis INCR + EXPIRE 原子（pipeline 一次往返）

## 4. 多登录方式预留

- [ ] `POST /api/auth/login` 真正实现 username + password 登录
- [ ] `POST /api/auth/login/mobile` 返回 501 + `ErrNotImplemented`
- [ ] mobile 接口 dto 占位（mobile + password）

## 5. notify 通知

- [ ] `Notifier.Notify` 不返回 error
- [ ] `Notifier.Notify` 不允许 panic（单测覆盖）
- [ ] `NoopNotifier` 输出 zap 安全日志到 `logs/security.log`
- [ ] 锁定触发时同步推送事件给安全管理员

## 6. 数据清理

- [ ] janitor 启动后台 goroutine（24h 间隔）
- [ ] janitor 错误经 `onError` callback 落 zap error 日志（**不 panic**）
- [ ] `App.Close()` 调 `runners.Stop()` 无 goroutine 泄漏
- [ ] 写入路径在线清理 LIMIT 1000

## 7. username 字段

- [ ] `model.User` 加 `Username` 字段 + uniqueIndex
- [ ] `email` 字段去 uniqueIndex（保留兼容索引）
- [ ] `domain.NewUser` 加 username 参数 + 校验
- [ ] `repository.UserRepository.FindByUsername` 实现
- [ ] `dto.LoginInput.Username + IP`
- [ ] **生产环境**手动迁移 SQL（详见 spec §4.4）
- [ ] **禁止**依赖 GORM AutoMigrate 加 uniqueIndex

## 8. 错误码

- [ ] `ErrAccountLocked = Code{2005, "账号已锁定，请稍后再试", 423}`
- [ ] `ErrNotImplemented = Code{2006, "功能未实现", 501}`
- [ ] 所有错误响应 body 含 `request_id`（应用规则 18）

## 9. 自动化验证

- [x] `go build ./...` 编译通过
- [x] `go vet ./...` 通过
- [x] `go test ./...` 全部通过（8 个包 OK）
  - [x] `internal/pkg/middleware`
  - [x] `internal/pkg/notify`（含不 panic 单测）
  - [x] `internal/pkg/janitor`（含 Start/Stop/ErrorCallback 单测）
  - [x] `internal/pkg/config`
  - [x] `internal/pkg/errcode`
  - [x] `internal/pkg/auth`
  - [x] `internal/app/admin/domain`
  - [x] `internal/app/admin/service`

## 10. 部署前手动动作（运维）

- [ ] **MySQL 迁移**（不可逆）：
  ```sql
  ALTER TABLE users ADD COLUMN username VARCHAR(64) NOT NULL DEFAULT '';
  UPDATE users SET username = CONCAT('user_', id) WHERE username = '';
  ALTER TABLE users ADD UNIQUE INDEX idx_users_username (username);
  ```
- [ ] `configs/config.yaml` 加 `route_limits` 配置（参见 spec §4.1）
- [ ] `configs/config.yaml` 加 `login_policy` 配置（阈值 / 时长 / janitor 间隔）
- [ ] `logs/security.log` 加入 logrotate 监控