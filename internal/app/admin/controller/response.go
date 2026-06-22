package controller

import (
	"github.com/gin-gonic/gin"

	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// WriteOK 统一成功响应。
func WriteOK(c *gin.Context, data any) {
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": data})
}

// WriteErr 把 errcode.Error 翻译为 HTTP 响应。
func WriteErr(c *gin.Context, err error) {
	if e, ok := err.(*errcode.Error); ok {
		c.JSON(e.C.HTTPStatus, gin.H{"code": e.C.Code, "message": e.C.Message})
		return
	}
	if e, ok := err.(errcode.Code); ok {
		c.JSON(e.HTTPStatus, gin.H{"code": e.Code, "message": e.Message})
		return
	}
	c.JSON(errcode.ErrInternal.HTTPStatus,
		gin.H{"code": errcode.ErrInternal.Code, "message": err.Error()})
}
