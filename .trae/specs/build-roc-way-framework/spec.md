# roc_way 框架构建 Spec

> **模块路径**：`github.com/cuiyuanxin/roc_way`（2026-06-22 起生效，之前为 `github.com/roc_way`）。所有 import 必须沿用新路径。

## Why

`roc_way` 当前只有空目录骨架和 `go.mod`，需要落地为一个**基于 Go + Gin 的轻量级 Web 开发框架**，提供配置、数据库、缓存、认证、日志、中间件、文件存储、实时通信、定时任务、验证器、错误处理、CLI 脚手架等开箱即用的基础设施，让用户可以基于此框架 5 分钟搭建一个生产级后端服务。

## What Changes

- **NEW** 初始化框架根目录结构（遵循 `project_rules.md`）
- **NEW** `internal/app/admin` 参考示例应用（演示框架用法）
- **NEW** `cmd/rocway` 框架二进制入口 + `cmd/rocway-cli` 脚手架 CLI
- **NEW** 配置管理子系统（**基于 viper**：YAML + env override + 热更新）
- **NEW** 数据库子系统（MySQL + GORM，自动迁移、重试、连接池、读写分离）
- **NEW** 缓存子系统（Redis，键前缀、TTL、分布式、SCAN 优化）
- **NEW** 认证子系统（JWT，黑名单、刷新）
- **NEW** 日志子系统（API/DB 分级 + 轮转）
- **NEW** 中间件链（CORS、限流、访问日志、JWT、CSRF）
- **NEW** 文件存储（本地 + 阿里云 OSS）
- **NEW** 实时通信（SSE + WebSocket）
- **NEW** 定时任务调度器
- **NEW** 验证器（自定义错误消息）
- **NEW** 统一错误码体系
- **NEW** CLI 脚手架（**简单子命令优先使用标准库 `flag`，复杂子命令才使用 Cobra**）
- **NEW** `Makefile`、`Dockerfile`（`build/package/`）、`docker-compose.yml`（`deployments/`）
- **NEW** Swagger/OpenAPI 3.0 文档（swag 自动生成 + Swagger UI）

## 设计原则（强制约束）

> 用户硬性要求，覆盖任何其他默认选择。

1. **配置解析统一使用 [viper](https://github.com/spf13/viper)**
   - 不再使用 `gopkg.in/yaml.v3 + fsnotify` 自实现热更新；改用 viper 内置 `WatchConfig`。
   - 环境变量绑定使用 viper 的 `AutomaticEnv` + `SetEnvPrefix("ROCWAY")`。
   - 缺失字段使用 viper 的 `SetDefault` 提供默认值。

2. **CLI 分级实现**
   - 简单子命令（参数 ≤2 个、无嵌套、无复杂帮助）→ **使用 Go 标准库 `flag`**
   - 复杂子命令（多级嵌套、自动补全、帮助文档、子命令分组）→ **使用 [cobra](https://github.com/spf13/cobra)**
   - `rocway-cli` 顶层命令使用 cobra 维护命令树；`new` / `gen` 子命令内部仅需参数，简单实现走 `flag` 即可。

3. **优先复用成熟的企业级开源包，禁止造轮子**
   - 所有功能必须**优先选择 GitHub 高 star、高使用率、有生产实践的开源包**。
   - 仅当**没有合适的开源包**或开源包**无法满足需求**时，才允许**二次封装**或**重新实现**。
   - 每个内部子系统在引入自研代码前，必须在 `tasks.md` 标注「为什么不能用现成包」的简短理由（如有）。

4. **日志实现 = Zap + Lumberjack**
   - 日志结构化输出使用 [`go.uber.org/zap`](https://github.com/uber-go/zap)。
   - 文件轮转使用 [`gopkg.in/natefinch/lumberjack.v2`](https://github.com/natefinch/lumberjack)。
   - 二者通过 `zapcore.AddSync(lumberjackLogger)` 桥接，**禁止**自实现日志库或文件切割。

5. **权限 RBAC 使用 [Casbin](https://github.com/casbin/casbin)**
   - 使用 `casbin` v2 作为权限策略引擎（model = RBAC with domains）。
   - 默认策略文件：`configs/rbac_model.conf` + `configs/rbac_policy.csv`。
   - 提供 `auth.Enforcer()` 接口加载 model/policy，热更新 policy。
   - 框架内置 `RequirePermission(p string)` 中间件，调用 `enforcer.Enforce(sub, obj, act)`。

6. **DDD 注入模式 + Wire**
   - **核心原则**：只有**外部可变依赖**（DB、Redis、MQ、OSS、第三方 API 等）才需要接口抽象与注入；纯内部值对象、工具函数、业务领域模型**不需要**抽象，避免过度设计。
   - **依赖注入实现**：使用 [`github.com/google/wire`](https://github.com/google/wire) 编译期生成注入代码。
   - Provider 函数集中在 `internal/wire/`，生成的 `wire_gen.go` 由 `wire ./...` 生成。
   - 应用层（`internal/app/admin`）通过 `wire.Build(...)` 把 config → logger → db → cache → storage → enforcer → services → handlers 一条龙串起来。

## Impact

- 受影响规范：`project_rules.md`（已存在，本 spec 严格遵循）
- 受影响代码：项目从 0 个 `.go` 文件起步
- 新增入口：
  - `cmd/rocway/main.go` — 框架运行时（启动 admin 应用）
  - `cmd/rocway-cli/main.go` — 脚手架工具
- 关键代码组织：
  - 公共能力 → `pkg/<name>/`
  - 应用独有 → `internal/app/<name>/`
  - 协议定义 → `api/`
  - 配置模板 → `configs/`
  - 部署编排 → `deployments/`
  - 脚手架模板 → `assets/scaffold/`
  - Git 钩子 → `githooks/`

## ADDED Requirements

### Requirement: 框架入口可启动

框架 SHALL 提供 `cmd/rocway` 二进制，启动后能拉起 admin 应用并监听 HTTP 端口。

#### Scenario: 成功启动
- **WHEN** 执行 `go run ./cmd/rocway`
- **THEN** admin 应用监听 `:8080`，访问 `GET /healthz` 返回 `200 OK {"status":"ok"}`

### Requirement: 配置管理（基于 viper）

框架 SHALL 使用 [viper](https://github.com/spf13/viper) 作为配置加载入口，提供 YAML 加载、环境变量覆盖、热更新与默认值。

#### Scenario: 默认加载
- **WHEN** 存在 `configs/config.yaml`
- **THEN** 应用启动时通过 `viper.ReadInConfig()` 加载该文件，缺失字段使用 `viper.SetDefault` 设定的默认值

#### Scenario: 环境变量覆盖
- **WHEN** 设置 `ROCWAY_DB_HOST=10.0.0.1`（已 `viper.SetEnvPrefix("ROCWAY")`）
- **THEN** `viper.GetString("database.host")` 返回 `10.0.0.1`

#### Scenario: 热更新
- **WHEN** 调用 `viper.WatchConfig()` 并运行时修改 `configs/config.yaml` 中某个标量字段
- **THEN** viper `OnConfigChange` 回调 5 秒内被触发，框架重新读取该字段并写 reload 日志

### Requirement: 数据库访问

框架 SHALL 基于 GORM 提供 MySQL 访问能力，包含自动迁移、连接池、重试、读写分离。

#### Scenario: 自动迁移
- **WHEN** 定义 struct `User{ID uint; Name string}` 并注册到 auto-migrate 列表
- **THEN** 启动时自动 `CREATE TABLE IF NOT EXISTS users`

#### Scenario: 连接失败重试
- **WHEN** 数据库初次连接失败
- **THEN** 框架按指数退避重试最多 3 次，最终失败时返回明确错误

#### Scenario: 读写分离
- **WHEN** 配置 `database.read` 节点列表
- **THEN** `Read`/`Find` 自动走从库，`Create`/`Update`/`Delete` 走主库

### Requirement: 缓存

框架 SHALL 基于 Redis 提供分布式缓存能力，支持键前缀、TTL、SCAN 迭代。

#### Scenario: 键前缀
- **WHEN** 配置 `cache.prefix=rocway:`
- **THEN** `cache.Set(ctx,"user:1",v)` 实际键为 `rocway:user:1`

#### Scenario: SCAN 优化
- **WHEN** 调用 `cache.Scan(ctx,"rocway:user:*",fn)`
- **THEN** 使用 Redis `SCAN` 而非 `KEYS`，每次扫描 500 条

### Requirement: 认证

框架 SHALL 提供 JWT 认证，支持 Token 黑名单与刷新。

#### Scenario: 签发 Token
- **WHEN** 调用 `auth.Issue(userID)`
- **THEN** 返回带 `exp` 的 JWT，payload 含 `sub=userID`

#### Scenario: 黑名单
- **WHEN** 用户登出并调用 `auth.Revoke(jti)`
- **THEN** 该 jti 在剩余有效期内被拒绝通过中间件

#### Scenario: 刷新
- **WHEN** 客户端携带 refresh token 请求 `/auth/refresh`
- **THEN** 返回新的 access token，旧 access token 不受影响

### Requirement: 日志

框架 SHALL 提供 API/DB 双通道分级日志，支持按大小轮转。

#### Scenario: API 日志
- **WHEN** 任意 HTTP 请求完成
- **THEN** 输出 `method/path/status/latency/client_ip` 一行 JSON

#### Scenario: 轮转
- **WHEN** `logs/api.log` 达到 100MB
- **THEN** 自动归档为 `logs/api.log.2026-06-22-001` 并新建空文件

### Requirement: 权限 RBAC（基于 Casbin）

框架 SHALL 使用 [Casbin](https://github.com/casbin/casbin) 提供 RBAC 权限校验，并提供中间件。

#### Scenario: 策略文件加载
- **WHEN** 启动时存在 `configs/rbac_model.conf` 与 `configs/rbac_policy.csv`
- **THEN** `auth.NewEnforcer(modelPath, policyPath)` 加载并可通过 `enforcer.Enforce("alice","/api/v1/user","GET")` 校验

#### Scenario: 权限中间件
- **WHEN** handler 注册 `router.GET("/api/v1/user", RequirePermission("user:read"), ctrl.List)`
- **THEN** 鉴权失败返回 `403 Forbidden {code:2003, message:"无权限"}`

#### Scenario: 热更新策略
- **WHEN** 运行时修改 `rbac_policy.csv`
- **THEN** 框架自动重新加载策略，无需重启

### Requirement: 中间件

框架 SHALL 内置 CORS、限流、访问日志、JWT 认证、CSRF 五个中间件。

#### Scenario: 限流
- **WHEN** 同一 IP 1 秒内请求 `/login` 超过 10 次
- **THEN** 第 11 次返回 `429 Too Many Requests`

#### Scenario: CSRF
- **WHEN** 浏览器 POST 请求缺少 `X-CSRF-Token` 头
- **THEN** 返回 `403 Forbidden`

### Requirement: 文件存储

框架 SHALL 提供本地存储与阿里云 OSS 两种 driver，统一接口 `Storage`。

#### Scenario: 上传
- **WHEN** 调用 `storage.Put(ctx,"avatars/1.png",reader)`
- **THEN** 文件被持久化，并返回可访问 URL

#### Scenario: driver 切换
- **WHEN** 配置 `storage.driver=oss` 并提供 access key
- **THEN** 后续 `Put/Get/Delete` 操作走 OSS

### Requirement: 实时通信

框架 SHALL 同时支持 SSE 与 WebSocket。

#### Scenario: SSE
- **WHEN** 客户端 `GET /sse/notifications` 携带 `Accept: text/event-stream`
- **THEN** 服务端保持长连接并周期性推送 `data: {...}\n\n`

#### Scenario: WebSocket
- **WHEN** 客户端通过 `ws://host/ws/chat` 升级
- **THEN** 服务端维护连接并支持双向消息

### Requirement: 定时任务

框架 SHALL 提供进程内任务调度器。

#### Scenario: 注册 Cron
- **WHEN** 调用 `scheduler.Cron("0 */5 * * * *", job)`
- **THEN** 任务每 5 分钟执行一次，错误日志被记录且不影响下一次触发

### Requirement: 验证器

框架 SHALL 基于 `go-playground/validator` 提供请求参数验证。

#### Scenario: 字段错误
- **WHEN** 请求体 `{"email":"not-an-email"}`
- **THEN** 返回 `400` 与错误码 `VALIDATION_ERROR`，消息为 `"邮箱格式不正确"`

### Requirement: 错误码体系

框架 SHALL 定义统一错误码 `ErrCode`。

#### Scenario: 业务错误
- **WHEN** handler 返回 `errors.New(ErrCodeUserNotFound)`
- **THEN** 响应 `{"code":1001,"message":"用户不存在","request_id":"..."}`

### Requirement: CLI 脚手架

框架 SHALL 提供 `rocway-cli` 工具。

#### Scenario: 新建项目
- **WHEN** 执行 `rocway-cli new myapp`
- **THEN** 在当前目录生成 `myapp/` 目录，含完整可运行的骨架

#### Scenario: 生成代码
- **WHEN** 执行 `rocway-cli gen controller user`
- **THEN** 在 `internal/app/admin/controller/` 生成 `user.go` 含基础 CRUD 模板

### Requirement: 依赖注入（DDD 风格 + Wire）

框架 SHALL 使用 [Wire](https://github.com/google/wire) 把外部可变依赖装配进应用，**禁止过度抽象**。

#### Scenario: 注入范围
- **WHEN** Provider 注册外部依赖（DB、Cache、Storage、MQ、Enforcer、Logger）
- **THEN** 编译期 `wire ./...` 通过，运行时零反射
- 内部值对象（如 `User`、`Money`）、工具函数（如 `time.Now`、`strings.ToLower`）**不**参与注入

#### Scenario: 一条龙装配
- **WHEN** `cmd/rocway/main.go` 调用 `wire.Build(ProviderSet)`
- **THEN** 生成的 `wire_gen.go` 提供 `InitApp(cfg) (*App, func(), error)`，自动注入所有依赖

### Requirement: 部署产物

框架 SHALL 提供 `Dockerfile`、`docker-compose.yml`、`Makefile`。

#### Scenario: 一键启动
- **WHEN** 执行 `docker compose up`
- **THEN** rocway + MySQL + Redis 三个容器全部 healthy

### Requirement: Swagger/OpenAPI 3.0 文档

框架 SHALL 提供自动化 API 文档生成与 Swagger UI，文档与代码保持同步。

#### Scenario: 自动生成文档
- **WHEN** 执行 `make swagger`
- **THEN** 在 `api/docs/swagger.json` 生成完整的 OpenAPI 3.0 规范
- **AND** 包含所有已注册路由的 path、method、parameters、responses

#### Scenario: Swagger UI 访问
- **WHEN** 浏览器访问 `GET /swagger/index.html`
- **THEN** 返回 Swagger UI 页面，左上角显示 "roc_way API"

#### Scenario: API 调试
- **WHEN** 在 Swagger UI 中点击接口 "Try it out" → "Execute"
- **THEN** 发起真实请求并在响应体中显示结果

#### Scenario: 路由变更自动更新
- **WHEN** 修改 controller 后重新 `make swagger`
- **THEN** `api/docs/swagger.json` 同步更新，Swagger UI 反映最新接口

### Requirement: Git Hooks 本地质量门禁

框架 SHALL 提供 Git 本地钩子（`githooks/` 目录），确保提交代码前自动执行质量检查。

#### Scenario: pre-commit 钩子
- **WHEN** 开发者执行 `git commit`
- **THEN** pre-commit 钩子自动执行：`go fmt ./...`、`go vet ./...`、`go test ./...`
- **AND** 任一检查失败则 commit 被拒绝

#### Scenario: commit-msg 钩子
- **WHEN** 开发者编写 commit message
- **THEN** commit-msg 钩子校验 message 格式（支持 Conventional Commits：`feat:` `fix:` `docs:` `chore:` `refactor:` `test:` `perf:`）
- **AND** 格式不符则 commit 被拒绝

#### Scenario: 钩子安装
- **WHEN** 新开发者克隆仓库后首次 setup
- **THEN** 执行 `make install-hooks` 或 `ln -sf ../../githooks/pre-commit .git/hooks/` 将钩子链接到 `.git/hooks/`

## MODIFIED Requirements

无（首次构建）。

## REMOVED Requirements

无（首次构建）。

## 技术栈与目录映射

> **设计原则**：优先选择 GitHub 高 star、有生产实践的开源包；表格中的每一项均已满足该原则。`internal/pkg/*` 为框架内部库（不允许外部 import），需要被外部使用的能力提升到 `pkg/`。

| 子系统 | 选型（成熟开源包） | 主要目录 | 选用理由 / 不造轮子的依据 |
| --- | --- | --- | --- |
| HTTP | [`github.com/gin-gonic/gin`](https://github.com/gin-gonic/gin) | `internal/app/admin`, `internal/pkg/middleware` | Go 生态最主流的 Web 框架 |
| 配置 | [`github.com/spf13/viper`](https://github.com/spf13/viper) | `internal/pkg/config` | 事实标准；YAML/env/热更新/默认值开箱即用 |
| 数据库 | [`gorm.io/gorm`](https://github.com/go-gorm/gorm) + `gorm.io/driver/mysql` | `internal/pkg/database` | Go ORM 业界首选 |
| 缓存 | [`github.com/redis/go-redis/v9`](https://github.com/redis/go-redis) | `internal/pkg/cache` | Redis 官方推荐 Go 客户端 |
| 认证 | [`github.com/golang-jwt/jwt/v5`](https://github.com/golang-jwt/jwt) | `internal/pkg/auth` | JWT 主流实现 |
| 日志 | [`go.uber.org/zap`](https://github.com/uber-go/zap) + [`gopkg.in/natefinch/lumberjack.v2`](https://github.com/natefinch/lumberjack) | `internal/pkg/logger` | 高性能结构化日志 + 文件轮转 |
| 文件存储 | [`github.com/aliyun/aliyun-oss-go-sdk/oss`](https://github.com/aliyun/aliyun-oss-go-sdk) | `internal/pkg/storage` | 阿里云官方 OSS SDK |
| 实时通信 | [`github.com/gorilla/websocket`](https://github.com/gorilla/websocket) | `internal/pkg/realtime` | WebSocket 事实标准 |
| 定时任务 | [`github.com/robfig/cron/v3`](https://github.com/robfig/cron) | `internal/pkg/scheduler` | Go 生态主流 Cron 库 |
| 验证器 | [`github.com/go-playground/validator/v10`](https://github.com/go-playground/validator) + [`github.com/go-playground/validator/v10/translations/zh`](https://github.com/go-playground/validator) | `internal/pkg/validator` | gin 默认绑定器，翻译器支持中文错误 |
| 权限 RBAC | [`github.com/casbin/casbin/v2`](https://github.com/casbin/casbin) + [`github.com/casbin/gorm-adapter/v3`](https://github.com/casbin/gorm-adapter)（可选） | `internal/pkg/auth`（enforcer） | 事实标准策略引擎，model+policy 解耦 |
| 依赖注入 | [`github.com/google/wire`](https://github.com/google/wire) | `internal/wire/` | 编译期生成，零反射 |
| 限流 | [`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate) | `internal/pkg/middleware` | Go 官方令牌桶 |
| CLI（顶层 / 复杂） | [`github.com/spf13/cobra`](https://github.com/spf13/cobra) | `cmd/rocway-cli` | 复杂命令树首选 |
| CLI（简单） | **标准库 `flag`** | `cmd/rocway-cli/cmd/new.go` 等 | Go 内置，避免引入依赖 |
| 脚手架模板 | — | `assets/scaffold/` | 文本模板，使用 `text/template` |
| API 文档 | [`github.com/swaggo/swag`](https://github.com/swaggo/swag) + [`github.com/swaggo/gin-swagger`](https://github.com/swaggo/gin-swagger) | `api/docs/`, `internal/app/admin/controller/` | Go AST 解析注释生成 OpenAPI 3.0，gin 集成事实标准 |
| Swagger UI | [`github.com/swaggo/files`](https://github.com/swaggo/files) | `web/static/swagger/` | swaggo 官方 UI 静态资源 |

> 备注：依据 `project_rules.md`，以上 `internal/pkg/*` 为**框架内部库**（不允许外部 import）。当某模块需要被外部项目使用（例如 CLI 单独分发）时，相应子包提升到 `pkg/`。

## 模块依赖关系

```
cmd/rocway (main)
    └── internal/app/admin          (应用)
            ├── internal/pkg/middleware
            ├── internal/pkg/auth
            ├── internal/pkg/cache
            ├── internal/pkg/database
            ├── internal/pkg/logger
            └── internal/pkg/config
                              ↑
cmd/rocway-cli (脚手架) ──────┘  (只读 assets/scaffold/)
```
