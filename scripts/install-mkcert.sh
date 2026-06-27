#!/usr/bin/env bash
# scripts/install-mkcert.sh
#
# 一键安装 mkcert（本地可信 HTTPS 证书生成工具）。
# **仅限 Linux / macOS 平台**。Windows 用户：
#   - PowerShell: powershell -ExecutionPolicy Bypass -File scripts\install-mkcert.ps1
#   - 或: make certs（跨平台通用入口）
#
# 用法：bash scripts/install-mkcert.sh
#
# mkcert 项目主页：https://github.com/FiloSottile/mkcert
# mkcert 是本地开发工具，**不**写入 go.mod，go install 装到 $(go env GOPATH)/bin。

set -euo pipefail

# 已安装则直接退出
if command -v mkcert >/dev/null 2>&1; then
  echo "✔ mkcert already installed: $(command -v mkcert)"
  mkcert -version
  exit 0
fi

# 没装：先看 go 是否在 PATH（go install 是兜底方案）
if ! command -v go >/dev/null 2>&1; then
  echo "✘ mkcert not found and go not in PATH" >&2
  echo "  install go first: https://go.dev/dl/" >&2
  exit 1
fi

GOBIN="$(go env GOPATH)/bin"
mkdir -p "$GOBIN"

# 优先尝试包管理器（速度快 + 浏览器信任一键搞定）
installed=false

if command -v brew >/dev/null 2>&1; then
  echo "→ installing mkcert via Homebrew (macOS / Linuxbrew) ..."
  brew install mkcert
  brew install nss 2>/dev/null || true  # Firefox 信任可选
  installed=true
elif command -v apt-get >/dev/null 2>&1; then
  echo "→ installing mkcert via apt (Debian / Ubuntu) ..."
  sudo apt-get update -y
  sudo apt-get install -y mkcert
  installed=true
elif command -v dnf >/dev/null 2>&1; then
  echo "→ installing mkcert via dnf (Fedora / RHEL) ..."
  sudo dnf install -y mkcert
  installed=true
elif command -v yum >/dev/null 2>&1; then
  echo "→ installing mkcert via yum (CentOS) ..."
  sudo yum install -y mkcert
  installed=true
elif command -v apk >/dev/null 2>&1; then
  echo "→ installing mkcert via apk (Alpine) ..."
  sudo apk add mkcert
  installed=true
fi

# 兜底：go install（最慢，但任何平台都能跑）
if [ "$installed" = false ]; then
  echo "→ no package manager found, falling back to go install ..."
  go install filippo.io/mkcert@latest
fi

# 校验
if ! command -v mkcert >/dev/null 2>&1 && [ ! -x "$GOBIN/mkcert" ]; then
  echo "✘ mkcert install failed" >&2
  exit 1
fi

MKCERT_BIN="$(command -v mkcert 2>/dev/null || echo "$GOBIN/mkcert")"
echo "✔ mkcert installed at $MKCERT_BIN"
"$MKCERT_BIN" -version
echo
echo "next: make certs   # 生成本地 HTTPS 证书到 configs/certs/"
