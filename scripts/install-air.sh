#!/usr/bin/env bash
# scripts/install-air.sh
#
# 一键安装 air（Go 热启动工具）到 $(go env GOPATH)/bin。
# **仅限 Linux / macOS 平台**。Windows 用户：
#   - PowerShell: powershell -ExecutionPolicy Bypass -File scripts\install-air.ps1
#   - 或: make install-air（跨平台通用入口）
#
# 用法：bash scripts/install-air.sh

set -euo pipefail

if ! command -v go >/dev/null 2>&1; then
  echo "✘ go not found in PATH" >&2
  exit 1
fi

GOBIN="$(go env GOPATH)/bin"
mkdir -p "$GOBIN"

if [ -x "$GOBIN/air" ]; then
  echo "✔ air already installed at $GOBIN/air"
  "$GOBIN/air" -v || true
  exit 0
fi

echo "→ installing github.com/air-verse/air@latest ..."
go install github.com/air-verse/air@latest

if [ ! -x "$GOBIN/air" ]; then
  echo "✘ install failed: $GOBIN/air not found" >&2
  exit 1
fi

echo "✔ air installed at $GOBIN/air"
"$GOBIN/air" -v || true
echo
echo "next: make dev   # 启动热重载开发模式"
