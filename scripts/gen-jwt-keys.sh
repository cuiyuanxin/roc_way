#!/usr/bin/env bash
# 生成 RS256 RSA 密钥对（Phase 2 JWT 升级配套脚本）。
#
# 在 configs/keys/ 目录下生成 RSA 私钥与公钥（PEM 格式）：
#   - jwt_private.pem  仅签发服务持有（auth.New 内部加载），权限自动设为 600
#   - jwt_public.pem   公钥可发给前端 / 其它服务本地验签
#
# 配套：
#   - configs/keys/ 已被 .gitignore 屏蔽，私钥不会入库
#   - config.yaml 里 private_key_path / public_key_path 默认指向这两个文件
#   - 已有同名文件会被备份为 jwt_private.pem.bak / jwt_public.pem.bak
#
# 用法：
#   bash scripts/gen-jwt-keys.sh
#   bash scripts/gen-jwt-keys.sh 4096

set -euo pipefail

BITS="${1:-2048}"

# 切到项目根
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

KEY_DIR="$PROJECT_ROOT/configs/keys"
PRIV_PATH="$KEY_DIR/jwt_private.pem"
PUB_PATH="$KEY_DIR/jwt_public.pem"

if [[ ! -d "$KEY_DIR" ]]; then
    mkdir -p "$KEY_DIR"
    echo "created: $KEY_DIR"
fi

# 备份旧文件
if [[ -f "$PRIV_PATH" ]]; then
    mv -f "$PRIV_PATH" "$PRIV_PATH.bak"
    echo "backup: $PRIV_PATH -> $PRIV_PATH.bak"
fi
if [[ -f "$PUB_PATH" ]]; then
    mv -f "$PUB_PATH" "$PUB_PATH.bak"
    echo "backup: $PUB_PATH -> $PUB_PATH.bak"
fi

# 探测 openssl
if ! command -v openssl >/dev/null 2>&1; then
    echo "ERROR: openssl not found in PATH. Install OpenSSL and retry." >&2
    exit 1
fi

echo "generating RSA $BITS bits..."
openssl genpkey -algorithm RSA -pkeyopt "rsa_keygen_bits:$BITS" -out "$PRIV_PATH"
openssl rsa -in "$PRIV_PATH" -pubout -out "$PUB_PATH"

# 私钥收紧权限（owner rw-，group/other ---）
chmod 600 "$PRIV_PATH"
chmod 644 "$PUB_PATH"

echo ""
echo "=== generated ==="
echo "  private: $PRIV_PATH  (mode 600)"
echo "  public : $PUB_PATH   (mode 644)"
echo ""
echo "next:"
echo "  - 确认 config.yaml 的 private_key_path / public_key_path 与上述路径一致"
echo "  - 私钥**只**在签发服务保留；公钥可发给前端 / 其它服务验签"
echo "  - 若已签发的 access/refresh token 用旧 HS256 secret 生成，需用户重新登录"
