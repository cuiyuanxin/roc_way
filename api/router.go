// Package api 提供 API 路由注册与 Swagger 文档集成。
package api

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/cuiyuanxin/roc_way/api/docs" // 导入 swag 生成的文档
)

// RegisterRoutes 注册 API 路由，包括 Swagger UI。
func RegisterRoutes(e *gin.Engine) {
	// Swagger UI
	e.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}
