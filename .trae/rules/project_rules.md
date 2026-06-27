# roc_way 项目规则（永久记忆）

> 本文件由 Trae IDE 在项目初始化阶段写入，记录项目目录结构与用途，用于后续开发时把代码放入对应的目录。**禁止随意删除或修改本文件。**

## 0. 总约束（本项目所有 Go 代码的最高优先规则）

> **2026-06-25 新增**：本项目所有 Go 代码 **必须** 遵循 `samber/cc-skills-golang` 全套规范，
> 覆盖但不限于：代码风格（code-style）、命名（naming）、错误处理（error-handling）、
> 并发（concurrency）、context 传递、性能（performance / benchmark / data-structures）、
> 测试（testing / testify）、可观测性（observability）、安全（security）、现代化（modernize）、
> 数据库（database）、依赖管理（dependency-management）、依赖注入（google-wire / uber-dig /
> uber-fx / samber-do）、项目布局（project-layout）、lint、troubleshooting、tracing 等。
>
> **开发工作流程技能**（git 规范、CI/CD、PR 审核、文档等）也 **必须优先使用对应技能或工具**，
> 而不是凭直觉处理。例如：写代码前查 `golang-code-style` / `golang-naming` /
> `golang-project-layout`；写并发前查 `golang-concurrency`；写测试前查 `golang-testing`；
> 做性能优化前查 `golang-benchmark` → `golang-performance`；排查问题时查 `golang-troubleshooting`；
> git / CI 操作参考 `golang-continuous-integration`。
>
> **技能注册位置**：技能清单与版本由仓库根目录 [`skills-lock.json`](file:///d:/WWW/golangProject/roc_way/skills-lock.json) 统一维护，
> 安装/索引目录由 IDE 自身管理（具体位置随 IDE 而异，不要硬编码到项目代码里）。
> 当某个新主题不确定该用哪个技能时，优先调用编排技能 `golang-how-to`，
> 它会自动按上下文加载其它相关技能（参考规则 14/16/17 等的写法）。
>
> **与本文件下方 1-19 条规则的关系**：本文件 1-19 条是「本项目特殊约束」
> （目录布局、命名细节、错误响应格式、登录安全等），优先级与 `samber/cc-skills-golang` 一致；
> 当两者冲突时，**先看本文件 1-19 条**（项目特殊 > 通用规范），**没有冲突的部分一律遵循 samber 规范**。

## 0.1 生产级别交付标准（企业级可直接上线的代码基线）

> **2026-06-25 新增**：本项目所有提交/合并的代码 **必须** 以「企业级生产可直接上线」为基线编写，
> 提交前自查清单如下。任何一条不达标，**禁止合并**。

### 0.1.1 正确性优先于速度
- 不允许"先跑通、再补"的占位实现（TODO / panic / 硬编码字符串 / 注释掉的代码）混进主干；
- 任何"临时绕过"必须配 `// TODO(issue#xxx): ...` 注释 + 创建对应 issue，**禁止**静默遗留。

### 0.1.2 安全基线（默认开启，不可降级）
- 输入校验：所有外部输入（HTTP / RPC / 配置 / DB）必经校验，禁止信任客户端；
- 密钥/凭证：禁止明文写代码或配置文件示例（`.env.example` 只放占位符，真实值由部署环境注入）；
- 敏感日志：禁止记录密码、token、密钥、身份证、银行卡、完整 UA 拼接到可定位身份的信息；
- SQL：禁止拼接 SQL，必须使用参数化（`?` / `$1`）；
- 依赖：`go.mod` 引入第三方库前评估维护活跃度、CVE、最近一次发布时间；
- 错误响应：release 环境禁止回显 stack trace / SQL / 内部路径 / 调试字段（参见规则 10）。

### 0.1.3 可观测性（出问题能 5 分钟定位）
- 日志：结构化（zap 或同等），带 `request_id`（参见规则 11）、关键业务字段、时间戳；
- 错误：业务错误必须可分类（`errcode` 体系），禁止直接 `errors.New` 抛给上游；
- 指标：核心路径（登录、支付、下单、限流命中）必加 Prometheus 指标或同等计数器；
- 链路：跨服务调用必带 `request_id` 透传（参见规则 11）；
- 慢请求：超过 P99 阈值的请求必打 warn 日志，含耗时与关键路径。

### 0.1.4 稳定性（故障不雪崩、不静默）
- 资源耗尽：所有外部依赖（DB / Redis / 第三方 HTTP）必须有超时 + 重试策略（推荐：
  超时 → 退避重试 → 降级 → 兜底默认值，绝不允许"调用方傻等"）；
- 并发安全：共享状态必加锁或用 `sync` 原子；goroutine 必有退出信号（`context.Done`），
  禁止泄漏（参见 `golang-concurrency`）；
- 容量边界：循环 / 批量操作必设上限（`LIMIT` / `maxRetry` / `maxConn`），禁止无限增长；
- 幂等性：所有写操作（创建 / 扣款 / 发奖）必须支持幂等或带事务补偿，
  避免重试导致的重复副作用；
- 优雅关停：HTTP / RPC / 后台 worker 必须监听 `SIGTERM/SIGINT`，完成在途请求后排空退出。

### 0.1.5 可测试性
- 新增 service / repository / handler **必须** 配套单测（happy path + 主要边界 + 错误路径）；
- 关键场景（登录、权限、扣款、锁定）必须有集成测试或 e2e；
- 测试禁止依赖真实网络 / 真实 DB（用 mock / sqlmock / testcontainers）；
- CI 上必须跑 `go test ./...` + `go vet ./...` + `golangci-lint run`，全 0 才能合并。

### 0.1.6 可运维性（部署 / 升级 / 回滚）
- 配置变更：所有可调参数走配置文件（`config.yaml`），**禁止**硬编码到代码里；
- 兼容性：DB schema 变更必须提供向前 / 向后迁移脚本（参见规则 19 关于 uniqueIndex 的手动迁移）；
- 特性开关：跨多个 PR 的功能用特性开关（feature flag）控制，避免长分支合并地狱；
- 监控告警：上线必配置告警（错误率 / 延迟 / 依赖健康度），没有告警的功能不准上线。

### 0.1.7 性能与成本
- 关键路径（QPS > 100 的接口）必须有性能基线（p50/p95/p99）记录在 PR 描述；
- 慢查询：所有 DB 查询必 `EXPLAIN` 过，禁止全表扫描；
- 资源占用：内存 / CPU / 磁盘使用有上限意识，禁止无界缓存、无界 channel、无界 map；
- 依赖选择：能用标准库的不引第三方（参见规则 13），避免引入大而重的库（如完整的 ORM
  反而不利于性能调优）。

### 0.1.8 代码可读性 + 可维护性
- 一个函数不超过 ~50 行（特殊工具类除外）；一个文件不超过 ~300 行（拆分的依据见规则 15）；
- 函数命名动词开头；包名小写单词；接口用名词不加 `I` 前缀或 `Impl` 后缀（规则 15）；
- 注释解释 **为什么**，而不是 **做什么**（代码本身就说明做什么）；
- 公共 API / 错误码 / 配置项变更必须在 PR 描述 + CHANGELOG 留痕。

### 0.1.9 与 samber/cc-skills-golang 的协同
- 上述基线是「底线」（不可妥协），`samber/cc-skills-golang` 提供「方法论」（怎么做到）；
- 提交前自查时，**优先调用对应技能**（参见规则 0）：
  - 安全审查 → `golang-security`
  - 性能调优 → `golang-benchmark` → `golang-performance`
  - 故障排查 → `golang-troubleshooting`
  - 测试设计 → `golang-testing` + `golang-stretchr-testify`
  - CI/CD → `golang-continuous-integration`
- **冲突解决**：本节规则与 samber 冲突时，**以本节为准**（企业基线 > 通用建议）。

## 项目背景

- 项目根目录：`d:\WWW\golangProject\roc_way`
- 语言：Go
- Go 模块路径（**module path**）：`github.com/cuiyuanxin/roc_way`  ⚠️ 2026-06-22 由 `github.com/roc_way` 改名为 `github.com/cuiyuanxin/roc_way`，所有 `.go` 文件、`go.mod` 与文档**必须**使用新路径。
  - **历史变更**：
    - 2026-06-22：模块路径由 `github.com/roc_way` 改名为 `github.com/cuiyuanxin/roc_way`（用户手动完成）。后续新增 import 必须沿用新路径。
    - 2026-06-22：`internal/pkg/middleware.Recovery` 升级为「环境自适应」—— release 仅返回 `code/message`，详细 panic 详情仅写 zap 日志，**禁止**在生产响应中回显堆栈或敏感信息。
    - 2026-06-23：新增 `internal/pkg/middleware.RequestID`，基于 `github.com/gin-contrib/requestid` v1.0.6，**优先读请求头 `X-Request-ID`、缺失则自动生成 UUID v4**，并把 ID 同步注入 `gin.Context`、写入响应头；同时所有对外错误响应（Recovery / RateLimit / JWT / CSRF）均在 body 中回带 `request_id` 字段，访问日志与 panic 日志也带 `request_id`。
    - 2026-06-23：删除 `internal/app/admin/app.go` 中自实现的 `timeoutMiddleware`（与"优先使用成熟开源包"原则冲突），统一改用 `github.com/gin-contrib/timeout` v1.2.1，由新增的 `internal/pkg/middleware.Timeout` 包装；同时 `errcode` 新增 `ErrTimeout = Code{1504, "请求处理超时", 504}`。
    - 2026-06-23：引入 `github.com/air-verse/air` 作为开发期热启动工具。配置：根目录 `.air.toml`；入口：`make install-air` + `make dev`；安装脚本：`scripts/install-air.sh`（Linux/Mac 备用）。**air 是开发工具，不写入 `go.mod`**，通过 `go install` 装到 `$(go env GOPATH)/bin`。Windows 因 fsnotify 不可靠，配置已开 `poll = true`。
    - 2026-06-23：air 配置升级为**三段式跨平台**——`.air.toml` 同时声明 `[build.windows] / [build.darwin] / [build.linux]` 三段，air 自动按 `runtime.GOOS` 选择对应 cmd / bin / full_bin；Makefile 用 `go env GOHOSTOS` 检测平台并自动选择 `air.exe` 或 `air`；新增 `scripts/install-air.ps1` 作为 Windows PowerShell 安装入口。Windows 下用 `cmd /C "set X=Y && prog"` 包装环境变量赋值（PowerShell 不支持 `KEY=VAL ./prog` 语法），编译产物必须 `tmp\main.exe`。
    - 2026-06-23：admin 应用执行 DDD 轻量分层重构——新增 `domain/user/`（聚合根 User + Repository 接口）、`application/`（UserService / AuthService）、`infrastructure/repository/`（GORM 仓储实现）、`infrastructure/password/`（bcrypt 实现）；`controller/` 改为薄层（只解析请求、调 service、写响应）；`model/` 改为纯 GORM 映射。Controller 不再直接操作 GORM，业务逻辑完全收敛到 application 层。
    - 2026-06-24：admin 应用执行 **Go 简洁命名 + 基础目录平铺** 的 DDD 重构——`controller/` → `handler/`、`application/` → `service/`、删除 `infrastructure/`、删除 `domain/user/` 嵌套；同时新增 `dto/` 包承载跨层入参 POJO。最终每个应用 5 个平铺基础目录：`domain/`、`repository/`、`service/`、`handler/`、`dto/`（外加 `model/` 持久化映射、`app.go` 组装层）。规则详见第 14 条。
    - 2026-06-24：admin 应用执行 **Go 简洁标识符命名** 收尾——文件名 `user_repo_gorm.go` / `user_repo_iface.go` → `user_gorm.go` / `user_iface.go`（**禁止**下划线分词）；包级函数 `logInfow` / `logWarnw` → `infow` / `warnw`（**禁止**包内调用时还要写包名）；未导出实现 `userRepository` → `userRepo`（**禁止** Java 风格的 `*Impl` / `*Interface` 后缀）。规则详见第 15 条。
    - 2026-06-24：修复 dto binding tag `fieldmatch` 报 `Undefined validation function` 的 runtime 报错——`internal/pkg/validator.New()` 改为**复用 gin 全局 validator engine**（之前新建实例导致 dto binding tag 走旧 engine、规则不共享）；导出 `validator.FieldMatch` 供注册；同时删除冗余的 `Validator.mu` 锁（validator/v10 内部已并发安全）。新增第 16 条（validator 复用 + 禁止自加锁）+ 第 17 条（禁止滥用 `init()`）。
    - 2026-06-25：**统一 HTTP 响应工具收敛到 `internal/pkg/response`**——新建 `internal/pkg/response/response.go`，把原 `api/response/` 的 `Response` / `ErrorResponse` 结构与构造器、`internal/app/admin/handler/response.go` 的 `WriteOK` / `WriteErr` 翻译函数**全部合并**到一个包；**删除** `api/response/` 目录与 `internal/app/admin/handler/response.go`；`middleware`（RateLimit / JWT / CSRF / Timeout）与 `auth.Enforcer` 内 7 处重复的 `c.AbortWithStatusJSON(...response.NewErrorResponse(...))` 模板**全部改用 `response.WriteErr`**，handler / middleware / auth 三层共用同一套「err → HTTP 响应」翻译。**避免循环依赖**的关键：`internal/pkg/response` 内部 `getRequestID` 直接 `c.Get("request_id")`，**禁止** import `internal/pkg/middleware`（否则 `middleware → response → middleware` 闭环）。新增第 18 条（统一响应规则 + 目录归属）。
    - 2026-06-25：**登录安全加固**——补齐 4 项能力：①路由级限流（`/healthz` 与 `/api/auth/login` 各 20次/分钟/IP，`RateLimitOptions` 新增 `Window/Limit` 字段，Redis INCR+EXPIRE 固定窗口算法）；②登录失败锁定（5次连败锁 15min / 10次连败锁 24h，按 username 粒度）；③Redis + MySQL 双存储（Redis 主存 / MySQL 兜底，业务不阻断）；④多登录方式预留（`/api/auth/login/mobile` 路由 + dto 占位 + service stub 返回 501）。新增包 `internal/pkg/notify`（Notifier 接口 + NoopNotifier）、`internal/pkg/janitor`（LoginAuditCleaner 后台清理 + 写入路径在线清理）；`model.User` 加 `Username` 字段（去 email uniqueIndex，需手动迁移）；`service.AuthService` 注入 `LockService` + `Notifier`。spec：[`.trae/specs/2026-06-25-login-security-design.md`](file:///d:/WWW/golangProject/roc_way/.trae/specs/2026-06-25-login-security-design.md)（**注意**：spec 放 `.trae/specs/`，**不**放 `docs/`，避免与对外文档混淆；与现有 `.trae/specs/build-roc-way-framework/` 同级）。新增第 19 条（路由级限流 + 锁定 + 通知约束）。
- 目录布局：基于 [golang-standards/project-layout](https://github.com/golang-standards/project-layout) 社区标准布局

## 目录用途对照表

后续开发新功能、新模块时，必须按下面的规则把代码放入对应的目录。

| 目录 | 作用 | 放什么 | 不放什么 |
| --- | --- | --- | --- |
| `api/` | API 协议定义 | OpenAPI/Swagger 规范、JSON Schema、proto/thrift 等协议文件 | 业务实现代码 |
| `assets/` | 仓库附属资源 | 图片、Logo、其他随仓库分发的静态资源 | 运行时配置、二进制 |
| `build/ci/` | CI 配置 | Travis CI、CircleCI、Drone、Github Actions 等 CI 配置 | 业务代码、Dockerfile |
| `build/package/` | 打包配置 | Docker、deb、rpm、pkg、AMI/云镜像等打包脚本 | 业务代码 |
| `cmd/<app>/` | 应用程序入口 | 每个可执行程序一个子目录，里面只有最小的 `main.go`，导入 `internal` 和 `pkg` 初始化并启动 | 业务逻辑代码（要放到 `internal`/`pkg`） |
| `configs/` | 配置模板 | `confd`、`consul-template` 模板文件，默认配置示例 | 业务代码 |
| `deployments/` | 部署编排 | docker-compose、k8s/helm、mesos、terraform、bosh 等 IaC/PaaS 配置 | 应用源码 |
| `docs/` | 项目文档 | 设计文档、用户手册（除 godoc 之外） | 自动生成的 API 文档 |
| `examples/` | 使用示例 | 公开库或应用的示例代码 | 单元测试、生产代码 |
| `githooks/` | Git 钩子 | pre-commit、commit-msg 等本地 git hooks | CI 配置 |
| `init/` | 系统进程管理 | systemd、upstart、sysv、runit、supervisord 配置 | 业务代码 |
| `internal/app/<app>/` | 私有业务代码 | 不希望被外部项目导入的应用业务实现（按应用分子目录） | 公共库 |
| `internal/pkg/<lib>/` | 项目内部共享库 | 应用之间共享、但不希望外部导入的私有库代码 | 公共 API |
| `pkg/<lib>/` | 公共库代码 | 允许外部项目导入的 Go 公共库（被外部 import 是预期行为） | 应用特定代码 |
| `scripts/` | 脚本 | 构建、安装、分析、生成等各类运维脚本（让根 Makefile 保持精简） | 应用源码 |
| `test/` | 外部测试 | 集成测试、E2E 测试、测试数据（可用 `test/data` 或 `test/testdata` 让 Go 忽略） | 单元测试（应与被测代码同包） |
| `third_party/` | 第三方工具 | fork 的外部代码、Swagger UI 等辅助工具 | 当前项目自有代码 |
| `tools/` | 项目辅助工具 | 构建辅助、代码生成等工具（可 import `pkg` 和 `internal`） | 业务代码 |
| `vendor/` | 依赖 | 第三方依赖（手动管理或由 modules 管理） | 自有业务代码；库项目不要提交 vendor |
| `web/app/` | Web 前端 SPA | 单页应用源文件 | 服务端代码 |
| `web/static/` | Web 静态资源 | JS、CSS、图片等浏览器直接请求的静态文件 | 服务端模板 |
| `web/template/` | 服务端模板 | 服务端渲染的 HTML 模板 | 前端 SPA |
| `website/` | 项目官网 | 不使用 GitHub Pages 时的项目站点数据 | 应用源码 |

## 编码时强制约束

0. **模块路径**：所有 import 必须以 `github.com/cuiyuanxin/roc_way/...` 开头；`go.mod` 中 `module github.com/cuiyuanxin/roc_way`。**严禁**继续使用旧路径 `github.com/roc_way`（编译器 + linter 双重报错）。
1. **新增应用入口**：在 `cmd/<app_name>/main.go` 写最小启动代码，业务逻辑放到 `internal/app/<app_name>/`。
2. **可复用库代码**：
   - 允许外部项目 import → `pkg/<lib_name>/`
   - 不允许外部项目 import → `internal/pkg/<lib_name>/` 或 `internal/app/<app_name>/`
3. **API/协议文件**：放 `api/`，不在业务目录里写协议定义。
4. **配置相关**：模板放 `configs/`，部署编排放 `deployments/`，CI 放 `build/ci/`，打包脚本放 `build/package/`。
5. **测试**：单元测试与被测代码放在同一包内（`_test.go`）；集成测试、E2E 测试、测试数据放 `test/`。
6. **前端资源**：浏览器直接请求的静态文件放 `web/static/`，服务端模板放 `web/template/`，SPA 放 `web/app/`。
7. **第三方依赖**：使用 Go modules（`go.mod`），**不要**把 `vendor/` 目录提交到版本控制。
8. **目录命名**：
   - `cmd/<app>/`、`internal/app/<app>/`、`pkg/<lib>/` 用真实业务名称替换占位符。
   - 目录名与最终可执行文件/包名保持一致。
9. **Go 编译器强制规则**：`internal/` 及其任何层级的子包只允许 `internal/` 同父目录树内的代码导入，公共库放 `pkg/`。
10. **错误响应与环境适配（`internal/pkg/middleware.Recovery` + `api/response`）**：
    - `Recovery` 中间件使用 `response.NewErrorResponse` 统一封装，**禁止**硬编码响应结构。
    - `ErrorResponse` 固定字段：`code`、`message`、`request_id`（详情见下方规则）。
    - `Recovery` 中间件**环境自适应**，由 `gin.Mode()` 决定 details 字段：
      - `debug` / `test` 模式 → `details` 填充 panic 内容，便于本地定位。
      - `release` 模式 → `details` 为 `nil`，**不**泄漏 panic 详情。
    - 无论哪种环境，**完整 panic 详情**（err / path / method / client_ip）**必须**写入 `zap` 日志，便于线上排障。
    - **禁止**在 release 响应中回显 stack trace、SQL、密钥、文件路径等敏感信息。
    - 新增任何对外错误响应中间件/Handler 时，沿用同一原则：debug 详、release 简、详情落日志。
11. **RequestID 链路追踪（`internal/pkg/middleware.RequestID`）**：
    - 唯一 ID 来源是 `github.com/gin-contrib/requestid` 事实标准库，**禁止**自实现 ID 生成。
    - 默认行为：**优先取请求头 `X-Request-ID`（透传，便于跨服务追踪）**，缺失或为空时自动用库默认 `uuid.New().String()` 生成。
    - 注入位置：
      1. `gin.Context`（key 默认 `"request_id"`，可通过 `RequestIDOptions.ContextKey` 自定义）
      2. 响应头 `X-Request-ID`（可通过 `RequestIDOptions.Header requestid.HeaderStrKey` 自定义）
    - 业务层取 ID 一律用 `middleware.GetRequestID(c)`，**禁止**直接 `c.Get("request_id")`。
    - **中间件链顺序强制**：`RequestID` 必须是**第一个**注册的中间件，后续所有中间件（`AccessLog` / `Recovery` / `JWT` / `CSRF` / `RateLimit`）才能从 context 读到 `request_id` 并写入日志 / 错误响应。
    - **所有对外错误响应 body 必须带 `request_id` 字段**（无敏感信息，便于前端报错时反馈给后端定位日志）。
12. **请求超时（`internal/pkg/middleware.Timeout`）**：
    - 唯一实现来源是 `github.com/gin-contrib/timeout` 事实标准库，**禁止**自实现超时逻辑（已删除 admin 中的 `timeoutMiddleware`）。
    - 调用方式：`middleware.Timeout(d time.Duration) gin.HandlerFunc`，`d <= 0` 时返回 `nil`（配置 0 = 不启用）。
    - 触发后响应 `504 + {"code":1504, "message":"请求处理超时", "request_id":...}`，与项目其它错误响应格式一致。
    - 接入顺序：放在 `RequestID / Recovery / AccessLog` 之后、`Auth / Controller` 之前，确保 `request_id` 已注入。
13. **开发模式热启动（air，跨平台）**：
    - 唯一使用 `github.com/air-verse/air`（非 `cosmtrek/air`，旧仓库已归档），**禁止**自实现文件监听+重启。
    - air 是**开发期工具，不写入 `go.mod`**，通过 `go install github.com/air-verse/air@latest` 装到 `$(go env GOPATH)/bin`。
    - 配置文件：仓库根目录 `.air.toml`，**禁止**散落多份。
    - 接入方式（**跨平台统一入口**）：
      - 首次安装：`make install-air`（跨平台通用）或 `bash scripts/install-air.sh`（Linux/Mac）/ `powershell -ExecutionPolicy Bypass -File scripts\install-air.ps1`（Windows）。
      - 启动开发模式：`make dev`（等价于 `air -c .air.toml`，Makefile 自动按 `GOHOSTOS` 选 `air.exe` 或 `air`）。
    - **跨平台强制**：
      - **`.air.toml` 必须三段式**：`[build.windows]` / `[build.darwin]` / `[build.linux]`，每段独立声明 `cmd` / `bin` / `full_bin`，**禁止**依赖基础段默认值。
      - **Windows 必加 `.exe` 后缀**：`tmp\main.exe`，`full_bin` 用 `cmd /C "set X=Y && .\tmp\main.exe"` 包装（PowerShell 不支持 `KEY=VAL ./prog` 语法）。
      - **全平台开 `poll = true`**：Win 下 fsnotify 不可靠；Mac/Linux 偶发 docker mount 场景也建议 poll，更稳。
      - `send_interrupt` 仅 macOS / Linux 开启（Windows 不支持）。
      - 监听后缀：`include_ext = ["go", "yaml", "yml", "toml"]`。
      - 排除目录必须包含：`tmp`、`bin`、`vendor`、`web`、`api/docs`、`internal/wire`、`.git`、`.idea`、`.vscode`。
    - **Makefile 跨平台检测**：**禁止**用 `uname`（Windows 上不可靠），用 `$(GO) env GOHOSTOS` 判断。
    - 临时产物（`/tmp/`、`/air_errors.log`）由 `.gitignore` 屏蔽，**禁止**提交。
14. **DDD 分层（Go 简洁命名，按基础目录平铺）**：
    - 业务应用统一放在 `internal/app/<app>/` 下，**采用 5 个基础目录**（每个都是平铺一层，不再嵌套子包）：
      | 目录 | 职责 | 放什么 | 典型文件 |
      | --- | --- | --- | --- |
      | `domain/` | 领域层 | 聚合根 + 领域错误 + 领域服务接口（纯 Go，无任何框架依赖） | `user.go`、`user_test.go` |
      | `repository/` | 仓储层 | 仓储接口 + 持久化实现（GORM / 内存 / 缓存等），**接口和实现可同包** | `user_iface.go`、`user_gorm.go` |
      | `service/` | 应用服务层 | 业务编排用例（注册、登录、改昵称…），接收 dto 输入、返回 domain 实体 | `user.go`、`auth.go`、`bcrypt.go` |
      | `handler/` | HTTP 表现层 | 路由注册 + 请求解析 + 调用 service + 写响应；**薄**，禁止业务逻辑 | `user.go`、`auth.go`、`health.go` |
      | `dto/` | 跨层 POJO | 跨层传递的纯数据入参/出参，**禁止**放 gin binding / ORM 标签 | `user.go` |
    - **禁止** Java 风格嵌套：
      - `domain/user/entity.go`、`domain/user/repository.go` ← **不允许**（合并到 `domain/user.go`）
      - `application/user_service.go` ← **不允许**（用 `service/user.go`）
      - `controller/user_controller.go` ← **不允许**（用 `handler/user.go`）
      - `infrastructure/persistence/...` ← **不允许**（用 `repository/...`）
      - `interfaces/dto/...` ← **不允许**（用 `dto/...`）
    - **基础目录必须存在**：即使当前只有一个聚合也要保留这 5 个目录（如 `dto` 当前只有入参，仍要建 `dto/` 包），**禁止**把所有逻辑塞到 `app.go` 单文件。
    - **dto 设计原则**：
      - dto 放「跨层传递的纯数据」，无方法、无 `binding` / `gorm` / `json` 以外的标签。
      - HTTP 请求体里带 `binding:"required,email"` 的 `registerReq`、`loginReq` **留在 handler 包内**（避免 dto 反向依赖 gin）。
      - service 层方法签名直接接收 `dto.XxxInput`，handler 用 `dto.XxxInput{...}` 构造入参。
    - **依赖方向**：handler → service → domain ← repository；dto 可被 handler、service、test 引用，但 dto 本身**禁止**反向依赖它们中的任何一个。
15. **Go 简洁命名约束（`.go` 文件 / 标识符）**：
    - **文件名**：**禁止**下划线分词（`user_repo_gorm.go`、`http_response.go` 这种**不允许**），统一用 Go 社区短文件名：
      | 含义 | 推荐 | 禁止 |
      | --- | --- | --- |
      | 仓储接口 | `user_iface.go` | `user_repo_iface.go`、`user_repository_iface.go` |
      | 仓储 GORM 实现 | `user_gorm.go` | `user_repo_gorm.go`、`user_repository_gorm.go` |
      | HTTP 响应工具 | `response.go` | `http_response.go`、`resp.go`（除非真的太长） |
      | 业务用例 | `user.go` / `auth.go` | `user_service.go`、`user_handler.go` |
    - **包名**：小写单词，**禁止**复数（`handler` 而非 `handlers`，`service` 而非 `services`），**禁止**下划线/驼峰（`httputil` 而非 `http_util` / `httpUtil`）。
    - **类型 / 接口**：
      - 导出类型用 **PascalCase**，**禁止** `IUserRepository` 这种 Java/IOS 前缀，**禁止** `UserRepositoryInterface` 后缀。接口直接用名词：`UserRepository`、`Hasher`、`Clock`。
      - 未导出实现用 **lowerCamelCase**：接口 `UserRepository` 的 GORM 实现 → `userRepo`（不是 `userRepository` / `UserRepositoryImpl` / `userRepositoryImpl`）。
    - **构造函数**：`New + 类型名`，**禁止** `NewUserServiceInstance` / `CreateUserService` 这种冗余后缀；返回错误一律 `NewUserRepository(...) (UserRepository, error)` 或 `(UserRepository, error)`。
    - **包内私有函数**：**包内调用方调用时省略包名**（这是 Go 风格的核心）——因此不要把包级函数命名为 `logInfow` / `logWarnw` / `httpResponse`，直接叫 `infow` / `warnw` / `respond`；调用方写 `infow(s.apiLog(), ...)` 而不是 `logInfow(s.apiLog(), ...)`。
    - **方法名**：动词或动词短语，**禁止** `doRegister`、`handleLogin` 这种前缀式命名；直接 `Register`、`Login`、`Delete`、`Save`。
    - **变量 / 常量**：
      - 局部变量用 **lowerCamelCase**，**禁止** `userName` 之外的下划线（`user_name` 不允许）。
      - 常量统一 `MaxLength`、`ErrNotFound` 这种 PascalCase（即使是包内私有）。
      - **禁止** `iUser`、`strName`、`bFlag` 这种 **类型前缀匈牙利命名**。
      - **禁止** `temp`、`data`、`info`、`result` 这种语义模糊的单字名——要么具体（`cached`、`user`、`totalErr`），要么注释说清。
    - **方法接收者名**：1-2 字母小写，与类型名首字母相关：`func (s *UserService)` / `func (r *userRepo)` / `func (u *User)`，**禁止** `self` / `this` / `me`。
    - **包注释**：每个包**必须有** `Package <name> <一句话职责>` 开头，**禁止** `// Package user 是用户聚合的领域层 package user` 这种「包名 = 文件名主体」的过期写法——包名以**目录功能**为单位（`domain`、`repository`、`service`、`handler`、`dto`、`model`），不按聚合分目录时不要在包注释里写「user 聚合」。
    - **GORM / JSON 等 tag 命名**：snake_case 列名（`password_hash`、`created_at`），与数据库惯例一致即可，**不**受 Go 变量命名风格约束（这是结构体 tag 而不是 Go 标识符）。
16. **validator/v10 使用约束（`internal/pkg/validator`）**：
    - **复用 gin 全局 validator engine**（**禁止**新建 `validator.New()` 实例）：
      ```go
      // ✅ 正确：复用 gin 单例
      v, ok := binding.Validator.Engine().(*validator.Validate)
      
      // ❌ 错误：新建实例会导致 dto 的 binding tag 走的还是 gin 旧 engine
      v := validator.New()
      ```
      原因：dto 的 `binding:"fieldmatch=..."` tag 在 `c.ShouldBindJSON` 时走的是 `binding.Validator.Engine()` 这个全局单例；如果你 `validator.New()` 建一份新的，`vs.Struct()` 走新 engine，dto binding tag 走旧 engine，两边规则不共享 → **dto 的自定义 tag 失效**。
    - **禁止**自加锁保护 `RegisterValidation` / `RegisterTranslation`：
      - `validator.Validate` 内部已用 `sync.RWMutex` 保护，自加锁是冗余且会引入死锁风险（外层锁 + 内层锁的获取顺序不一致）。
      - 并发注册同一 tag 的行为是「后注册者覆盖前注册者」，不会 panic，无需外部锁。
    - **自定义规则注册位置**：在 `internal/pkg/validator.New()` 里集中注册内置 tag（如 `fieldmatch`），业务侧通过 `WithRule(...)` 注入。**禁止**在 `cmd/`、`main.go`、各 dto 文件里散落注册。
    - **dto binding tag 与自定义规则的依赖关系**：dto 写的 `binding:"...,fieldmatch=..."` 依赖 `validator.New()` 已被调用过（注册了 fieldmatch）。约定：任何启动 admin 应用的入口都必须经过 `admin.NewApp` → `rocvalidator.New()`，handler 才会拿到已注册的自定义规则。
    - **新增自定义 tag 的步骤**（按顺序）：
      1. 在 [internal/pkg/validator/validator.go](file:///d:/WWW/golangProject/roc_way/internal/pkg/validator/validator.go) 新增导出函数 `FieldXxx(fl validator.FieldLevel) bool`；
      2. 在 `New()` 里追加 `_ = vs.v.RegisterValidation("xxx", FieldXxx)` + `vs.registerTranslationLocked("xxx", "{0} 中文", "{0} english")`；
      3. dto 在 binding tag 里写 `binding:"...,xxx=..."` 即可使用，**不需要**再改 main.go 或 wire.go。
17. **`init()` 函数使用约束**：
    - **禁止**使用 `func init()` 做框架级副作用（注册全局 validator、连接池、gRPC client 等）：
      - `init()` 是**包级全局副作用**，违反显式依赖原则；任何 import 该包的代码都会被无差别触发，不可观察、不可 mock、单测无法隔离。
      - 启动期副作用必须**显式调用**，调用方看得到、能 defer、能加日志。
    - **正确做法**：
      - 启动钩子：放在 `cmd/<app>/main.go` 的 `run()` 里，按依赖顺序**显式调用**，例如 `registerValidators() → wire.InitApp(...) → srv.ListenAndServe()`。
      - 框架内复用：放在框架包的 `New()` 构造函数里（如 `validator.New()`），调用方拿到对象时副作用已完成。
      - 真正的延迟初始化：用 `sync.Once` 封装在**第一次真正用到时**触发，不是 init。
    - **唯一允许 `init()` 的场景**：与标准库互操作的硬性要求（例如 `image.RegisterFormat` 这种官方要求注册到全局表的情况）。
18. **统一 HTTP 响应工具（`internal/pkg/response`）**：
    - 所有对外 HTTP 响应**必须**走 `internal/pkg/response`，**禁止**手写 `c.JSON(...)` / `c.AbortWithStatusJSON(...)` 错误响应模板：
      ```go
      // ✅ 正确：handler / middleware / auth 三层都调同一套
      response.WriteOK(c, data)
      response.WriteErr(c, err)               // 传 errcode.Error / errcode.Code / 未知 error 均可
      response.WriteErr(c, errcode.ErrTokenInvalid)
      
      // ❌ 错误：手写模板会导致响应格式漂移（漏 request_id / details 处理不一致）
      c.JSON(200, gin.H{"code": 0, "data": user})
      c.AbortWithStatusJSON(401, response.NewErrorResponse(2002, "Token 无效", rid, nil))
      ```
    - **`internal/pkg/response` 是 gin 适配层**（写 `c.JSON`），**禁止**塞进 `api/`（`api/` 是协议层，必须无 gin 依赖）；也**禁止**塞进 `pkg/`（外部项目不该被绑定到 gin 框架）。
    - **包职责**：
      - `Response[T] / ErrorResponse / PaginatedResponse[T]` —— 协议结构（保持 `example` tag，方便 swag 文档生成）
      - `NewResponse / NewErrorResponse / NewPaginatedResponse` —— 纯函数构造器（无 gin 依赖，便于单测和 mock）
      - `WriteOK(c, data) / WriteErr(c, err)` —— gin 适配层（**唯一允许**调 `c.JSON` 的地方）
    - **`WriteErr` 错误翻译规则**：
      - `*errcode.Error` → 用其 `C.HTTPStatus / C.Code / C.Message`，`details = nil`
      - `errcode.Code`   → 直接用其 `HTTPStatus / Code / Message`，`details = nil`
      - 其它 `error`     → 走 `errcode.ErrInternal`（HTTPStatus 500, Code 5000），`message = err.Error()`（与原来 middleware / auth 兜底行为一致）
      - 自定义 message 走 `errcode.ErrXxx.WithMessage("...")`，**禁止**在 middleware / handler 里临时拼装。
    - **`request_id` 注入与循环依赖规避**：
      - `internal/pkg/response` **禁止** import `internal/pkg/middleware`（否则 `middleware → response → middleware` 闭环）。
      - `response` 包内置私有 `getRequestID(c)`，直接 `c.Get("request_id")`，key 字符串 `"request_id"` 与 `internal/pkg/middleware.DefaultRequestIDContextKey` **必须**保持一致；如改 key，**两处同步**。
      - 调用方**禁止**自己 `c.Get("request_id")` 后再传给 `response.NewErrorResponse`（破坏抽象）；要么 `response.WriteErr(c, err)` 要么 `middleware.GetRequestID(c)` 单独使用。
    - **唯一例外**：`internal/pkg/middleware.Recovery` 需要传 `details`（debug 模式填 panic 详情），仍用 `c.AbortWithStatusJSON(errcode.ErrInternal.HTTPStatus, response.NewErrorResponse(...))` 直调构造器；其它 middleware / handler 一律走 `response.WriteErr`。
    - **新增错误码**：在 `internal/pkg/errcode` 加 `ErrXxx = Code{...}`，**禁止**在业务包或 handler 里就地定义 `errcode.Code`。
19. **登录安全（路由级限流 + 失败锁定 + 通知）**：
    - **双层限流**（机器承载力 + 接口级防刷）：
      - 全局限流保留 RPS/Burst 令牌桶语义（`e.Use(globalLimitMw)`，所有请求计数）。
      - 路由级限流**新增** `Window + Limit` 字段（`Window time.Duration, Limit int`），走 Redis `INCR + EXPIRE` 固定窗口；按 `RouteLimits` 配置在路由处挂载（`e.GET(path, routeLimitMw, handler)`）。
      - **顺序强制**：全局先、路由后——超全局配额直接拦截，不浪费路由级 INCR 计数。
      - 全局 key = `rl:global:{ip}`；路由级 key = `rl:route:{KeyPrefix}:{ip}`（KeyPrefix 由 `RouteLimitConfig.KeyPrefix` 注入）。
    - **失败锁定**（按 username 粒度）：
      - 阈值：5 次连败锁 15 分钟（`LockShort`），10 次连败锁 24 小时（`LockLong`）。
      - **成功登录重置失败计数**（Redis Del + DB delete failure 记录）；**不删除 lock 记录**（防攻击者试探到 4 次后故意输对 1 次再继续）。
      - 锁定到期 → 自动解锁（Redis TTL 自然过期；DB 记录由 janitor 清理）。
    - **Redis + MySQL 双存储**：
      - Redis 主存（key 前缀 `auth:fail:` / `auth:lock:short:` / `auth:lock:long:`）。
      - MySQL 兜底：`login_audits` 单表（`event_type` ∈ {`failure`, `lock_short`, `lock_long`}，`occurred_at` 索引）。
      - Redis 故障降级：读失败查 DB；写失败写 DB + zap warn；DB 也失败 → zap error + **业务不阻断**。
      - **禁止**在 service 层直接调 cache + DB；双写逻辑封装在 `service/lock.go`（`LockService`）。
    - **Notifier 通知**（`internal/pkg/notify`）：
      - 接口签名强制：**`Notify(ctx, Event)` 不返回 error、不 panic**——实现体内部 swallow 错误并 zap 日志，避免「推送系统故障拖垮登录」。
      - 默认实现 `NoopNotifier`：仅 `logger.Security()` channel 输出（`Warnw "security_event"`）。
      - 未来接邮件 / 钉钉 / IM 时**新增实现体**，业务代码**零改动**。
    - **janitor 后台清理**：
      - `internal/pkg/janitor.LoginAuditCleaner`：`time.NewTicker(24*time.Hour)` 触发 `DELETE WHERE occurred_at < now() - 24h`。
      - `app.go` 启动 goroutine；`App.Close()` 调 cancel，**禁止** goroutine 泄漏。
      - **写入路径在线清理**：每次 `RecordFailure` 后附 `DELETE ... LIMIT 1000`，避免 janitor 单次 DELETE 过多行（DB 长事务）。
    - **username 字段**：
      - `model.User` 加 `Username string gorm:"size:64;uniqueIndex"`，email 字段**去 uniqueIndex**（保留兼容索引）。
      - **生产环境**加 uniqueIndex **必须手动迁移**：`ALTER TABLE users ADD COLUMN username ...` → `UPDATE SET username = CONCAT('user_', id)` → `ALTER TABLE ... ADD UNIQUE INDEX`；**禁止**依赖 GORM AutoMigrate 加 uniqueIndex（可能锁表）。
      - `domain.User.Validate()` 加 username 校验（5-24 位字母数字下划线短横线）。
      - `repository.UserRepository` 加 `FindByUsername`；`FindByEmail` 保留兼容。
      - `dto.LoginInput` 字段 `Email → Username`，新增 `IP string`（handler 注入 `c.ClientIP()`）。
    - **多登录方式预留**：
      - `POST /api/auth/login`：**真正实现** username + password 登录。
      - `POST /api/auth/login/mobile`：**handler 路由 + dto 占位 + service stub**，统一返回 `errcode.ErrNotImplemented`（HTTPStatus 501）。
      - 未来接手机号 + 短信验证码时**只填 service stub**，路由 / dto 已就位。
    - **新增错误码**（`internal/pkg/errcode`）：
      - `ErrAccountLocked = Code{2005, "账号已锁定，请稍后再试", 423}`（HTTP 423 Locked）
      - `ErrNotImplemented = Code{2006, "功能未实现", 501}`
      - **禁止**在 service / handler 里就地定义 `errcode.Code`。
20. **JWT 签名算法选型（HS256 智能 secret 管理，Phase 2.5 决策）**：
    - **算法固定 HS256**（行业标准；单服务后台脚手架首选；与 gin-jwt / go-admin / go-zero 一致）。
      - **禁止**无脑升 RS256：单服务架构下 RS256 的「公私钥分离」优势用不上，徒增 ~150 行 PEM 解析代码。
      - RS256 仅在**多服务架构 / BFF / 前端本地验签**场景才有意义；本项目是单服务后台，不需要。
    - **secret 三级回退加载顺序**（`internal/pkg/auth.New`）：
      1. **环境变量 `JWT_SECRET`**（生产推荐，K8s Secret / Vault 注入）
      2. **`config.yaml` 的 `auth.jwt_secret` 字段**（本地 / 单一节点）
      3. **自动生成到 `configs/.jwt_secret`**（dev 模式专用，文件保留重启不丢）
    - **`production_mode` 硬开关**（`auth.production_mode`，`mapstructure:"production_mode"`）：
      - `true` 时：禁止 dev fallback，必须显式提供 secret，**否则启动 panic**（不会带病上线）。
      - `false` 时：允许 dev fallback（自动生成 secret）。
      - **推荐**：本地开发 `false`，部署到任何正式环境都设 `true`。
    - **secret 强度强制**：
      - 启动时校验 `len(secret) >= 32`（OWASP HS256 推荐 ≥ 256 bit）；< 32 启动失败。
      - 启动横幅只打印**来源**（env/config/file）与**长度**，**禁止**打印 secret 内容。
      - dev 模式 banner 用 `WARN` 等级（醒目的 ⚠️ 符号 + `warning` / `fix` 字段）。
    - **文件权限**：
      - `configs/.jwt_secret` 写入时 `chmod 600`（Linux/macOS）/ `icacls inheritance:r`（Windows）。
      - `.gitignore` 必须屏蔽 `/configs/.jwt_secret` 与 `*.bak`。
    - **配套工具**：
      - `scripts/gen-jwt-secret.{ps1,sh}`：openssl 强随机生成（默认 48 字节 = 384 bit）。
      - `make gen-jwt-secret` 跨平台入口；`make gen-jwt-keys` 已废弃。
      - `make install-mkcert` 仍保留（HTTPS cert 与 JWT secret 是两件事）。
    - **Token 安全特性（与算法无关，HS256/RS256 通用）**：
      - AccessToken TTL 默认 2h（`access_ttl_sec: 7200`）；RefreshToken TTL 默认 7d。
      - **Refresh Token Rotation**：每次 refresh 换新一对 token，旧 refresh 立即进黑名单。
      - **黑名单**（Redis `auth:blacklist:{jti}`）：TTL = token 剩余有效期，吊销单 token。
      - **DeviceID 绑定**（Claims `device_id` 字段）：登录时绑定设备指纹，中间件校验 `X-Device-ID` 一致性。

## 后续开发检查清单

每次新增代码前问自己：

- [ ] 这是入口程序吗？→ `cmd/`
- [ ] 是允许外部 import 的公共库吗？→ `pkg/`
- [ ] 是项目内部共享但禁止外部 import 的库吗？→ `internal/pkg/`
- [ ] 是某个应用独有的业务逻辑吗？→ `internal/app/<app>/`
- [ ] 是 API/协议定义吗？→ `api/`
- [ ] 是 CI/打包/部署相关吗？→ `build/` 或 `deployments/`
- [ ] 是文档吗？→ `docs/`
- [ ] 是脚本吗？→ `scripts/`
- [ ] 是测试数据或集成测试吗？→ `test/`
- [ ] 是前端相关吗？→ `web/`

## 备注

- 本规则文件由 AI 助手写入，作为项目的"长期记忆"。用户后续要求开发新功能时，助手应**优先参照本文件**决定代码位置。
- 如需调整目录布局或新增自定义目录，请同步更新本文件后再修改目录结构。
