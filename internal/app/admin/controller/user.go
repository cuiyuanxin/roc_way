package controller

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/app/admin/model"
	"github.com/cuiyuanxin/roc_way/internal/pkg/database"
	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// User 演示 CRUD，使用 RBAC 中间件保护接口。
type User struct {
	DB *database.DB
}

// NewUser 用户控制器构造函数。
func NewUser(db *database.DB) *User { return &User{DB: db} }

type createUserReq struct {
	Name  string `json:"name" binding:"required,min=2,max=64"`
	Email string `json:"email" binding:"required,email"`
}

// Register 绑定路由。
func (u *User) Register(r gin.IRouter) {
	r.POST("/api/v1/users", u.create)
	r.GET("/api/v1/users", u.list)
	r.GET("/api/v1/users/:id", u.detail)
}

// @Summary 创建用户
// @Description 创建新用户
// @Tags Users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body createUserReq true "创建用户请求"
// @Success 200 {object} model.User
// @Failure 400 {object} errcode.Error "参数错误"
// @Failure 500 {object} errcode.Error "数据库错误"
// @Router /api/v1/users [post]
func (u *User) create(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteErr(c, errcode.New(errcode.ErrInvalidParam, err))
		return
	}
	user := model.User{Name: req.Name, Email: req.Email}
	if err := u.DB.Write.Create(&user).Error; err != nil {
		WriteErr(c, errcode.New(errcode.ErrDatabase, err))
		return
	}
	WriteOK(c, user)
}

// @Summary 用户列表
// @Description 获取所有用户
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {array} model.User
// @Failure 500 {object} errcode.Error "数据库错误"
// @Router /api/v1/users [get]
func (u *User) list(c *gin.Context) {
	var users []model.User
	if err := u.DB.RO().Find(&users).Error; err != nil {
		WriteErr(c, errcode.New(errcode.ErrDatabase, err))
		return
	}
	WriteOK(c, users)
}

// @Summary 用户详情
// @Description 根据 ID 获取用户详情
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Param id path int true "用户ID"
// @Success 200 {object} model.User
// @Failure 400 {object} errcode.Error "参数错误"
// @Failure 404 {object} errcode.Error "用户不存在"
// @Router /api/v1/users/{id} [get]
func (u *User) detail(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var user model.User
	if err := u.DB.RO().First(&user, id).Error; err != nil {
		WriteErr(c, errcode.New(errcode.ErrUserNotFound, err))
		return
	}
	WriteOK(c, user)
}
