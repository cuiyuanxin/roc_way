<#
.SYNOPSIS
    Generate RS256 RSA key pair for Phase 2 JWT upgrade.

.DESCRIPTION
    Generates 2048-bit RSA private/public key (PEM format) under configs/keys/:
      - jwt_private.pem  Only the signing service holds it (loaded by auth.New).
                          Permission auto-set to current-user-only on Windows.
      - jwt_public.pem   Public key; can be shared with frontend / other services
                          for local signature verification.

    Companion notes:
      - configs/keys/ is in .gitignore; private key will NOT be committed.
      - config.yaml points private_key_path / public_key_path to these files.
      - Existing files are backed up to .bak before regeneration.

.PARAMETER Bits
    RSA key length in bits. Default 2048. OWASP minimum 2048; 4096 recommended.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts\gen-jwt-keys.ps1
    powershell -ExecutionPolicy Bypass -File scripts\gen-jwt-keys.ps1 -Bits 4096
#>
[CmdletBinding()]
param(
    [int]$Bits = 2048
)

$ErrorActionPreference = "Stop"

# Switch to project root so relative paths resolve from there
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Resolve-Path (Join-Path $ScriptDir "..")
Set-Location $ProjectRoot

$KeyDir   = Join-Path $ProjectRoot "configs\keys"
$PrivPath = Join-Path $KeyDir "jwt_private.pem"
$PubPath  = Join-Path $KeyDir "jwt_public.pem"

if (-not (Test-Path $KeyDir)) {
    New-Item -ItemType Directory -Force -Path $KeyDir | Out-Null
    Write-Host "created: $KeyDir" -ForegroundColor Green
}

# Backup existing files
if (Test-Path $PrivPath) {
    $Bak = "$PrivPath.bak"
    Move-Item -Force $PrivPath $Bak
    Write-Host "backup: $PrivPath -> $Bak" -ForegroundColor Yellow
}
if (Test-Path $PubPath) {
    $Bak = "$PubPath.bak"
    Move-Item -Force $PubPath $Bak
    Write-Host "backup: $PubPath -> $Bak" -ForegroundColor Yellow
}

# Probe openssl
$Openssl = (Get-Command openssl -ErrorAction SilentlyContinue)
if (-not $Openssl) {
    Write-Host "ERROR: openssl not found in PATH. Install OpenSSL (or use WSL / Git Bash) and retry." -ForegroundColor Red
    exit 1
}

Write-Host "generating RSA $Bits bits..." -ForegroundColor Cyan
& openssl genpkey -algorithm RSA -pkeyopt "rsa_keygen_bits:$Bits" -out $PrivPath
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: openssl genpkey failed" -ForegroundColor Red
    exit 1
}

& openssl rsa -in $PrivPath -pubout -out $PubPath
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: openssl rsa -pubout failed" -ForegroundColor Red
    exit 1
}

# Tighten private key permission on Windows: disable inheritance, current user R only.
try {
    icacls $PrivPath /inheritance:r /grant:r "$($env:USERNAME):(R)" | Out-Null
    Write-Host "private key permission: $env:USERNAME:R (inheritance disabled)" -ForegroundColor Green
} catch {
    Write-Host "WARN: icacls failed (not critical): $_" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== generated ===" -ForegroundColor Green
Write-Host "  private: $PrivPath" -ForegroundColor Green
Write-Host "  public : $PubPath" -ForegroundColor Green
Write-Host ""
Write-Host "next:" -ForegroundColor Cyan
Write-Host "  - confirm config.yaml private_key_path / public_key_path match above"
Write-Host "  - private key MUST stay on signing service only; public key is shareable"
Write-Host "  - if access/refresh tokens were issued under old HS256 secret, users must re-login"
