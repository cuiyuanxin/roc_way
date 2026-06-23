// Package response 提供统一的 API 响应结构定义。
package response

import "github.com/google/uuid"

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
	Code    int         `json:"code" example:"1000"`
	Message string      `json:"message" example:"参数错误"`
	Details interface{} `json:"details,omitempty"`
}

// NewResponse 创建通用响应。
func NewResponse[T any](data T, message string) Response[T] {
	return Response[T]{
		Code:      0,
		Message:   message,
		Data:      data,
		RequestID: uuid.New().String(),
	}
}

// NewPaginatedResponse 创建分页响应。
func NewPaginatedResponse[T any](list []T, total int64, page, pageSize int) PaginatedResponse[T] {
	return PaginatedResponse[T]{
		Code:      0,
		Message:   "success",
		Data:      list,
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
		RequestID: uuid.New().String(),
	}
}

// NewErrorResponse 创建错误响应。
func NewErrorResponse(code int, message string, details interface{}) ErrorResponse {
	return ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
	}
}
