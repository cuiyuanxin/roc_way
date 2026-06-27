// Package domain 是 admin 应用的领域层。
//
// 该包是 DDD 的核心：
//   - 定义聚合根 User（含行为，不只是数据）
//   - 定义领域错误（由 service 层翻译为业务错误码）
//   - 不依赖任何基础设施（DB / GORM / Gin / bcrypt）
//
// 持久化抽象在 repository 包（接口 + GORM 实现同包）。
package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// 领域错误：定义在领域层，由 application / controller 层翻译为 HTTP 响应。
var (
	ErrInvalidName     = errors.New("user: invalid name")
	ErrInvalidPassword = errors.New("user: password must be at least 6 chars")
	ErrNotFound        = errors.New("user: not found")
	ErrUsernameTaken   = errors.New("user: username already taken")
	ErrEmailTaken      = errors.New("user: email already taken")
)

// User 领域用户聚合根。
type User struct {
	ID        uint
	Username  string
	Email     string
	Name      string
	Password  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// usernameRe 用户名校验正则：5-24 位字母数字下划线短横线。
var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{5,24}$`)

// emailRe 邮箱校验正则。
var emailRe = regexp.MustCompile(`^[\w._%+-]+@[\w.-]+\.[A-Za-z]{2,}$`)

// NewUser 构造 User 并校验关键字段。
//
// username 必填且满足 5-24 位字母数字下划线短横线；email 可选但填了就必须合法；
// name 必填且非空（去除前后空格后）；passwordHash 必填。
func NewUser(id uint, username, email, name, passwordHash string, now time.Time) (*User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	name = strings.TrimSpace(name)
	if !usernameRe.MatchString(username) {
		return nil, errcode.ErrInvalidParam.WithMessage("用户名必须为 5-24 位字母、数字、下划线、短横线")
	}
	if email != "" && !emailRe.MatchString(email) {
		return nil, errcode.ErrInvalidParam.WithMessage("邮箱格式不正确")
	}
	if name == "" {
		return nil, errcode.ErrInvalidParam.WithMessage("姓名不能为空")
	}
	if passwordHash == "" {
		return nil, errcode.ErrInvalidParam.WithMessage("密码哈希不能为空")
	}
	return &User{
		ID:        id,
		Username:  username,
		Email:     email,
		Name:      name,
		Password:  passwordHash,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Validate 校验当前聚合根的不变量。
func (u *User) Validate() error {
	if !usernameRe.MatchString(u.Username) {
		return errcode.ErrInvalidParam.WithMessage("用户名必须为 5-24 位字母、数字、下划线、短横线")
	}
	if u.Email != "" && !emailRe.MatchString(u.Email) {
		return errcode.ErrInvalidParam.WithMessage("邮箱格式不正确")
	}
	if strings.TrimSpace(u.Name) == "" {
		return errcode.ErrInvalidParam.WithMessage("姓名不能为空")
	}
	if u.Password == "" {
		return errcode.ErrInvalidParam.WithMessage("密码哈希不能为空")
	}
	return nil
}

// SetName 修改昵称（领域方法）。
func (u *User) SetName(name string, now time.Time) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		return ErrInvalidName
	}
	u.Name = name
	u.UpdatedAt = now
	return nil
}

// SetPasswordHash 更新密码哈希（领域方法）。
//
// 调用方负责把明文密码先 bcrypt，再传入本方法。
func (u *User) SetPasswordHash(hash string, now time.Time) {
	u.Password = hash
	u.UpdatedAt = now
}
