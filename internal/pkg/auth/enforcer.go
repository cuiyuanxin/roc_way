// Package auth: enforcer 提供基于 Casbin 的 RBAC 权限校验。
package auth

import (
	"fmt"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
	"github.com/cuiyuanxin/roc_way/internal/pkg/response"
)

// Enforcer 包装 casbin.Enforcer。
type Enforcer struct {
	e *casbin.Enforcer
}

// NewEnforcer 从 modelPath/policyPath 加载 RBAC 策略。
func NewEnforcer(modelPath, policyPath string) (*Enforcer, error) {
	e, err := casbin.NewEnforcer(modelPath, policyPath)
	if err != nil {
		return nil, fmt.Errorf("auth: enforcer: %w", err)
	}
	if err := e.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("auth: load policy: %w", err)
	}
	return &Enforcer{e: e}, nil
}

// Enforce 内部调用 Enforce。
func (e *Enforcer) Enforce(sub, obj, act string) (bool, error) {
	return e.e.Enforce(sub, obj, act)
}

// LoadPolicy 重新加载策略（热更新场景）。
func (e *Enforcer) LoadPolicy() error { return e.e.LoadPolicy() }

// RequirePermission gin 中间件：从 context 取 sub（JWT 注入），调用 Enforce。
// 失败返回 errcode.ErrForbidden。
//
// 注意：本函数未直接 import middleware 包以避免循环依赖（middleware → auth），
// request_id 由 internal/pkg/response.WriteErr 内部读取 gin.Context 的 "request_id" key。
// key 必须与 internal/pkg/middleware.DefaultRequestIDContextKey 保持一致。
func (e *Enforcer) RequirePermission(obj, act string) gin.HandlerFunc {
	return func(c *gin.Context) {
		sub, _ := c.Get("user_id")
		s, _ := sub.(string)
		if s == "" {
			s = "anonymous"
		}
		ok, err := e.Enforce(s, obj, act)
		if err != nil {
			response.WriteErr(c, err)
			c.Abort()
			return
		}
		if !ok {
			response.WriteErr(c, errcode.ErrForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}
