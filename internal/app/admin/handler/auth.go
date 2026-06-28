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

// Register 路由注册（公开路径：login / refresh）。
//
// 路由级限流 / 鉴权中间件由 app.go 统一挂载（全局先、路由后）。
func (a *Auth) Register(r gin.IRouter) {
	// 仅 login 挂路由级限流；其他路径（refresh）不受此中间件约束
	if a.limitMw != nil {
		r.POST("/api/auth/login", a.limitMw, a.login)
	} else {
		r.POST("/api/auth/login", a.login)
	}
	r.POST("/api/auth/login/mobile", a.loginByMobile) // 预留
	r.POST("/api/auth/refresh-token", a.refresh)
}

// RegisterLogout 注册受保护路径（需 JWT 中间件前置）。
//
// logout 必须带 access token 才知道要吊销哪个 jti，所以**单独**挂到
// 受保护路由组（app.go 里用 apiGroup + middleware.JWT），不与公开路径
// 的 Register 混在一起，避免「公开路径被 JWT 误拦」或「logout 被误开」。
func (a *Auth) RegisterLogout(protected gin.IRouter) {
	protected.POST("/api/auth/logout", a.logout)
}

// login POST /api/auth/login
func (a *Auth) login(c *gin.Context) {
	var req dto.LoginReq
	if err := a.v.Bind(c, &req); err != nil {
		response.WriteErr(c, err)
		return
	}
	pair, err := a.svc.Login(c.Request.Context(), dto.LoginInput{
		Username:  req.Username,
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

// logoutReq 登出请求参数（refresh 可选，传入则一并吊销）。
type logoutReq struct {
	Refresh string `json:"refresh" binding:"omitempty"`
}

// logout POST /api/auth/logout
//
// Phase 2：可选传 refresh token（与 access 同 pair 的那个），
// 后端会同时吊销 access + refresh 两个 jti，单 logout 立刻
// 废掉整对 token，防 refresh token 仍可换新 access。
//
// refresh 留空时只吊销 access（向后兼容老调用方）。
func (a *Auth) logout(c *gin.Context) {
	jti, _ := c.Get("jti")
	accessJTI, _ := jti.(string)
	var req logoutReq
	// 解析失败也不阻断（refresh 是 optional）
	_ = c.ShouldBindJSON(&req)
	if err := a.svc.Logout(c.Request.Context(), accessJTI, req.Refresh); err != nil {
		response.WriteErr(c, err)
		return
	}
	response.WriteOK(c, gin.H{"message": "logged out"})
}
