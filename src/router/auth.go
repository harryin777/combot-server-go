package router

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/log"
	"combot-server-go/src/handlers"
	"context"

	"github.com/gin-gonic/gin"
)

// AuthRouter 注册认证相关路由
func AuthRouter(ctx context.Context, apiGroup *gin.RouterGroup, config *configs.Config) {
	// 创建AuthHandler实例
	authHandler := handlers.NewAuthHandler(config)

	// 注册认证相关路由
	authGroup := apiGroup.Group("/auth")
	{
		// 图形验证码
		apiGroup.GET("/captcha/image", authHandler.GetCaptcha)

		// 短信验证码
		apiGroup.POST("/sms/send", authHandler.SendSMS)

		// 手机号登录/注册
		authGroup.POST("/phone", authHandler.PhoneAuth)
	}

	log.Info(ctx, "Auth HTTP服务路由注册完成")
}
