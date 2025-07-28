package router

import (
	"context"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/handlers"
	"xiaozhi-server-go/src/middleware"

	"github.com/gin-gonic/gin"
)

// ActiveRouter 注册激活相关路由
func ActiveRouter(ctx context.Context, apiGroup *gin.RouterGroup, config *configs.Config) {
	// 创建ActiveHandler实例
	activeHandler := handlers.NewActiveHandler()

	// 匹配combot实际API调用
	apiGroup.Any("", activeHandler.CheckVersion)       // 对应combot的CheckVersion调用
	apiGroup.POST("/activate", activeHandler.Activate) // 对应combot的Activate调用

	// 设备添加相关路由 (兼容现有逻辑)
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

	utils.Info(ctx, "Active HTTP服务路由注册完成")
}
