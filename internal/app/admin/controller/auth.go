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

// NewAuth 认证控制器构造函数。
func NewAuth(a *auth.Auth) *Auth { return &Auth{A: a} }

type loginReq struct {
	UserID string `json:"user_id" binding:"required"`
}

// Register 绑定路由。
func (a *Auth) Register(r gin.IRouter) {
	r.POST("/auth/login", a.login)
	r.POST("/auth/refresh", a.refresh)
	r.POST("/auth/logout", a.logout)
}

// @Summary 用户登录
// @Description 根据 user_id 签发 JWT token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body loginReq true "登录请求"
// @Success 200 {object} map[string]interface{} "access_token & refresh_token"
// @Failure 400 {object} errcode.Error "参数错误"
// @Failure 500 {object} errcode.Error "内部错误"
// @Router /auth/login [post]
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

// @Summary 刷新 Token
// @Description 使用 refresh_token 获取新的 access_token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body object{refresh=string} true "刷新请求"
// @Success 200 {object} map[string]interface{} "新的 access_token & refresh_token"
// @Failure 400 {object} errcode.Error "参数错误"
// @Failure 401 {object} errcode.Error "Token无效"
// @Router /auth/refresh [post]
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

// @Summary 用户登出
// @Description 将 token 加入黑名单
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string "message"
// @Failure 401 {object} errcode.Error "未授权"
// @Router /auth/logout [post]
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
