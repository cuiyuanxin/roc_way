// Package auth: enforcer 提供基于 Casbin 的 RBAC 权限校验。
package auth

import (
	"fmt"

	"github.com/casbin/casbin/v2"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
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
func (e *Enforcer) RequirePermission(obj, act string) gin.HandlerFunc {
	return func(c *gin.Context) {
		sub, _ := c.Get("user_id")
		s, _ := sub.(string)
		if s == "" {
			s = "anonymous"
		}
		ok, err := e.Enforce(s, obj, act)
		if err != nil {
			c.AbortWithStatusJSON(errcode.ErrInternal.HTTPStatus,
				gin.H{"code": errcode.ErrInternal.Code, "message": err.Error()})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(errcode.ErrForbidden.HTTPStatus,
				gin.H{"code": errcode.ErrForbidden.Code, "message": errcode.ErrForbidden.Message})
			return
		}
		c.Next()
	}
}
