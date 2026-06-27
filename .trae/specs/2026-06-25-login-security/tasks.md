# 登录安全加固 实施任务清单

> 关联 spec：[`../2026-06-25-login-security-design.md`](../2026-06-25-login-security-design.md)
> 关联 checklist：[`checklist.md`](./checklist.md)
> 关联 project_rules：[`.trae/rules/project_rules.md` 第 19 条](../../../rules/project_rules.md)

## Phase 1：错误码 + logger.Security() + notify 包

| Task | 状态 | 备注 |
|---|---|---|
| 1.1 `errcode` 加 `ErrAccountLocked` / `ErrNotImplemented` | ✅ DONE | [errcode.go](../../../internal/pkg/errcode/errcode.go) |
| 1.2 `logger.Loggers` 加 `Security()` channel | ✅ DONE | [logger.go](../../../internal/pkg/logger/logger.go) |
| 1.3 `New()` 构造 securityCore + security logger | ✅ DONE | 同上 |
| 1.4 新建 `internal/pkg/notify/notify.go`（接口 + Event） | ✅ DONE | [notify.go](../../../internal/pkg/notify/notify.go) |
| 1.5 新建 `internal/pkg/notify/noop.go`（默认实现） | ✅ DONE | [noop.go](../../../internal/pkg/notify/noop.go) |
| 1.6 单测 `Notify` 不 panic | ✅ DONE | [noop_test.go](../../../internal/pkg/notify/noop_test.go) |

## Phase 2：限流中间件改造

| Task | 状态 | 备注 |
|---|---|---|
| 2.1 `RateLimitOptions` 加 `Window` / `Limit` 字段 | ✅ DONE | [middleware.go](../../../internal/pkg/middleware/middleware.go) |
| 2.2 新增 `fixedWindowRedisLimiter`（Redis INCR+EXPIRE） | ✅ DONE | 同上 |
| 2.3 新增 `fixedWindowMemoryLimiter`（内存版兜底） | ✅ DONE | 同上 |
| 2.4 `NewRateLimiter` 模式分发（固定窗口 vs 令牌桶） | ✅ DONE | 同上 |
| 2.5 `config.RouteLimitConfig` 配置类型 + 默认值 | ✅ DONE | [config.go](../../../internal/pkg/config/config.go) |
| 2.6 `config.LoginPolicyConfig` 锁定策略配置 | ✅ DONE | 同上 |

## Phase 3：model.User + domain + repo.FindByUsername

| Task | 状态 | 备注 |
|---|---|---|
| 3.1 `model.User` 加 `Username` 字段 + uniqueIndex，去 email uniqueIndex | ✅ DONE | [user.go](../../../internal/app/admin/model/user.go) |
| 3.2 `domain.NewUser` 加 username 参数 + 校验正则 | ✅ DONE | [user.go](../../../internal/app/admin/domain/user.go) |
| 3.3 `domain.User.Validate()` 加 username 校验 | ✅ DONE | 同上 |
| 3.4 清理 `isValidEmail` 死代码 | ✅ DONE | 同上 |
| 3.5 `domain.ErrEmailTaken` → `domain.ErrUsernameTaken` | ✅ DONE | 同上 |
| 3.6 `repository.UserRepository` 加 `FindByUsername` 接口 | ✅ DONE | [user_iface.go](../../../internal/app/admin/repository/user_iface.go) |
| 3.7 `userRepo.FindByUsername` GORM 实现 | ✅ DONE | [user_gorm.go](../../../internal/app/admin/repository/user_gorm.go) |
| 3.8 `toModel` / `toDomain` 加 Username 字段 | ✅ DONE | 同上 |
| 3.9 `dto.RegisterInput` 加 Username + `LoginInput` Email→Username + IP | ✅ DONE | [user.go](../../../internal/app/admin/dto/user.go) |
| 3.10 `service.UserService.Register` 用 username 查重 | ✅ DONE | [user.go](../../../internal/app/admin/service/user.go) |

## Phase 4：login_audits 表 + LoginAuditRepository

| Task | 状态 | 备注 |
|---|---|---|
| 4.1 `model/login_audit.go` 单表 GORM 映射 + EventType 常量 | ✅ DONE | [login_audit.go](../../../internal/app/admin/model/login_audit.go) |
| 4.2 `domain/lock.go` `LockLevel` + `AccountLock` 聚合 | ✅ DONE | [lock.go](../../../internal/app/admin/domain/lock.go) |
| 4.3 `repository/audit_iface.go` LoginAuditRepository 接口 | ✅ DONE | [audit_iface.go](../../../internal/app/admin/repository/audit_iface.go) |
| 4.4 `repository/audit_gorm.go` GORM 实现（RecordFailure / RecordLock / LatestActiveLock / RecentFailuresCount / ClearFailures / CleanupExpired） | ✅ DONE | [audit_gorm.go](../../../internal/app/admin/repository/audit_gorm.go) |
| 4.5 写入路径在线清理（LIMIT 1000 failure） | ✅ DONE | 同上 |

## Phase 5：LockService（双写逻辑封装）

| Task | 状态 | 备注 |
|---|---|---|
| 5.1 `cache.Client.IncrWithTTL`（Redis INCR+EXPIRE pipeline） | ✅ DONE | [cache.go](../../../internal/pkg/cache/cache.go) |
| 5.2 `cache.Client.SetNX`（分布式锁场景） | ✅ DONE | 同上 |
| 5.3 `service/lock.go` `LockService.GetLock`（Redis 主读 + DB 兜底） | ✅ DONE | [lock.go](../../../internal/app/admin/service/lock.go) |
| 5.4 `LockService.RecordFailure`（累加 + 判断 + 写锁 + Notify） | ✅ DONE | 同上 |
| 5.5 `LockService.ClearFailures`（成功登录重置计数） | ✅ DONE | 同上 |
| 5.6 错误全部 zap warn 容错，业务不阻断 | ✅ DONE | 同上 |

## Phase 6：service/auth.go 改造

| Task | 状态 | 备注 |
|---|---|---|
| 6.1 `AuthService` 注入 `LockService` + `repo`（直接 FindByUsername） | ✅ DONE | [auth.go](../../../internal/app/admin/service/auth.go) |
| 6.2 `Login` 重写：username 查 → 锁定查 → 密码校验 → 失败计数 + 可能锁 | ✅ DONE | 同上 |
| 6.3 用户不存在也计数（防枚举） | ✅ DONE | 同上 |
| 6.4 触发锁定时优先返回 ErrAccountLocked | ✅ DONE | 同上 |
| 6.5 成功登录 ClearFailures + Issue token | ✅ DONE | 同上 |

## Phase 7：handler 改造

| Task | 状态 | 备注 |
|---|---|---|
| 7.1 `handler.Health` 加 limitMw 参数 + Register 挂载 | ✅ DONE | [health.go](../../../internal/app/admin/handler/health.go) |
| 7.2 `handler.Auth` 加 limitMw 参数 + Register 只对 login 挂载 | ✅ DONE | [auth.go](../../../internal/app/admin/handler/auth.go) |
| 7.3 `handler/auth.go loginByMobile` 路由 + dto + stub 返回 501 | ✅ DONE | 同上 |
| 7.4 `handler/auth.go login` 注入 IP（c.ClientIP） | ✅ DONE | 同上 |

## Phase 8：janitor 包 + app.go 装配

| Task | 状态 | 备注 |
|---|---|---|
| 8.1 `internal/pkg/janitor/janitor.go` 通用 Janitor + Runners | ✅ DONE | [janitor.go](../../../internal/pkg/janitor/janitor.go) |
| 8.2 `internal/pkg/janitor/login_audit.go` 登录审计清理任务 | ✅ DONE | [login_audit.go](../../../internal/pkg/janitor/login_audit.go) |
| 8.3 janitor 单测（Start/Stop/ErrorCallback/ZeroInterval） | ✅ DONE | [janitor_test.go](../../../internal/pkg/janitor/janitor_test.go) |
| 8.4 `app.go` 装配 LockService / Notifier / janitor | ✅ DONE | [app.go](../../../internal/app/admin/app.go) |
| 8.5 `app.go buildRouteLimitMw` 按 path+method 查找 RouteLimitConfig 构造中间件 | ✅ DONE | 同上 |
| 8.6 `App.Close()` 调 `runners.Stop()` 清理 goroutine | ✅ DONE | 同上 |

## Phase 9：自动化验证

| Task | 状态 | 备注 |
|---|---|---|
| 9.1 `go build ./...` 编译通过 | ✅ DONE | exit 0 |
| 9.2 `go vet ./...` 通过 | ✅ DONE | exit 0 |
| 9.3 `go test ./...` 全部通过（8 个包） | ✅ DONE | admin/{domain,service}, pkg/{auth, config, errcode, janitor, middleware, notify} |

## 总体统计

- **新增文件**：6 个（含 3 个测试文件）
- **修改文件**：14 个
- **删除文件**：0
- **依赖变更**：无新外部依赖
- **编译 / vet / 测试**：全过

## 待人工处理（部署前）

- [ ] MySQL 迁移 SQL（spec §4.4）
- [ ] `configs/config.yaml` 加 `route_limits` 配置
- [ ] `configs/config.yaml` 加 `login_policy` 配置
- [ ] `logs/security.log` 加入日志监控告警