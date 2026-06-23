# Tasks

> 任务按"从底层到上层"的顺序排列，前置任务不阻塞可以并行的标 `[P]`。
> 每个任务完成后立即勾选 `- [x]`。
>
> **模块路径**：`github.com/cuiyuanxin/roc_way`（2026-06-22 起；旧 `github.com/roc_way` 已废弃）。

## Phase 0 — 工程初始化

- [x] **Task 0.1**: 初始化 `go.mod` 依赖与目录占位
  - [x] SubTask 0.1.1: 删除占位目录里的 `_your_app_/` `_your_private_lib_/` `_your_public_lib_/`
  - [x] SubTask 0.1.2: 添加核心依赖：`gin`、`viper`、`gorm`、`mysql`、`go-redis/v9`、`golang-jwt/jwt/v5`、`zap`、`lumberjack`、`casbin/v2`、`wire`、`go-playground/validator/v10`、**`cobra`（仅复杂命令）+ 标准库 `flag`（简单命令）**、`gorilla/websocket`、`robfig/cron/v3`、`golang.org/x/time/rate`、`aliyun-oss-go-sdk`
  - [x] SubTask 0.1.3: **不**引入 `gopkg.in/yaml.v3`、`fsnotify`（已被 viper 内置替代）
  - [x] SubTask 0.1.4: 安装 wire 工具链：`go install github.com/google/wire/cmd/wire@latest`，写进 Makefile
  - [x] SubTask 0.1.5: 提交 `go mod tidy`，确保 `go build ./...` 通过（即使为空 main）

## Phase 1 — 核心子系统 (P1)

- [x] **Task 1.1 [P]**: 实现配置管理 `internal/pkg/config`（基于 viper）
  - [x] 1.1.1 定义 `Config` 结构体（YAML tag + mapstructure tag）
  - [x] 1.1.2 使用 `viper.New()` 封装 `Load(path)` / `Watch(onChange)` 接口，**直接调用 viper 内置 `viper.WatchConfig()` + `OnConfigChange`**
  - [x] 1.1.3 `viper.SetEnvPrefix("ROCWAY")` + `viper.AutomaticEnv()`，键以 `.` 走 `viper.BindEnv`
  - [x] 1.1.4 `viper.SetDefault` 设定全部默认值
  - [x] 1.1.5 提供单元测试覆盖 YAML 加载、env 覆盖、热更新三个场景
  - **不造轮子依据**：viper 是 spf13 系事实标准，已内含 YAML/env/热更新/默认值，**禁止**自实现 fsnotify 监听。

- [x] **Task 1.2 [P]**: 实现日志 `internal/pkg/logger`（**Zap + Lumberjack**）
  - [x] 1.2.1 使用 `zap.NewProduction()` / `zap.NewDevelopment()` 工厂，提供 `New(level, file) (*zap.Logger, error)`
  - [x] 1.2.2 接入 `lumberjack.Logger` 实现按 100MB 轮转、保留 7 份、压缩，通过 `zapcore.AddSync(lumberjack)` 桥接
  - [x] 1.2.3 暴露 `API()` / `DB()` 两个独立 SugaredLogger，分别输出到 `logs/api.log` 与 `logs/db.log`
  - **不造轮子依据**：用户硬性指定 **Zap + Lumberjack**；这是 Go 生态结构化日志与文件轮转的标准组合。

- [x] **Task 1.3 [P]**: 实现统一错误码 `internal/pkg/errcode`
  - [x] 1.3.1 定义 `Code int` + `Message string` + `HTTPStatus int`
  - [x] 1.3.2 预置常见码：`ErrInvalidParam(1000)`、`ErrUserNotFound(1001)`、`ErrUnauthorized(2001)`、`ErrForbidden(2003)`、`ErrInternal(5000)` 等
  - [x] 1.3.3 提供 `Error` 类型实现 `error` 接口 + `WithMessage(s)`
  - **不造轮子依据**：业务错误码属于应用层语义，没有现成包可替代（pkg/errors 仅处理包装，不定义语义码）。

- [x] **Task 1.4 [P]**: 实现验证器 `internal/pkg/validator`
  - [x] 1.4.1 封装 `go-playground/validator/v10`，注册中文翻译器 `validator/v10/translations/zh`
  - [x] 1.4.2 暴露 `Bind(c *gin.Context, dst any) error`
  - [x] 1.4.3 错误统一转换为 `errcode.ErrInvalidParam` 并附带字段名
  - **不造轮子依据**：`go-playground/validator` 是 gin 推荐的请求验证器，官方提供中文翻译器；自实现既无必要也无法覆盖完整 tag 语法。

## Phase 2 — 资源子系统 (P2)

- [x] **Task 2.1 [P]**: 实现数据库 `internal/pkg/database`
  - [x] 2.1.1 定义 `Config{Write DSN; Read []DSN}` 与连接池参数
  - [x] 2.1.2 实现 `Open(cfg)`，主库 + 从库分别创建 `*gorm.DB`
  - [x] 2.1.3 连接重试：指数退避 1s/2s/4s，最多 3 次
  - [x] 2.1.4 提供 `Resolver`：读写分离（`Read`/`Find` → 随机从库，写 → 主库，事务内强制走主库）
  - [x] 2.1.5 `AutoMigrate(models ...any)` 接口
  - [x] 2.1.6 单元测试：使用 `sqlmock` 模拟重试与读写分离
  - **不造轮子依据**：GORM 自带 `Read/Write` 分离与 `Session` API，连接池由 `sql.DB` 提供，**禁止**自实现 ORM。

- [x] **Task 2.2 [P]**: 实现缓存 `internal/pkg/cache`
  - [x] 2.2.1 封装 `go-redis/v9`，配置 `Addr/Password/DB`
  - [x] 2.2.2 提供 `Client` 结构体：`Set/Get/Del/Expire/Exists/Inc/Dec`
  - [x] 2.2.3 键前缀：所有操作内部拼上 `cfg.Prefix`
  - [x] 2.2.4 `Scan(ctx, pattern, fn)`：基于 `SCAN cursor MATCH ... COUNT 500`，自动分批
  - [x] 2.2.5 单元测试：使用 `alicebob/miniredis/v2` 验证前缀与 Scan
  - **不造轮子依据**：`go-redis/v9` 已是 Redis 官方推荐 Go 客户端；前缀与 SCAN 属于应用层组合逻辑。

- [x] **Task 2.3 [P]**: 实现文件存储 `internal/pkg/storage`
  - [x] 2.3.1 定义统一接口 `Storage { Put, Get, Delete, URL }`
  - [x] 2.3.2 实现 `LocalDriver`：基于 `os` + `http.ServeFile`，支持 public URL
  - [x] 2.3.3 实现 `OSSDriver`：**直接复用 `aliyun-oss-go-sdk`**，仅做 driver 适配
  - [x] 2.3.4 `New(cfg) (Storage, error)` 工厂根据 `cfg.Driver` 选择 driver
  - **不造轮子依据**：阿里云官方 OSS SDK 已封装 REST/HTTP/签名，OSS driver 只做接口适配。

## Phase 3 — 业务子系统 (P3)

- [x] **Task 3.1 [P]**: 实现认证 `internal/pkg/auth`
  - [x] 3.1.1 定义 `Claims { sub, jti, exp, iat }`，**底层使用 `golang-jwt/jwt/v5` 签发/验签**
  - [x] 3.1.2 `Issue(sub)` 签发 access token + refresh token
  - [x] 3.1.3 `Parse(token)` 验签
  - [x] 3.1.4 `Revoke(jti)` 将 jti 写入 Redis 黑名单，TTL = token 剩余有效期
  - [x] 3.1.5 `IsRevoked(jti)` 查询 Redis
  - **不造轮子依据**：`golang-jwt/jwt/v5` 是 Go JWT 事实标准；黑名单借用 Redis 已有的 SET + EXPIRE。

- [x] **Task 3.2 [P]**: 实现实时通信 `internal/pkg/realtime`
  - [x] 3.2.1 SSE：`SSE(c *gin.Context, ch <-chan any)` 工具函数（基于 gin 原生流式响应）
  - [x] 3.2.2 WebSocket：`Upgrade(c)` 工具函数 + `Hub` 维护连接集合（基于 `gorilla/websocket`）
  - [x] 3.2.3 心跳：30s ping/pong，断连自动清理
  - **不造轮子依据**：`gorilla/websocket` 是 Go WebSocket 事实标准；SSE 借助 net/http flush，框架只做封装。

- [x] **Task 3.3 [P]**: 实现定时任务 `internal/pkg/scheduler`
  - [x] 3.3.1 封装 `robfig/cron/v3`
  - [x] 3.3.2 暴露 `Cron(expr string, job func(ctx)) error`
  - [x] 3.3.3 `Start()` / `Stop(ctx)` 优雅启停
  - [x] 3.3.4 任务执行错误写入 `logger.API().Errorw`，不中断调度
  - **不造轮子依据**：`robfig/cron/v3` 是 Go 主流 Cron 库。

- [x] **Task 3.4 [P]**: 实现权限 RBAC `internal/pkg/auth/enforcer.go`（**基于 Casbin**）
  - [x] 3.4.1 `configs/rbac_model.conf` 写入 `request_definition` / `policy_definition` / `role_definition` / `policy_rule` / `matcher`（RBAC with domains）
  - [x] 3.4.2 `configs/rbac_policy.csv` 写入示例策略：`p, alice, user, read`、`g, alice, admin`
  - [x] 3.4.3 `NewEnforcer(modelPath, policyPath) (*casbin.Enforcer, error)`
  - [x] 3.4.4 提供 `RequirePermission(obj, act string) gin.HandlerFunc` 中间件：从 JWT 取 `sub` 作 subject，调用 `enforcer.Enforce(sub, obj, act)`，失败返回 `errcode.ErrForbidden`
  - [x] 3.4.5 提供 `WatchPolicy(onChange)` 监听 `rbac_policy.csv` 热更新（结合 fsnotify，仅用于 policy；model 不热更）
  - **不造轮子依据**：用户硬性指定 **Casbin**。Casbin 是事实标准策略引擎，RBAC/ABAC/CASL 全部覆盖；自实现策略引擎既不必要也易错。

## Phase 4 — 中间件 (P4)

- [x] **Task 4.1**: 实现中间件集合 `internal/pkg/middleware`
  - [x] 4.1.1 `CORS()` 允许配置的 origins/methods/headers/expose_headers/max_age/allow_credentials（从请求头获取 Origin，动态设置）
  - [x] 4.1.2 `NewRateLimiter()` 支持 memory 或 redis 后端，基于 `golang.org/x/time/rate`，自动设置 `X-RateLimit-*` 响应头，触发限流返回 429
  - [x] 4.1.3 `AccessLog(logger)` 记录 API 日志
  - [x] 4.1.4 `JWT(auth)` 解析 Authorization 头 + 校验黑名单
  - [x] 4.1.5 `CSRF(secret)` 校验 `X-CSRF-Token`，GET/HEAD/OPTIONS 跳过（基于 [`gorilla/csrf`](https://github.com/gorilla/csrf) 或 `gin-csrf`）
  - [x] 4.1.6 `Recovery()` 捕获 panic 转 `errcode.ErrInternal` JSON
  - **不造轮子依据**：CORS / CSRF 是社区已有成熟中间件，限流使用 Go 官方令牌桶。

## Phase 5 — 应用与入口 (P5)

- [x] **Task 5.1**: 实现参考应用 `internal/app/admin`
  - [x] 5.1.1 `app.go`：构造 `*gin.Engine`，按顺序挂载中间件链，`TrustedProxies` 从配置读取（为空则使用 gin 默认），`CORS` 从配置读取（无默认值）
  - [x] 5.1.2 `controller/health.go`：`GET /healthz`
  - [x] 5.1.3 `controller/user.go`：示例 CRUD，演示 validator + auth + errcode
  - [x] 5.1.4 `controller/sse.go` + `controller/ws.go`：演示实时通信
  - [x] 5.1.5 `controller/auth.go`：演示 JWT 签发/刷新/登出
  - [x] 5.1.6 `model/user.go`：`User` GORM 模型 + AutoMigrate 注册

- [x] **Task 5.2**: 实现框架入口 `cmd/rocway/main.go`
  - [x] 5.2.1 初始化 logger
  - [x] 5.2.2 加载 config（含热更新 Watch）
  - [x] 5.2.3 初始化 database、cache、storage、scheduler
  - [x] 5.2.4 调用 `admin.New(cfg, deps).Run()`
  - [x] 5.2.5 监听 SIGINT/SIGTERM 优雅关停
  - [x] 5.2.6 命令行 `-c` 参数指定配置文件
  - [x] 5.2.7 请求超时中间件（从 `server.timeout` 读取，0 表示禁用，超时返回 504 + request_id）
  - [x] 5.2.7 HTTP Server `ReadHeaderTimeout` 从配置读取（可选，默认10秒）
  - [x] 5.2.8 HTTPS 支持（可选，`server.tls.enabled`，默认关闭）

- [x] **Task 5.3**: 实现 CLI 脚手架 `cmd/rocway-cli/main.go`（**分级实现**）
  - [x] 5.3.1 顶层入口使用 **cobra** 维护命令树（`rocway-cli new`、`rocway-cli gen`、`rocway-cli version` 等子命令）
  - [x] 5.3.2 `rocway-cli new <name>` 内部使用 **标准库 `flag`** 解析 `-m module`、目标目录等 ≤2 个参数，拷贝 `assets/scaffold/` 到目标目录
  - [x] 5.3.3 `rocway-cli gen controller <name>` 内部使用 **标准库 `flag`** 解析 `-o output` 等，渲染模板到 `internal/app/admin/controller/`
  - [x] 5.3.4 `rocway-cli gen model <name>` 同上 → `internal/app/admin/model/`
  - [x] 5.3.5 `rocway-cli gen middleware <name>` 同上 → `internal/pkg/middleware/`
  - [x] 5.3.6 模板使用 `text/template`，占位符 `{{.Name}}` `{{.Time}}`
  - **不造轮子依据**：cobra 用于多级命令树（带子命令分组与自动补全），简单参数场景下用标准库 `flag` 已足够，避免过度依赖。

- [x] **Task 5.4**: 准备脚手架模板 `assets/scaffold/`
  - [x] 5.4.1 目录结构：`cmd/<name>/main.go`、`internal/app/<name>/`、`configs/config.yaml`、`go.mod.tmpl`
  - [x] 5.4.2 README 提示用户如何 `go mod init` 与 `go run`

- [x] **Task 5.5**: 实现依赖注入 `internal/wire/`（**DDD + Wire**）
  - [x] 5.5.1 `internal/wire/wire.go` 中直接列出所有 Provider（`provideLogger`、`provideEnforcer`、`database.Open`、`cache.New`、`auth.New`、`realtime.NewHub`）
  - [x] 5.5.2 `internal/wire/wire.go` 中 `func InitializeApp(cfg *Config) (*App, func(), error)`，body 为 `wire.Build(...)` 直接列出所有 Provider
  - [x] 5.5.3 执行 `wire ./internal/wire` 生成 `wire_gen.go`
  - [x] 5.5.4 `cmd/rocway/main.go` 调用 `wire.InitializeApp(cfg)`，**不**再手写 `app := admin.New(cfg, logger, db, ...)`
  - [x] 5.5.5 **DDD 原则审计**：仅外部可变依赖（DB / Cache / Storage / MQ / Enforcer / Logger）走 Provider；领域模型 `User`、`Money` 等不写 Provider；工具函数不抽象
  - **不造轮子依据**：用户硬性指定 **Wire**。Wire 是 google 出品的编译期 DI 工具，零反射、零开销；自实现 DI 框架会重蹈 antirez/kelseyhightower 等小众方案的覆辙。

- [x] **Task 5.6**: 命令行注入配置文件
  - [x] 5.6.1 使用标准库 `flag` 解析 `-c` 或 `--config` 参数指定配置文件路径
  - [x] 5.6.2 默认值为 `configs/config.yaml`
  - [x] 5.6.3 命令行参数优先于默认路径
  - [x] 5.6.4 配置文件加载后，环境变量仍可覆盖（最高优先级）

## Phase 6 — 配置与部署产物 (P6)

- [x] **Task 6.1 [P]**: 默认配置 `configs/config.yaml`
  - [x] 6.1.1 server / database / cache / auth / storage / logger 全部段
  - [x] 6.1.2 注释说明每段字段

- [x] **Task 6.2 [P]**: Makefile（根目录）
  - [x] 6.2.1 `make build` / `make run` / `make test` / `make lint` / `make docker`
  - [x] 6.2.2 `make cli` 构建 rocway-cli

- [x] **Task 6.3 [P]**: Dockerfile `build/package/Dockerfile`
  - [x] 6.3.1 多阶段构建：第一阶段 golang:1.25 编译，第二阶段 distroless 运行
  - [x] 6.3.2 非 root 运行、暴露 8080

- [x] **Task 6.4 [P]**: docker-compose `deployments/docker-compose.yml`
  - [x] 6.4.1 rocway + mysql:8 + redis:7 三个 service
  - [x] 6.4.2 healthcheck 保证启动顺序
  - [x] 6.4.3 数据卷挂载 logs / storage

- [x] **Task 6.5 [P]**: `.env.example`、`.gitignore`、`.dockerignore`

## Phase 7 — 验证 (P7)

- [x] **Task 7.1**: 单元测试
  - [x] 7.1.1 `internal/pkg/config` `cache` `auth` `errcode` `validator` 全部覆盖
  - [x] 7.1.2 `go test ./...` 全绿

- [x] **Task 7.2**: 集成 smoke
  - [x] 7.2.1 `make run` 起 admin 应用，curl `/healthz` 返回 200
  - [x] 7.2.2 `make cli && ./bin/rocway-cli new myapp` 生成可运行新项目

- [x] **Task 7.3**: 文档
  - [x] 7.3.1 `docs/quickstart.md`：5 分钟跑通 admin
  - [x] 7.3.2 `docs/architecture.md`：模块依赖图

## Phase 8 — Swagger/OpenAPI 文档 (P8)

> 补充 Phase：添加自动化 API 文档生成与 Swagger UI

- [x] **Task 8.1 [P]**: 安装 swag 工具链
  - [x] 8.1.1 `go install github.com/swaggo/swag/cmd/swag@latest`
  - [x] 8.1.2 Makefile 添加 `swagger: swag init -g cmd/rocway/main.go -o api/docs`
  - [x] 8.1.3 创建 `api/docs/` 目录（swag 输出目录）
  - [x] 8.1.4 首次 `make swagger` 确认 `api/docs/swagger.json` 生成

- [x] **Task 8.2 [P]**: 引入 Swagger 依赖
  - [x] 8.2.1 `go get github.com/swaggo/swag` `github.com/swaggo/gin-swagger` `github.com/swaggo/files`
  - [x] 8.2.2 `go mod tidy` 确认无冲突

- [x] **Task 8.3 [P]**: 定义统一响应模型
  - [x] 8.3.1 创建 `api/response/response.go`
  - [x] 8.3.2 定义 `Response<T>` 含 `code/message/data/request_id`
  - [x] 8.3.3 定义 `PaginatedResponse<T>` 含 `list/total/page/page_size`

- [x] **Task 8.4**: 控制器添加 swag 注释
  - [x] 8.4.1 `controller/health.go`：添加 `@Summary` `@Tags` `@Router`
  - [x] 8.4.2 `controller/auth.go`：登录/注册/刷新/登出全部 handler 添加完整注释
  - [x] 8.4.3 `controller/user.go`：CRUD 全部 handler 添加 `@Param` `@Success` `@Failure` `@Router`
  - [x] 8.4.4 `controller/sse.go` / `controller/ws.go`：添加 `@Summary` `@Tags` `@Router`

- [x] **Task 8.5**: 集成 Swagger UI
  - [x] 8.5.1 创建 `api/router.go` 注册 `/swagger/*` 路由
  - [x] 8.5.2 使用 `ginSwagger.WrapHandler(swaggerFiles.Handler)` 挂载 UI
  - [x] 8.5.3 `admin.NewApp(d Deps)` 中调用 `api.RegisterRoutes(e)`

## Phase 9 — GitHooks 本地质量门禁 (P9)

> 补充 Phase：添加 Git 本地钩子（githooks/ 目录），符合 project_rules.md 约束

- [x] **Task 9.1 [P]**: 创建 GitHooks 目录与脚本
  - [x] 9.1.1 创建 `githooks/pre-commit` 脚本
  - [x] 9.1.2 创建 `githooks/commit-msg` 脚本
  - [x] 9.1.3 脚本添加可执行权限（`chmod +x`）

- [x] **Task 9.2 [P]**: 实现 pre-commit 钩子
  - [x] 9.2.1 检查 Go 代码格式：`gofmt -l .`
  - [x] 9.2.2 执行静态分析：`go vet ./...`
  - [x] 9.2.3 运行单元测试：`go test ./... -short`
  - [x] 9.2.4 全部通过才允许 commit，否则打印错误并 exit 1

- [x] **Task 9.3 [P]**: 实现 commit-msg 钩子
  - [x] 9.3.1 读取 `$1`（commit message 文件）
  - [x] 9.3.2 校验格式：支持 `feat:` `fix:` `docs:` `chore:` `refactor:` `test:` `perf:` `style:` `ci:` `build:` `revert:`
  - [x] 9.3.3 可选：要求 `scope(optional): subject` 格式
  - [x] 9.3.4 格式不符拒绝提交并给出提示

- [x] **Task 9.4**: 配置钩子安装
  - [x] 9.4.1 Makefile 添加 `install-hooks` 目标
  - [x] 9.4.2 `install-hooks` 将 `githooks/*` 链接到 `.git/hooks/`
  - [x] 9.4.3 文档说明：克隆后首次 `make install-hooks`

---

## Task Dependencies

```
0.1 ──▶ 1.1 ──┐
     ├─▶ 1.2 ──┤
     ├─▶ 1.3 ──┤
     └─▶ 1.4 ──┤
                ├─▶ 2.1 ──┐
                ├─▶ 2.2 ──┤
                └─▶ 2.3 ──┤
                           ├─▶ 3.1 ──┐
                           ├─▶ 3.2 ──┤
                           └─▶ 3.3 ──┤
                                      ├─▶ 4.1 ──┐
                                               ├─▶ 5.1 ──▶ 5.2
                                               │
                                               └─▶ 5.3 ◀── 5.4
                                               └─▶ 6.1, 6.2, 6.3, 6.4, 6.5
                                               └─▶ 7.1, 7.2, 7.3
                                               └─▶ 8.1, 8.2, 8.3 ──▶ 8.4 ──▶ 8.5
                                               └─▶ 9.1, 9.2, 9.3 ──▶ 9.4
```

- Phase 1 内 `[P]` 任务彼此独立，可并行。
- Phase 2 内 `[P]` 任务彼此独立，可并行。
- Phase 3 内 `[P]` 任务彼此独立，可并行。
- Phase 4 依赖 Phase 1~3。
- Phase 5 依赖 Phase 1~4。
- Phase 6 与 Phase 5 可并行。
- Phase 7 依赖所有 Phase。
- Phase 8 依赖 Phase 5（需要 admin 应用已注册路由）
- Phase 9 可独立执行，无外部依赖
