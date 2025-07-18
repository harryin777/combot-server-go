package router

import (
	"context"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/handlers"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ActiveRouter 注册激活相关路由
func ActiveRouter(ctx context.Context, apiGroup *gin.RouterGroup, config *configs.Config) {
	// 创建ActiveHandler实例
	activeHandler := handlers.NewActiveHandler(config)

	// 注册激活相关路由
	activeGroup := apiGroup.Group("/active")
	{
		activeGroup.POST("/register", activeHandler.Register)
		activeGroup.POST("/login", activeHandler.Login)
		activeGroup.POST("/token", activeHandler.GetToken)
		activeGroup.POST("/logout", activeHandler.Logout)
		activeGroup.GET("/info", activeHandler.Info)
	}

	logrus.Info("Active HTTP服务路由注册完成")
}
