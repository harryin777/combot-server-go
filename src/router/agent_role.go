package router

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/log"
	"combot-server-go/src/handlers"
	"combot-server-go/src/middleware"
	"context"

	"github.com/gin-gonic/gin"
)

// AgentRoleRouter 注册智能角色管理相关路由
func AgentRoleRouter(ctx context.Context, apiGroup *gin.RouterGroup, config *configs.Config) {
	// 创建AgentRoleHandler实例
	agentRoleHandler := handlers.NewAgentRoleHandler(config)

	// 智能角色管理相关路由
	agentRoleGroup := apiGroup.Group("/agent-role")
	{
		// 需要身份验证的路由
		authGroup := agentRoleGroup.Group("")
		authGroup.Use(middleware.JWTAuthMiddleware(config.Server.Token))
		{
			// 角色模板相关路由（只读参考数据）
			authGroup.POST("/templates", agentRoleHandler.GetRoleTemplates)
			authGroup.POST("/template/detail", agentRoleHandler.GetRoleTemplate)

			// 智能体角色配置管理
			configGroup := authGroup.Group("/config")
			{
				configGroup.POST("/get", agentRoleHandler.GetAgentRole)
				configGroup.POST("/create", agentRoleHandler.CreateAgentRole)
				configGroup.POST("/update", agentRoleHandler.UpdateAgentRole)
			}
		}
	}

	log.Info(ctx, "AgentRole HTTP服务路由注册完成")
}
