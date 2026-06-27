<#
.SYNOPSIS
    Generate HS256 JWT secret (Phase 2.5: downgrade RS256 -> HS256).

.DESCRIPTION
    Generates a 48-byte (384-bit) random secret, base64-encoded, and writes it to
    configs/.jwt_secret (mode 0600 / current user R only).

    Why HS256 not RS256:
      - Single-service backend scaffolds (like this project) don't need PEM
        private/public key separation
      - HS256 is the industry default (gin-jwt / go-admin / go-zero all use it)
      - A single strong secret (>= 32 bytes) is just as secure for our use case
      - Code: ~150 lines (RS256) -> ~50 lines (HS256)

    Smart loading in [auth.New] handles three sources in priority order:
      1) env JWT_SECRET (production)
      2) config: auth.jwt_secret
      3) file: configs/.jwt_secret (dev only, auto-generated)

    Existing file is backed up to .bak before regeneration.

.PARAMETER Bits
    Random byte length. Default 48 (384-bit). Minimum 32 (256-bit, OWASP).

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\gen-jwt-secret.ps1
    powershell -ExecutionPolicy Bypass -File scripts\gen-jwt-secret.ps1 -Bits 64
#>
[CmdletBinding()]
param(
    [int]$Bytes = 48
)

$ErrorActionPreference = "Stop"

# Switch to project root
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Resolve-Path (Join-Path $ScriptDir "..")
Set-Location $ProjectRoot

if ($Bytes -lt 32) {
    Write-Host "ERROR: secret length must be >= 32 bytes (OWASP HS256 minimum 256-bit)" -ForegroundColor Red
    exit 1
}

$SecretPath = Join-Path $ProjectRoot "configs\.jwt_secret"
$KeyDir     = Split-Path $SecretPath -Parent

if (-not (Test-Path $KeyDir)) {
    New-Item -ItemType Directory -Force -Path $KeyDir | Out-Null
    Write-Host "created: $KeyDir" -ForegroundColor Green
}

# Backup existing
if (Test-Path $SecretPath) {
    $Bak = "$SecretPath.bak"
    Move-Item -Force $SecretPath $Bak
    Write-Host "backup: $SecretPath -> $Bak" -ForegroundColor Yellow
}

# Generate random bytes (use .NET RNGCryptoServiceProvider)
$Provider = New-Object System.Security.Cryptography.RNGCryptoServiceProvider
$Buf = New-Object byte[] $Bytes
$Provider.GetBytes($Buf)
$Secret = [Convert]::ToBase64String($Buf)
$Provider.Dispose()

# Write file (mode 600; on Windows ACL=current user R only)
Set-Content -Path $SecretPath -Value $Secret -NoNewline
try {
    icacls $SecretPath /inheritance:r /grant:r "$($env:USERNAME):(R)" | Out-Null
    Write-Host "secret permission: $env:USERNAME:R (inheritance disabled)" -ForegroundColor Green
} catch {
    Write-Host "WARN: icacls failed (not critical): $_" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== generated ===" -ForegroundColor Green
Write-Host "  path:   $SecretPath" -ForegroundColor Green
Write-Host "  bytes:  $Bytes" -ForegroundColor Green
Write-Host "  length: $($Secret.Length) chars (base64)" -ForegroundColor Green
Write-Host ""
Write-Host "next:" -ForegroundColor Cyan
Write-Host "  - for production: set env JWT_SECRET=... and config.yaml production_mode=true"
Write-Host "  - for dev: leave config.yaml production_mode=false; this file is auto-loaded"
Write-Host "  - secret will NOT be printed; do not paste this value in chat / git"
