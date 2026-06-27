# scripts/install-mkcert.ps1
#
# 一键安装 mkcert（本地可信 HTTPS 证书生成工具）。
# **仅限 Windows 平台**。Linux/macOS 用户：
#   - bash scripts/install-mkcert.sh
#   - 或: make certs（跨平台通用入口）
#
# 用法（PowerShell）：
#   powershell -ExecutionPolicy Bypass -File scripts\install-mkcert.ps1
# 或在仓库根目录：
#   .\scripts\install-mkcert.ps1
#
# mkcert 项目主页：https://github.com/FiloSottile/mkcert
# mkcert 是本地开发工具，**不**写入 go.mod，go install 装到 $(go env GOPATH)\bin。

$ErrorActionPreference = "Stop"

# 已安装则直接退出
$existing = Get-Command mkcert -ErrorAction SilentlyContinue
if ($existing) {
    Write-Host "✔ mkcert already installed: $($existing.Source)" -ForegroundColor Green
    & mkcert -version
    exit 0
}

# 没装：先看 go 是否在 PATH（go install 是兜底方案）
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "✘ mkcert not found and go not in PATH" -ForegroundColor Red
    Write-Host "  install go first: https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

$GOBIN = (go env GOPATH) + "\bin"
if (-not (Test-Path $GOBIN)) {
    New-Item -ItemType Directory -Path $GOBIN | Out-Null
}

# 优先尝试 Windows 包管理器（choco / scoop / winget）
$installed = $false
if (Get-Command choco -ErrorAction SilentlyContinue) {
    Write-Host "→ installing mkcert via Chocolatey ..." -ForegroundColor Cyan
    choco install mkcert -y
    $installed = $true
}
elseif (Get-Command scoop -ErrorAction SilentlyContinue) {
    Write-Host "→ installing mkcert via Scoop ..." -ForegroundColor Cyan
    scoop install mkcert
    $installed = $true
}
elseif (Get-Command winget -ErrorAction SilentlyContinue) {
    Write-Host "→ installing mkcert via winget ..." -ForegroundColor Cyan
    winget install --id FiloSottile.mkcert -e --accept-source-agreements --accept-package-agreements
    $installed = $true
}

# 兜底：go install（最慢，但任何平台都能跑）
if (-not $installed) {
    Write-Host "→ no package manager found, falling back to go install ..." -ForegroundColor Cyan
    go install filippo.io/mkcert@latest
}

# 刷新 PATH 缓存（go install 路径可能没在当前会话 PATH 里）
$env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
            [System.Environment]::GetEnvironmentVariable("Path", "User")

# 校验
$mkcertBin = Get-Command mkcert -ErrorAction SilentlyContinue
if (-not $mkcertBin) {
    $mkcertExe = Join-Path $GOBIN "mkcert.exe"
    if (Test-Path $mkcertExe) {
        $mkcertBin = $mkcertExe
    }
}
if (-not $mkcertBin) {
    Write-Host "✘ mkcert install failed" -ForegroundColor Red
    exit 1
}

Write-Host "✔ mkcert installed at $mkcertBin" -ForegroundColor Green
& $mkcertBin -version
Write-Host ""
Write-Host "next: make certs   # 生成本地 HTTPS 证书到 configs/certs/" -ForegroundColor Yellow
