# Checklist

> 每个验收点在实现完成后由验证 sub-agent 勾选。`[x]` 表示已通过。
>
> **模块路径**：`github.com/cuiyuanxin/roc_way`（2026-06-22 起；旧 `github.com/roc_way` 已废弃）。

## 工程结构

- [x] 占位目录 `_your_app_/`、`_your_private_lib_/`、`_your_public_lib_/` 已删除
- [x] `go.mod` 中已添加 spec 列出的全部第三方依赖（**包含 `github.com/spf13/viper`**）
- [x] `go.mod` 中**不包含** `gopkg.in/yaml.v3` 与 `github.com/fsnotify/fsnotify`（已被 viper 替代）
- [x] `go build ./...` 通过
- [x] 所有新增 `.go` 文件位置符合 `project_rules.md`

## 配置管理（基于 viper）

- [x] `internal/pkg/config` 使用 `viper.New()` 封装 `Load(path)` / `Watch(onChange)`
- [x] `cmd/rocway/main.go` 调用 `cfgMgr.Watch()` 启用热更新
- [x] 修改 `configs/config.yaml` 后 5s 内 `viper.OnConfigChange` 回调被触发
- [x] `ROCWAY_DB_HOST=10.0.0.1` 通过 `viper.AutomaticEnv` 覆盖默认配置
- [x] 缺失字段由 `viper.SetDefault` 提供默认值
- [x] 单元测试覆盖 YAML 加载 / env 覆盖 / 热更新
- [x] `./rocway -c /custom/config.yaml` 使用指定配置文件
- [x] 命令行参数优先于默认路径 `configs/config.yaml`
- [x] 环境变量仍可覆盖命令行 YAML 配置（最高优先级）

## 日志（Zap + Lumberjack）

- [x] `internal/pkg/logger` 使用 `go.uber.org/zap` 作为核心 logger
- [x] 通过 `zapcore.AddSync(lumberjackLogger)` 接入 `natefinch/lumberjack` 实现文件轮转
- [x] `logs/api.log` 与 `logs/db.log` 分开输出
- [x] 单文件超过 100MB 自动归档，保留 7 份，启用压缩
- [x] API 请求日志含 `method/path/status/latency/client_ip`

## 错误码 & 验证器

- [x] `errcode` 预置 ≥10 个常用错误码
- [x] 验证失败返回 `code:1000` 并附带字段名与中文消息

## 数据库

- [x] GORM MySQL 启动可连接，启动失败按 1s/2s/4s 退避重试 3 次
- [x] 启用读从库后，`Find/Read` 走从库，`Create/Update/Delete` 走主库
- [x] `AutoMigrate(User{})` 启动后 `users` 表已存在

## 缓存

- [x] `cache.Set(ctx,"u:1",v)` 在 Redis 中实际键为 `rocway:u:1`
- [x] `cache.Scan` 使用 `SCAN` 而非 `KEYS`

## 认证

- [x] `auth.Issue(userID)` 返回含 `sub/jti/exp` 的 access + refresh token
- [x] `auth.Revoke(jti)` 后，对应 token 在中间件中被拒
- [x] `/auth/refresh` 返回新 access token

## RBAC（Casbin）

- [x] `go.mod` 引入 `github.com/casbin/casbin/v2`
- [x] `configs/rbac_model.conf` 包含 RBAC with domains 语法段
- [x] `configs/rbac_policy.csv` 含示例策略并能被 enforcer 加载
- [x] `internal/pkg/auth/enforcer.go` 提供 `NewEnforcer(model, policy)` 与 `RequirePermission(obj, act)` 中间件
- [x] 无权限访问返回 `403 {code:2003, message:"无权限"}`
- [x] 修改 `rbac_policy.csv` 后策略热更新无需重启

## 文件存储

- [x] `local` driver 上传文件后可通过 `/storage/<key>` 访问
- [x] 切到 `oss` driver 后 `Put` 调用阿里云 OSS SDK

## 实时通信

- [x] `GET /sse/notifications` 返回 `Content-Type: text/event-stream`
- [x] `WS /ws/chat` 升级成功，支持双向消息
- [x] 客户端断连 30s 后服务端清理

## 定时任务

- [x] 注册的 Cron 每 5 分钟触发一次
- [x] 任务 panic 被捕获并写入错误日志，下一次调度仍正常

## 中间件

- [x] CORS 从 `server.cors` 配置读取（origins/methods/headers/expose_headers/max_age/allow_credentials，无默认值，从请求头获取 Origin）
- [x] 限流：从 `server.rate_limit` 配置读取（enabled/driver/rps/burst/key_prefix），支持 memory 和 redis 后端，自动设置 X-RateLimit-* 响应头，触发返回 429
- [x] 缺 `X-CSRF-Token` 的 POST 返回 `403`
- [x] Panic 在 `Recovery` 后转 `errcode.ErrInternal` JSON 响应

## admin 应用

- [x] `go run ./cmd/rocway` 后 `curl /healthz` 返回 200 `{"status":"ok"}`
- [x] `curl /api/v1/users` 经过 JWT 中间件校验
- [x] SSE 与 WebSocket 示例可访问
- [x] HTTP Server `ReadHeaderTimeout` 从 `server.read_header_timeout` 读取（默认10秒）
- [x] HTTP Server `Timeout` 从 `server.timeout` 读取（0 表示不启用，超时返回 504）
- [x] HTTPS 可通过 `server.tls.enabled=true` 启用（ListenAndServeTLS）
- [x] validator 支持中英文翻译（基于 go-playground/universal-translator，根据 Accept-Language 自动切换）
- [x] validator 自定义 `fieldmatch` 正则验证（用法：fieldmatch=REGEX[:错误消息]）
- [x] validator 使用 New+Option 模式注入自定义规则（无 init 副作用）

## CLI 脚手架（分级实现）

- [x] 顶层命令树基于 **cobra**：`rocway-cli {new,gen,version,...}`
- [x] 简单子命令内部使用 **标准库 `flag`**：`new` / `gen` 的 `-m module` / `-o output` 等参数解析
- [x] `./bin/rocway-cli new myapp` 生成可运行的 `myapp/` 目录
- [x] `./bin/rocway-cli gen controller user` 生成 controller 模板

## 部署产物

- [x] `make build` 产出 `bin/rocway` 与 `bin/rocway-cli`
- [x] `make wire` 调用 wire 工具重新生成 `wire_gen.go`
- [x] `docker compose up` 后 rocway / mysql / redis 均 healthy
- [x] `.env.example` 含所有可配置项

## 文档

- [x] `docs/quickstart.md` 让新用户 5 分钟跑通 admin
- [x] `docs/architecture.md` 包含模块依赖图

## Swagger/OpenAPI 文档

- [x] `swag` 命令已安装（`go install github.com/swaggo/swag/cmd/swag@latest`）
- [x] `make swagger` 命令已添加到 Makefile
- [x] `make swagger` 执行后 `api/docs/swagger.json` 存在且非空
- [x] `go.mod` 包含 `github.com/swaggo/swag` `github.com/swaggo/gin-swagger` `github.com/swaggo/files`
- [x] `api/docs/` 目录存在（swag 输出目录）
- [x] `api/response/response.go` 定义 `Response<T>` 含 `code/message/data/request_id`
- [x] `api/response/response.go` 定义 `PaginatedResponse<T>` 含 `list/total/page/page_size`
- [x] `controller/health.go` handler 有 `@Summary` `@Tags` `@Router`
- [x] `controller/auth.go` 登录/注册/刷新/登出 handler 有完整注释
- [x] `controller/user.go` CRUD handler 有 `@Param` `@Success` `@Failure` `@Router`
- [x] `controller/sse.go` / `controller/ws.go` 有 `@Summary` `@Tags` `@Router`
- [x] `api/router.go` 注册 `/swagger/*` 路由
- [x] `ginSwagger.WrapHandler(swaggerFiles.Handler)` 已挂载
- [ ] `make run` 后 `GET /swagger/index.html` 返回 200
- [x] `api/docs/swagger.json` 包含 `/healthz`、`/api/v1/users`、认证接口、SSE/WebSocket 接口
- [x] `make wire && make swagger` 串行执行无报错

## GitHooks 本地质量门禁

- [x] `githooks/pre-commit` 脚本存在且可执行
- [x] `githooks/commit-msg` 脚本存在且可执行
- [x] `pre-commit` 执行 `gofmt -l .`、`go vet ./...`、`go test ./... -short`
- [x] `commit-msg` 支持校验 Conventional Commits 格式（feat/fix/docs/chore/refactor/test/perf 等）
- [x] `make install-hooks` 可将钩子链接到 `.git/hooks/`
- [x] `git commit` 时钩子自动触发，检查失败则 commit 被拒绝

## 质量门禁

- [x] `go vet ./...` 无 warning
- [x] `gofmt -l .` 无输出
- [x] `go test ./...` 全绿
- [x] `wire ./...` 生成代码无报错

## 依赖注入（DDD + Wire）

- [x] `internal/wire/wire.go` 列出所有 Provider；`wire_gen.go` 已生成并随源码提交
- [x] `cmd/rocway/main.go` 通过 `wire.InitializeApp(cfg)` 一行启动，无手写依赖装配
- [x] Provider **仅**覆盖外部可变依赖（DB / Cache / Storage / MQ / Enforcer / Logger / Config）
- [x] 领域模型（`User`、`Money` 等）、工具函数未出现 Provider / Interface 抽象（避免过度注入）
- [x] `make wire` 重新生成注入代码成功

## 设计原则（用户硬性约束）

- [x] 配置解析使用 **viper**（`internal/pkg/config` 引用 `github.com/spf13/viper`）
- [x] 日志使用 **Zap + Lumberjack**（`internal/pkg/logger` 引用 `go.uber.org/zap` + `natefinch/lumberjack`）
- [x] 权限使用 **Casbin**（`internal/pkg/auth/enforcer.go` 引用 `github.com/casbin/casbin/v2`）
- [x] 依赖注入使用 **Wire**（`internal/wire/` 引用 `github.com/google/wire`）
- [x] CLI 顶层使用 **cobra**，简单子命令使用标准库 **`flag`**
- [x] 所有子系统优先复用成熟开源包，无自实现的 YAML/env 监听、JWT 签名、WebSocket 升级、CORS、ORM 等
