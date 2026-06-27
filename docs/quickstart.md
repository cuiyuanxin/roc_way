# roc_way 快速上手（5 分钟跑通 admin）

## 1. 启动 MySQL & Redis

```
docker compose -f deployments/docker-compose.yml up -d mysql redis
```

## 2. 启动 admin

```
go build -o bin/rocway ./cmd/rocway
./bin/rocway
```

## 3. 验证

```
curl http://localhost:8080/healthz
# {"status":"ok"}

curl -X POST http://localhost:8080/auth/login -d '{"user_id":"alice"}' -H 'Content-Type: application/json'
# {"code":0,"data":{"access":"...","refresh":"...","access_exp":...,"refresh_exp":...}}
```

## 4. 使用 CLI 生成新项目

```
go build -o bin/rocway-cli ./cmd/rocway-cli
./bin/rocway-cli new myapp
cd myapp && go mod init github.com/me/myapp && go run ./cmd/myapp
./bin/rocway-cli gen controller order
```

## 5. 一键 docker-compose 全栈

```
docker compose -f deployments/docker-compose.yml up
```

## 6. 开发模式（air 热启动，跨平台）

> 修改 Go 源码后自动重新编译并重启服务，免去手动 Ctrl+C → go build → ./bin/rocway。
> `air` 是**开发工具**，**不**写入 `go.mod`；用 `go install` 装到 `$(go env GOPATH)/bin` 即可。

### 6.1 首次安装

跨平台统一入口（推荐）：

```bash
make install-air
```

或按平台选择：

| 平台 | 命令 |
| --- | --- |
| **Windows (PowerShell)** | `powershell -ExecutionPolicy Bypass -File scripts\install-air.ps1` |
| **Linux / macOS** | `bash scripts/install-air.sh` |

也可以直接：

```bash
go install github.com/air-verse/air@latest
```

### 6.2 启动开发模式

```bash
make dev
```

`.air.toml` 已用 **三段式跨平台配置**（`[build.windows]` / `[build.darwin]` / `[build.linux]`），无需手动切换。可用环境变量：
- `ROCWAY_CONFIG`：指定配置文件路径（默认 `configs/config.yaml`）
- `GIN_MODE`：覆盖 gin 模式（默认按配置 `server.mode`）

### 6.3 平台差异说明

| 关注点 | Windows | Linux / macOS |
| --- | --- | --- |
| 编译产物 | `tmp\main.exe` | `tmp/main` |
| 环境变量赋值 | `set X=Y && prog`（cmd /C 包装） | `X=Y ./prog`（sh 语法） |
| 文件监听 | 强制 `poll = true` | 强制 `poll = true`（docker mount 兼容） |
| 优雅 Ctrl+C | ❌ 不支持（air 限制） | ✅ send_interrupt = true |
