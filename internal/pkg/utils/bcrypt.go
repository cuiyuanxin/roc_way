// Package utils 提供项目内部共享工具函数。
//
// 位置说明：放在 internal/pkg/ 而非 pkg/，因为：
//   - 本工具仅被项目内部（admin service / e2e 测试）使用，**禁止**被外部项目依赖
//   - 规则第 2 条：「不允许外部项目 import → internal/pkg/<lib_name>/」
package utils

import "golang.org/x/crypto/bcrypt"

// Hash 把明文密码哈希。
func Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Verify 比对明文密码与 bcrypt 哈希。
func Verify(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
