package router

import (
	"context"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/handlers"
	"xiaozhi-server-go/src/middleware"

	"github.com/gin-gonic/gin"
)

// UserRouter 注册用户相关路由
func UserRouter(ctx context.Context, apiGroup *gin.RouterGroup, config *configs.Config) {
	// 创建UserHandler实例
	userHandler := handlers.NewUserHandler(config)

	// 注册用户相关路由
	userGroup := apiGroup.Group("/user")
	{
		// 用户名密码登录（无需认证）
		userGroup.POST("/login", userHandler.UsernamePasswordLogin)

		// 需要JWT认证的路由
		authenticated := userGroup.Use(middleware.JWTAuthMiddleware(config.Server.Token))
		authenticated.PUT("/profile", userHandler.UpdateProfile)
		authenticated.PUT("/change-password", userHandler.ChangePassword)
		authenticated.GET("/devices", userHandler.GetLoginDevices)
		authenticated.POST("/logout-device", userHandler.LogoutDevice)
		authenticated.DELETE("/delete-account", userHandler.DeleteAccount)
	}

	utils.Info(ctx, "User HTTP服务路由注册完成")
}
