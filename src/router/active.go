package router

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/log"
	"combot-server-go/src/handlers"
	"combot-server-go/src/middleware"
	"context"

	"github.com/gin-gonic/gin"
)

// ActiveRouter 注册激活相关路由
func ActiveRouter(ctx context.Context, apiGroup *gin.RouterGroup, config *configs.Config) {
	// 创建ActiveHandler实例
	activeHandler := handlers.NewActiveHandler()

	// 设备管理相关路由 (仅供Web前端使用)
	activeGroup := apiGroup.Group("/active")
	{
		// 需要身份验证的路由
		authGroup := activeGroup.Group("")
		authGroup.Use(middleware.JWTAuthMiddleware(config.Server.Token))
		{
			authGroup.POST("/bind", activeHandler.BindDevice)       // Web前端绑定设备
			authGroup.GET("/devices", activeHandler.GetUserDevices) // 获取用户设备列表
		}
	}

	log.Info(ctx, "Active HTTP服务路由注册完成")
}
