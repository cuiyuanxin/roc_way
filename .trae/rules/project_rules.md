# roc_way 项目规则（永久记忆）

> 本文件由 Trae IDE 在项目初始化阶段写入，记录项目目录结构与用途，用于后续开发时把代码放入对应的目录。**禁止随意删除或修改本文件。**

## 项目背景

- 项目根目录：`d:\WWW\golangProject\roc_way`
- 语言：Go
- Go 模块路径（**module path**）：`github.com/cuiyuanxin/roc_way`  ⚠️ 2026-06-22 由 `github.com/roc_way` 改名为 `github.com/cuiyuanxin/roc_way`，所有 `.go` 文件、`go.mod` 与文档**必须**使用新路径。
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
