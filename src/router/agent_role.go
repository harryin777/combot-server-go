package router

import (
	"context"

	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/handlers"
	"xiaozhi-server-go/src/middleware"

	"github.com/gin-gonic/gin"
)

// AgentRoleRouter 设置智能体角色相关路由
func AgentRoleRouter(groupCtx context.Context, apiGroup *gin.RouterGroup, config *configs.Config) {
	agentRoleHandler := handlers.NewAgentRoleHandler()

	// 智能体角色路由组
	agentGroup := apiGroup.Group("/agent")
	{
		// 获取角色模板列表（无需认证）
		agentGroup.GET("/role/templates", agentRoleHandler.GetRoleTemplates)

		// 需要认证的路由
		authGroup := agentGroup.Use(middleware.JWTAuthMiddleware(config.Server.Token))
		{
			// 获取智能体角色配置
			authGroup.GET("/role", agentRoleHandler.GetAgentRole)
			// 保存智能体角色配置
			authGroup.POST("/role", agentRoleHandler.SaveAgentRole)
			// 删除智能体角色配置
			authGroup.DELETE("/role", agentRoleHandler.DeleteAgentRole)
			// 基于模板创建智能体角色
			authGroup.POST("/role/from-template", agentRoleHandler.CreateAgentRoleFromTemplate)
		}
	}

	// 用户相关的智能体角色路由
	userGroup := apiGroup.Group("/user")
	userAuthGroup := userGroup.Use(middleware.JWTAuthMiddleware(config.Server.Token))
	{
		// 获取用户的所有智能体角色配置
		userAuthGroup.GET("/agent-roles", agentRoleHandler.GetUserAgentRoles)
	}

	utils.Info(groupCtx, "智能体角色路由注册完成")
}
