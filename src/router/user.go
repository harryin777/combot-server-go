package router

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/handlers"
	"combot-server-go/src/log"
	"combot-server-go/src/middleware"
	"context"

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

		// 用户名密码注册（无需认证）
		userGroup.POST("/register", userHandler.UsernamePasswordRegister)

		// 需要JWT认证的路由
		authenticated := userGroup.Use(middleware.JWTAuthMiddleware(config.Server.Token))
		authenticated.GET("/profile", userHandler.GetProfile)
		authenticated.POST("/profile", userHandler.UpdateProfile)
		authenticated.POST("/change-password", userHandler.ChangePassword)
		authenticated.POST("/devices", userHandler.GetLoginDevices)
		authenticated.POST("/logout-device", userHandler.LogoutDevice)
		authenticated.POST("/delete-account", userHandler.DeleteAccount)
	}

	log.Info(ctx, "User HTTP服务路由注册完成")
}
