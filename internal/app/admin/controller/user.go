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

func NewUser(db *database.DB) *User { return &User{DB: db} }

type createUserReq struct {
	Name  string `json:"name" binding:"required,min=2,max=64"`
	Email string `json:"email" binding:"required,email"`
}

func (u *User) Register(r gin.IRouter) {
	r.POST("/api/v1/users", u.create)
	r.GET("/api/v1/users", u.list)
	r.GET("/api/v1/users/:id", u.detail)
}

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

func (u *User) list(c *gin.Context) {
	var users []model.User
	if err := u.DB.RO().Find(&users).Error; err != nil {
		WriteErr(c, errcode.New(errcode.ErrDatabase, err))
		return
	}
	WriteOK(c, users)
}

func (u *User) detail(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var user model.User
	if err := u.DB.RO().First(&user, id).Error; err != nil {
		WriteErr(c, errcode.New(errcode.ErrUserNotFound, err))
		return
	}
	WriteOK(c, user)
}
