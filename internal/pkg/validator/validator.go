// Package validator 封装 go-playground/validator/v10 并注册中文翻译器。
package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// v 全局 validator 实例。
var v *validator.Validate

func init() {
	v = validator.New()
	if gv, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v = gv
	}
}

// Bind 从 gin.Context 绑定并校验请求体到 dst，错误统一包装为 errcode.ErrInvalidParam。
func Bind(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		return mapErr(err, dst)
	}
	if err := v.Struct(dst); err != nil {
		return mapErr(err, dst)
	}
	return nil
}

func mapErr(err error, dst any) error {
	var verr validator.ValidationErrors
	if errors.As(err, &verr) {
		t := reflect.TypeOf(dst).Elem()
		fields := make([]string, 0, len(verr))
		for _, fe := range verr {
			field, _ := t.FieldByName(fe.StructField())
			name := field.Tag.Get("json")
			if name == "" {
				name = fe.StructField()
			}
			fields = append(fields, fmt.Sprintf("%s(%s)", name, humanizeTag(fe)))
		}
		return errcode.New(errcode.ErrInvalidParam.WithMessage("字段不合法: "+strings.Join(fields, ", ")), err)
	}
	return errcode.New(errcode.ErrInvalidParam.WithMessage(err.Error()), err)
}

func humanizeTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "必填"
	case "email":
		return "邮箱格式不正确"
	case "min":
		return fmt.Sprintf("最小 %s", fe.Param())
	case "max":
		return fmt.Sprintf("最大 %s", fe.Param())
	default:
		return fe.Tag()
	}
}
