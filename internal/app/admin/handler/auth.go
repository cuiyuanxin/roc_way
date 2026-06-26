package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/dto"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/service"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
	"github.com/cuiyuanxin/roc_way/internal/pkg/response"
	"github.com/cuiyuanxin/roc_way/internal/pkg/validator"
)

// Auth 认证 HTTP 表现层。
type Auth struct {
	svc     *service.AuthService
	v       *validator.Validator
	limitMw gin.HandlerFunc // 路由级限流（仅 login 路径生效）
}

// NewAuth 构造 Auth controller。
//
// limitMw 为路由级限流中间件（仅作用于 /api/auth/login）；
// nil 或未传入表示不限流。
func NewAuth(svc *service.AuthService, v *validator.Validator, limitMw ...gin.HandlerFunc) *Auth {
	var mw gin.HandlerFunc
	if len(limitMw) > 0 {
		mw = limitMw[0]
	}
	return &Auth{svc: svc, v: v, limitMw: mw}
}

// Register 路由注册。
//
// 路由级限流 / 鉴权中间件由 app.go 统一挂载（全局先、路由后）。
func (a *Auth) Register(r gin.IRouter) {
	// 仅 login 挂路由级限流；其他路径（refresh / logout）不受此中间件约束
	if a.limitMw != nil {
		r.POST("/api/auth/login", a.limitMw, a.login)
	} else {
		r.POST("/api/auth/login", a.login)
	}
	r.POST("/api/auth/login/mobile", a.loginByMobile) // 预留
	r.POST("/api/auth/refresh", a.refresh)
	r.POST("/api/auth/logout", a.logout)
}

// login POST /api/auth/login
func (a *Auth) login(c *gin.Context) {
	var req dto.LoginReq
	if err := a.v.Bind(c, &req); err != nil {
		response.WriteErr(c, err)
		return
	}
	pair, err := a.svc.Login(c.Request.Context(), dto.LoginInput{
		Username:  req.UserName,
		Password:  req.Password,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})
	if err != nil {
		response.WriteErr(c, err)
		return
	}
	response.WriteOK(c, pair)
}

// loginByMobileReq 手机号登录请求参数（预留）。
type loginByMobileReq struct {
	Mobile   string `json:"mobile"   binding:"required,mobile"`
	Password string `json:"password" binding:"required,min=1,max=64"`
}

// loginByMobile POST /api/auth/login/mobile
//
// **预留接口**：当前返回 501，告知前端「手机号 + 验证码登录尚未实现」；
// 未来接短信验证码 + 用户体系时只需填本方法实现，路由 / dto 已就位。
func (a *Auth) loginByMobile(c *gin.Context) {
	var req loginByMobileReq
	if err := a.v.Bind(c, &req); err != nil {
		response.WriteErr(c, err)
		return
	}
	// 预留：返回 501 + ErrNotImplemented
	response.WriteErr(c, errcode.ErrNotImplemented)
}

type refreshReq struct {
	Refresh string `json:"refresh" binding:"required"`
}

// refresh POST /api/auth/refresh
//
// 修复 [C4]：原实现仅回显入参，任何字符串可换 token。
// 现真校验 refresh token（验签 + Kind 断言 + 黑名单 + rotation）。
func (a *Auth) refresh(c *gin.Context) {
	var req refreshReq
	if err := a.v.Bind(c, &req); err != nil {
		response.WriteErr(c, err)
		return
	}
	pair, err := a.svc.Refresh(c.Request.Context(), req.Refresh)
	if err != nil {
		response.WriteErr(c, err)
		return
	}
	response.WriteOK(c, pair)
}

// logout POST /api/auth/logout
func (a *Auth) logout(c *gin.Context) {
	jti, _ := c.Get("jti")
	s, _ := jti.(string)
	if err := a.svc.Logout(c.Request.Context(), s); err != nil {
		response.WriteErr(c, err)
		return
	}
	response.WriteOK(c, gin.H{"message": "logged out"})
}
