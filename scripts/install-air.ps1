# scripts/install-air.ps1
#
# 一键安装 air（Go 热启动工具）到 $(go env GOPATH)\bin。
# **仅限 Windows 平台**。Linux/macOS 用户：
#   - bash scripts/install-air.sh
#   - 或: make install-air（跨平台通用入口）
#
# 用法（PowerShell）：
#   powershell -ExecutionPolicy Bypass -File scripts\install-air.ps1
# 或在仓库根目录：
#   .\scripts\install-air.ps1

$ErrorActionPreference = "Stop"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "✘ go not found in PATH" -ForegroundColor Red
    exit 1
}

$GOBIN = (go env GOPATH) + "\bin"
if (-not (Test-Path $GOBIN)) {
    New-Item -ItemType Directory -Path $GOBIN | Out-Null
}

$airExe = Join-Path $GOBIN "air.exe"
if (Test-Path $airExe) {
    Write-Host "✔ air already installed at $airExe" -ForegroundColor Green
    & $airExe -v
    exit 0
}

Write-Host "→ installing github.com/air-verse/air@latest ..."
go install github.com/air-verse/air@latest

if (-not (Test-Path $airExe)) {
    Write-Host "✘ install failed: $airExe not found" -ForegroundColor Red
    exit 1
}

Write-Host "✔ air installed at $airExe" -ForegroundColor Green
& $airExe -v
Write-Host ""
Write-Host "next: make dev   # 启动热重载开发模式"
