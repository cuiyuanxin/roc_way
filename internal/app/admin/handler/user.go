// Package handler 是表现层：解析 HTTP 入参、调用应用服务、返回响应
//
// 约束（DDD）：
//   - 不直接接触领域对象
//   - 不直接接触 GORM
//   - **禁止**写业务逻辑
//   - 透传 service 返回的 dto（service 负责 model→dto 映射，handler 不再做）
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/dto"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/service"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
	"github.com/cuiyuanxin/roc_way/internal/pkg/response"
	"github.com/cuiyuanxin/roc_way/internal/pkg/validator"
)

// User 用户管理 HTTP 表现层。
type User struct {
	svc *service.UserService
	v   *validator.Validator
}

// NewUser 构造 User controller。
func NewUser(svc *service.UserService, v *validator.Validator) *User { return &User{svc: svc, v: v} }

// Register 路由注册。
func (u *User) Register(r gin.IRouter) {
	r.GET("/api/v1/users/info", u.info)
	r.POST("/api/v1/users", u.register)
	r.GET("/api/v1/users", u.list)
	r.GET("/api/v1/users/:id", u.detail)
	r.PATCH("/api/v1/users/:id", u.updateName)
	r.DELETE("/api/v1/users/:id", u.delete)
}

// info GET /api/v1/users/info
//
// 当前登录用户查「自己的」个人信息：
//   - 从 JWT 中间件写入 gin.Context 的 "user_id" 取出 uid
//   - 调 UserService.GetByID 走 GORM 查 users 表
//   - service 直返 *dto.UserInfo，handler 透传（不二次映射）
//
// 安全约束（应用 project_rules.md 第 0.1.2 条「输入校验」+ 第 0.1.4 条「越权防护」）：
//   - 路径是 /info 而不是 /:id，**禁止**接受 URL 里的 id 参数，只能用 token 里的 uid，
//     避免「拿别人 token 改 URL」越权查询。
//   - 路由必须在受 JWT 保护的路由组（app.go apiGroup）下挂载，否则 user_id 拿不到。
//   - token 过期 / 伪造 / 设备不匹配：由 JWT 中间件前置拦截，本 handler 不重复校验。
//   - 用户被删（token 仍有效）：GetByID 返回 nil → 404 ErrUserNotFound，行为正确。
func (u *User) info(c *gin.Context) {
	raw, ok := c.Get("user_id")
	if !ok || raw == nil {
		response.WriteErr(c, errcode.ErrUnauthorized.WithMessage("未登录或 token 已失效"))
		return
	}
	uidStr, _ := raw.(string)
	uid, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil || uid == 0 {
		response.WriteErr(c, errcode.ErrUnauthorized.WithMessage("token 中的用户 ID 不合法"))
		return
	}
	user, err := u.svc.GetByID(c.Request.Context(), uint(uid))
	if err != nil {
		response.WriteErr(c, err)
		return
	}
	if user == nil {
		response.WriteErr(c, errcode.ErrUserNotFound.WithMessage("用户不存在或已被删除"))
		return
	}
	response.WriteOK(c, user)
}

// register POST /api/v1/users
func (u *User) register(c *gin.Context) {
	var req dto.RegisterReq
	if err := u.v.Bind(c, &req); err != nil {
		response.WriteErr(c, err)
		return
	}
	user, err := u.svc.Register(c.Request.Context(), dto.RegisterInput{
		Email:    req.Email,
		Name:     req.Name,
		Password: req.Password,
	})
	if err != nil {
		response.WriteErr(c, err)
		return
	}
	response.WriteOK(c, user)
}

// list GET /api/v1/users?page=1&page_size=20
func (u *User) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	size, _ := strconv.Atoi(c.Query("page_size"))
	users, total, err := u.svc.List(c.Request.Context(), dto.ListInput{
		Page:     page,
		PageSize: size,
	})
	if err != nil {
		response.WriteErr(c, err)
		return
	}
	response.WriteOK(c, gin.H{
		"list":      users,
		"total":     total,
		"page":      page,
		"page_size": size,
	})
}

// detail GET /api/v1/users/:id
func (u *User) detail(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	user, err := u.svc.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.WriteErr(c, err)
		return
	}
	if user == nil {
		response.WriteErr(c, errcode.ErrUserNotFound.WithMessage("用户不存在"))
		return
	}
	response.WriteOK(c, user)
}

type updateNameReq struct {
	Name string `json:"name" binding:"required,min=2,max=64"`
}

// updateName PATCH /api/v1/users/:id
func (u *User) updateName(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req updateNameReq
	if err := u.v.Bind(c, &req); err != nil {
		response.WriteErr(c, err)
		return
	}
	user, err := u.svc.UpdateName(c.Request.Context(), dto.UpdateNameInput{
		ID:   uint(id),
		Name: req.Name,
	})
	if err != nil {
		response.WriteErr(c, err)
		return
	}
	response.WriteOK(c, user)
}

// delete DELETE /api/v1/users/:id
func (u *User) delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := u.svc.Delete(c.Request.Context(), uint(id)); err != nil {
		response.WriteErr(c, err)
		return
	}
	response.WriteOK(c, gin.H{"message": "deleted"})
}
