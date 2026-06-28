# rocway Makefile

GO ?= go
WIRE ?= $(shell go env GOPATH)/bin/wire
SWAG ?= $(shell go env GOPATH)/bin/swag
BIN := bin

# ============== 跨平台检测 ==============
# 用 Go 的 runtime.GOOS 字符串判断（比 uname 更可靠，避免 Windows 上 Make
# 自带的 shell 与 uname 行为差异）。通过 `go env GOHOSTOS` 取值，输出 windows / linux / darwin。
GOHOSTOS := $(shell $(GO) env GOHOSTOS)
# 编译产物后缀：Windows 是 .exe，其它平台空
EXE_SUFFIX := $(if $(filter windows,$(GOHOSTOS)),.exe,)

# air 二进制路径（带平台后缀）
ifeq ($(GOHOSTOS),windows)
  AIR_BIN := $(shell $(GO) env GOPATH)/bin/air.exe
else
  AIR_BIN := $(shell $(GO) env GOPATH)/bin/air
endif

.PHONY: help tidy wire build run test vet fmt lint clean cli docker swagger install-hooks install-air install-mkcert certs gen-jwt-secret dev

help:
	@echo "make tidy           - go mod tidy"
	@echo "make wire           - regenerate wire_gen.go"
	@echo "make swagger        - generate swagger docs"
	@echo "make build          - build rocway and rocway-cli to bin/"
	@echo "make run            - run rocway locally"
	@echo "make cli            - build rocway-cli only"
	@echo "make test           - go test ./..."
	@echo "make vet            - go vet ./..."
	@echo "make fmt            - gofmt -w ."
	@echo "make docker         - build docker image"
	@echo "make install-hooks  - install git hooks to .git/hooks/"
	@echo "make install-air    - install air (live reload) to \$$(go env GOPATH)/bin"
	@echo "make install-mkcert - install mkcert (local HTTPS cert tool) via scripts/install-mkcert.*"
	@echo "make certs          - generate local HTTPS certs to configs/certs/ (dev only)"
	@echo "make gen-jwt-secret - generate HS256 random secret to configs/.jwt_secret (Phase 2.5)"
	@echo "make dev            - run rocway with air hot reload"

tidy:
	# `-e`: 容忍 .trae/skills/**/examples/ 等 skill 自带示例里
	#       `github.com/you/myapp/cmd` 这类占位 import 解析失败 —— 这些是
	#       reference 示例,不应该被当作项目源码扫描,但 Go 工具链会全盘扫。
	#       主项目依赖仍按正常流程 tidy 完毕。
	$(GO) mod tidy -e

wire:
	@if [ ! -x "$(WIRE)" ]; then $(GO) install github.com/google/wire/cmd/wire@latest; fi
	$(WIRE) ./internal/wire

swagger:
	@if [ ! -x "$(SWAG)" ]; then $(GO) install github.com/swaggo/swag/cmd/swag@latest; fi
	mkdir -p api/docs
	$(SWAG) init -g cmd/rocway/main.go -o api/docs

build: wire
	mkdir -p $(BIN)
	$(GO) build -o $(BIN)/rocway ./cmd/rocway
	$(GO) build -o $(BIN)/rocway-cli ./cmd/rocway-cli

cli: wire
	mkdir -p $(BIN)
	$(GO) build -o $(BIN)/rocway-cli ./cmd/rocway-cli

run: build
	./$(BIN)/rocway

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint: vet fmt

clean:
	rm -rf $(BIN)

docker:
	docker build -f build/package/Dockerfile -t rocway:latest .

install-hooks:
	@echo "Installing git hooks..."
	@chmod +x githooks/pre-commit githooks/commit-msg
	@ln -sf ../../githooks/pre-commit .git/hooks/pre-commit
	@ln -sf ../../githooks/commit-msg .git/hooks/commit-msg
	@echo "Git hooks installed successfully!"

# 安装 air（开发期热启动工具）。
# 注意：air 是开发工具，不写入 go.mod；装到 $(go env GOPATH)/bin 即可。
# 跨平台处理：使用 $(GO) install，Go 工具链自动选正确的可执行后缀
# （Linux/macOS → air，Windows → air.exe）。
install-air:
	@echo "→ installing github.com/air-verse/air@latest ..."
	$(GO) install github.com/air-verse/air@latest
	@if [ -x "$(AIR_BIN)" ]; then echo "✔ air installed at $(AIR_BIN)"; else echo "✘ air not found at $(AIR_BIN)"; fi

# 开发模式：air 监听源码变化，自动重新编译并重启服务。
# 前置条件：先 make install-air。
# 配置文件：.air.toml（已用 [build.windows] / [build.darwin] / [build.linux]
# 三段式跨平台配置，无需手动切换）。
dev: install-air
	$(AIR_BIN) -c .air.toml

# 安装 mkcert（本地可信 HTTPS 证书生成工具）。
# mkcert 是开发工具，不写入 go.mod。优先用系统包管理器，兜底走 go install。
# 跨平台分脚本（参考 install-air 的拆分模式）：
#   - Linux/macOS → scripts/install-mkcert.sh
#   - Windows     → scripts/install-mkcert.ps1
install-mkcert:
ifeq ($(GOHOSTOS),windows)
	@powershell -ExecutionPolicy Bypass -File scripts/install-mkcert.ps1
else
	@bash scripts/install-mkcert.sh
endif

# 生成本地 HTTPS 证书到 configs/certs/，供开发环境使用。
#
# 注意：
#   - 这是**开发期**证书（mkcert 本地 CA 签发），**禁止**用于生产环境。
#   - 真实证书由部署环境的 cert-manager / Let's Encrypt 申请，与本目标无关。
#   - 生成路径与 configs/config.yaml 的 server.tls.cert_file / key_file 对应。
#
# 流程：
#   1. 检查 mkcert，未安装则自动 install
#   2. mkcert -install（写入本地 CA 到系统/浏览器信任库）
#   3. mkcert -cert-file server.crt -key-file server.key localhost 127.0.0.1 ::1
#      （带 localhost / 127.0.0.1 / ::1 三个常用地址，足够本地起服务测试）
#
# 跨平台 PATH 解析：
#   - Windows 上 winget 把 mkcert 装到 %LOCALAPPDATA%\Microsoft\WindowsApps
#     并只在该 shell 的 PATH 里注册为"cmd 别名"，make 走的 sh 不会展开别名，
#     所以必须用 `where` 解析绝对路径。
#   - Linux/macOS 用 `command -v` 拿到 GOBIN 下的真实路径。
#
# 强制把 mkcert 路径注入 PATH（避免「winget 别名 + 新 shell」典型坑）。
ifeq ($(GOHOSTOS),windows)
  MKCERT_BIN := $(shell where mkcert 2>nul)
  ifeq ($(MKCERT_BIN),)
    # where 找不到 → 兜底查 GOBIN
    MKCERT_BIN := $(shell $(GO) env GOPATH)/bin/mkcert.exe
  endif
else
  MKCERT_BIN := $(shell command -v mkcert 2>/dev/null || echo "$(shell $(GO) env GOPATH)/bin/mkcert")
endif
certs: install-mkcert
	@if [ -z "$(MKCERT_BIN)" ] || [ ! -x "$(MKCERT_BIN)" ]; then \
		echo "✘ mkcert not found. Open a NEW terminal and re-run 'make certs' (winget path takes effect after PATH refresh)"; \
		exit 1; \
	fi
	@mkdir -p configs/certs
	"$(MKCERT_BIN)" -install
	"$(MKCERT_BIN)" -cert-file configs/certs/server.crt -key-file configs/certs/server.key localhost 127.0.0.1 ::1
	@echo "✔ HTTPS certs generated:"
	@ls -l configs/certs/server.crt configs/certs/server.key 2>/dev/null || dir configs\certs\server.crt configs\certs\server.key

# Phase 2.5: 生成 HS256 随机 secret 到 configs/.jwt_secret。
#
# 配套：
#   - auth.New 启动时三级回退加载 secret：env JWT_SECRET > config > 配置文件
#   - 本命令生成的 dev 模式 secret 写入 configs/.jwt_secret（mode 600）
#   - configs/.jwt_secret 已被 .gitignore 屏蔽
#   - 已有同名文件自动备份为 .bak
#
# 跨平台：依赖系统 openssl（Linux/macOS 自带；Windows 通过 Git Bash / WSL / choco 装）。
# 用法：make gen-jwt-secret [BYTES=64]
#
# 为什么不直接用 RS256：单服务后台脚手架不需要公私钥分离，行业标准（gin-jwt / go-admin /
# go-zero）都使用 HS256 + 强 secret。代码量比 RS256 少 70%，且同样安全。
BYTES ?= 48
gen-jwt-secret:
ifeq ($(GOHOSTOS),windows)
	@powershell -ExecutionPolicy Bypass -File scripts/gen-jwt-secret.ps1 -Bytes $(BYTES)
else
	@bash scripts/gen-jwt-secret.sh $(BYTES)
endif
