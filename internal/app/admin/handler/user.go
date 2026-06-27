// Package handler 是表现层：解析 HTTP 入参、调用应用服务、返回响应
//
// 约束（DDD）：
//   - 不直接接触领域对象
//   - 不直接接触 GORM
//   - **禁止**写业务逻辑
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/dto"
	"github.com/cuiyuanxin/roc_way/internal/app/admin/service"
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
	r.POST("/api/v1/users", u.register)
	r.GET("/api/v1/users", u.list)
	r.GET("/api/v1/users/:id", u.detail)
	r.PATCH("/api/v1/users/:id", u.updateName)
	r.DELETE("/api/v1/users/:id", u.delete)
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
