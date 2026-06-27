// Package response 统一对外 HTTP 响应：响应结构 + 构造器 + gin 适配。
//
// 统一响应字段：
//   - code:       业务错误码，0 表示成功
//   - message:    给用户/前端看的文案
//   - data:       业务数据（成功响应）
//   - request_id: 链路追踪 ID（由 middleware.RequestID 注入）
//   - details:    错误详情（仅 debug 模式填，release 永远 nil）
//
// request_id 统一由调用方传入（通过 WriteOK / WriteErr 自动从 gin.Context 读取），**禁止**在此包内自生成。
//
// gin 适配：
//   - WriteOK(c, data)  → c.JSON(200, NewResponse(data, "ok", rid))
//   - WriteErr(c, err)  → 把 errcode.Error / errcode.Code / 未知 error 翻译成 ErrorResponse
//
// 错误翻译规则（WriteErr）：
//   - *errcode.Error / errcode.Code → 使用对应 HTTPStatus + Code + Message
//   - 其它 error                    → 走 errcode.ErrInternal(500)，message = err.Error()
package response

import (
	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// requestIDContextKey gin.Context 中 request_id 的 key。
//
// 与 internal/pkg/middleware.DefaultRequestIDContextKey 保持一致；
// 此处不导入 middleware 包避免循环依赖（middleware → response → middleware）。
const requestIDContextKey = "request_id"

// Response 通用响应结构。
type Response[T any] struct {
	Code      int    `json:"code" example:"0"`
	Message   string `json:"message" example:"success"`
	Data      T      `json:"data,omitempty"`
	RequestID string `json:"request_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// PaginatedResponse 分页响应结构。
type PaginatedResponse[T any] struct {
	Code      int    `json:"code" example:"0"`
	Message   string `json:"message" example:"success"`
	Data      []T    `json:"list"`
	Total     int64  `json:"total" example:"100"`
	Page      int    `json:"page" example:"1"`
	PageSize  int    `json:"page_size" example:"10"`
	RequestID string `json:"request_id,omitempty"`
}

// ErrorResponse 错误响应结构。
type ErrorResponse struct {
	Code      int         `json:"code" example:"1000"`
	Message   string      `json:"message" example:"参数错误"`
	RequestID string      `json:"request_id,omitempty"`
	Details   interface{} `json:"details,omitempty"`
}

// NewResponse 创建通用响应。
func NewResponse[T any](data T, message, requestID string) Response[T] {
	return Response[T]{
		Code:      0,
		Message:   message,
		Data:      data,
		RequestID: requestID,
	}
}

// NewPaginatedResponse 创建分页响应。
func NewPaginatedResponse[T any](list []T, total int64, page, pageSize int, requestID string) PaginatedResponse[T] {
	return PaginatedResponse[T]{
		Code:      0,
		Message:   "success",
		Data:      list,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
		RequestID: requestID,
	}
}

// NewErrorResponse 创建错误响应。
func NewErrorResponse(code int, message, requestID string, details interface{}) ErrorResponse {
	return ErrorResponse{
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Details:   details,
	}
}

// WriteOK 统一成功响应。message 固定为 "ok"。
func WriteOK(c *gin.Context, data any) {
	c.JSON(200, NewResponse(data, "ok", getRequestID(c)))
}

// WriteErr 把 error 翻译为 HTTP 错误响应。
//
// 支持的 err 类型：
//   - *errcode.Error  → 使用其 C.HTTPStatus / C.Code / C.Message
//   - errcode.Code    → 直接用其 HTTPStatus / Code Message
//   - 其它 error      → 走 errcode.ErrInternal(500)，但**不**回显 err.Error() 给客户端
//     （原始 err 仅在服务端可观测，调用方应自行在 service 层 zap.Error 落日志）
//
// 不调用 c.Abort()，调用方按需决定是否 return / 后续是否再写响应。
func WriteErr(c *gin.Context, err error) {
	if e, ok := err.(*errcode.Error); ok {
		c.JSON(e.C.HTTPStatus, NewErrorResponse(
			e.C.Code, e.C.Message, getRequestID(c), nil,
		))
		return
	}
	if e, ok := err.(errcode.Code); ok {
		c.JSON(e.HTTPStatus, NewErrorResponse(
			e.Code, e.Message, getRequestID(c), nil,
		))
		return
	}
	// 兜底：未知 error 类型不把 err.Error() 透传客户端（可能含 SQL / 堆栈 / 路径等敏感信息），
	// 统一回 ErrInternal 的安全文案。原始 err 须由调用方在 service 层落日志。
	c.JSON(errcode.ErrInternal.HTTPStatus,
		NewErrorResponse(
			errcode.ErrInternal.Code, errcode.ErrInternal.Message, getRequestID(c), nil,
		))
}

// getRequestID 从 gin.Context 取 request_id，找不到返回空字符串。
func getRequestID(c *gin.Context) string {
	v, _ := c.Get(requestIDContextKey)
	s, _ := v.(string)
	return s
}
