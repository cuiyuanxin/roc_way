// Package errcode 定义统一错误码与 HTTP 状态码映射。
package errcode

import "fmt"

// Code 表示一个语义错误码。
//
// 字段命名避免 stutter：调用方写 `errcode.ErrInternal.Code` 而非 `errcode.Code.Code`。
type Code struct {
	Code       int    // 业务错误码（数字）
	Message    string // 业务错误描述
	HTTPStatus int    // HTTP 状态码
}

func (c Code) Error() string {
	return fmt.Sprintf("code=%d: %s", c.Code, c.Message)
}

// WithMessage 返回带自定义消息的拷贝。
func (c Code) WithMessage(msg string) Code {
	return Code{Code: c.Code, Message: msg, HTTPStatus: c.HTTPStatus}
}

// 预置错误码。
var (
	ErrInvalidParam     = Code{1000, "参数错误", 400}
	ErrTokenInvalid     = Code{2002, "Token 无效", 401}
	ErrForbidden        = Code{2003, "无权限", 403}
	ErrCSRFToken        = Code{2004, "CSRF 校验失败", 403}
	ErrNotImplemented   = Code{2006, "功能未实现", 501}
	ErrNotFound         = Code{2007, "资源不存在", 404}
	ErrMethodNotAllowed = Code{2008, "HTTP 方法不被允许", 405}
	ErrRateLimited      = Code{3001, "请求过于频繁", 429}
	ErrTimeout          = Code{1504, "请求处理超时", 504}
	ErrInternal         = Code{5000, "服务器内部错误", 500}
	ErrDatabase         = Code{5001, "数据库错误", 500}
	ErrCache            = Code{5002, "缓存错误", 500}
	ErrStorage          = Code{5003, "存储错误", 500}
	ErrConfigNotFound   = Code{5004, "配置错误", 500}
)

// Error 携带 Code 与附加 cause。
type Error struct {
	C     Code
	Cause error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%d: %s: %v", e.C.Code, e.C.Message, e.Cause)
	}
	return e.C.Error()
}

// Unwrap 返回 cause，用于 errors.Is/As。
func (e *Error) Unwrap() error { return e.Cause }

// New 构造一个 errcode.Error。
func New(c Code, cause error) *Error {
	return &Error{C: c, Cause: cause}
}
