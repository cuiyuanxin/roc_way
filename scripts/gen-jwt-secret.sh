#!/usr/bin/env bash
# Generate HS256 JWT secret (Phase 2.5: downgrade RS256 -> HS256).
#
# Generates 48-byte (384-bit) random secret, base64-encoded, and writes it to
# configs/.jwt_secret (chmod 600).
#
# Why HS256 not RS256:
#   - Single-service backend scaffolds don't need PEM private/public key separation
#   - HS256 is the industry default (gin-jwt / go-admin / go-zero all use it)
#   - A single strong secret (>= 32 bytes) is just as secure for our use case
#   - Code: ~150 lines (RS256) -> ~50 lines (HS256)
#
# Smart loading in [auth.New] handles three sources in priority order:
#   1) env JWT_SECRET (production)
#   2) config: auth.jwt_secret
#   3) file: configs/.jwt_secret (dev only, auto-generated)
#
# Existing file is backed up to .bak before regeneration.
#
# Usage:
#   bash scripts/gen-jwt-secret.sh
#   bash scripts/gen-jwt-secret.sh 64    # 64 bytes = 512-bit secret

set -euo pipefail

BYTES="${1:-48}"

if [[ "$BYTES" -lt 32 ]]; then
    echo "ERROR: secret length must be >= 32 bytes (OWASP HS256 minimum 256-bit)" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

SECRET_PATH="$PROJECT_ROOT/configs/.jwt_secret"
KEY_DIR="$(dirname "$SECRET_PATH")"

mkdir -p "$KEY_DIR"

# Backup existing
if [[ -f "$SECRET_PATH" ]]; then
    mv -f "$SECRET_PATH" "$SECRET_PATH.bak"
    echo "backup: $SECRET_PATH -> $SECRET_PATH.bak"
fi

# Generate secret using openssl (portable: Linux/macOS/Win-Git-Bash)
if ! command -v openssl >/dev/null 2>&1; then
    echo "ERROR: openssl not found in PATH. Install OpenSSL and retry." >&2
    exit 1
fi

SECRET="$(openssl rand -base64 "$BYTES")"
echo -n "$SECRET" > "$SECRET_PATH"
chmod 600 "$SECRET_PATH"

echo ""
echo "=== generated ==="
echo "  path:   $SECRET_PATH  (mode 600)"
echo "  bytes:  $BYTES"
echo "  length: ${#SECRET} chars (base64)"
echo ""
echo "next:"
echo "  - for production: set env JWT_SECRET=... and config.yaml production_mode=true"
echo "  - for dev: leave config.yaml production_mode=false; this file is auto-loaded"
echo "  - secret will NOT be printed; do not paste this value in chat / git"
