package controller

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/auth"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// Auth 演示 JWT 签发 / 刷新 / 登出。
type Auth struct {
	A *auth.Auth
}

func NewAuth(a *auth.Auth) *Auth { return &Auth{A: a} }

type loginReq struct {
	UserID string `json:"user_id" binding:"required"`
}

func (a *Auth) Register(r gin.IRouter) {
	r.POST("/auth/login", a.login)
	r.POST("/auth/refresh", a.refresh)
	r.POST("/auth/logout", a.logout)
}

func (a *Auth) login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c, errcode.New(errcode.ErrInvalidParam, err))
		return
	}
	pair, err := a.A.Issue(req.UserID)
	if err != nil {
		WriteErr(c, errcode.New(errcode.ErrInternal, err))
		return
	}
	WriteOK(c, pair)
}

func (a *Auth) refresh(c *gin.Context) {
	var req struct {
		Refresh string `json:"refresh" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c, errcode.New(errcode.ErrInvalidParam, err))
		return
	}
	claims, err := a.A.Parse(req.Refresh)
	if err != nil || claims.Kind != "refresh" {
		WriteErr(c, errcode.New(errcode.ErrTokenInvalid, err))
		return
	}
	pair, err := a.A.Issue(claims.Subject)
	if err != nil {
		WriteErr(c, errcode.New(errcode.ErrInternal, err))
		return
	}
	WriteOK(c, pair)
}

func (a *Auth) logout(c *gin.Context) {
	jti, _ := c.Get("jti")
	s, _ := jti.(string)
	if s == "" {
		WriteErr(c, errcode.New(errcode.ErrUnauthorized, nil))
		return
	}
	_ = a.A.Revoke(c.Request.Context(), s, 24*time.Hour)
	WriteOK(c, gin.H{"message": "logged out"})
}
