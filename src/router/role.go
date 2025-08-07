package router

import (
	"context"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/handlers"
	"xiaozhi-server-go/src/middleware"

	"github.com/gin-gonic/gin"
)

// RoleRouter 注册角色管理相关路由
func RoleRouter(ctx context.Context, apiGroup *gin.RouterGroup, config *configs.Config) {
	// 创建RoleHandler实例
	roleHandler := handlers.NewRoleHandler(config)

	// 角色管理相关路由
	roleGroup := apiGroup.Group("/roles")
	{
		// 公开路由（不需要认证）
		roleGroup.GET("/templates", roleHandler.GetRoleTemplates)
		roleGroup.GET("/templates/:templateId", roleHandler.GetRoleTemplate)

		// 需要身份验证的路由
		authGroup := roleGroup.Group("")
		authGroup.Use(middleware.JWTAuthMiddleware(config.Server.Token))
		{
			authGroup.POST("/config", roleHandler.SaveRoleConfig)
		}
	}

	utils.Info(ctx, "Role HTTP服务路由注册完成")
}
