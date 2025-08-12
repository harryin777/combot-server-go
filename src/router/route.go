package router

import (
	"context"

	"xiaozhi-server-go/src/configs"
	cfg "xiaozhi-server-go/src/configs/server"
	"xiaozhi-server-go/src/core/utils"

	"github.com/gin-gonic/gin"
)

// SetupRoutes 统一注册所有路由
func SetupRoutes(groupCtx context.Context, router *gin.Engine, config *configs.Config) error {
	// API路由全部挂载到/api前缀下
	apiGroup := router.Group("/api")

	// 注册各模块路由
	OtaRouter(groupCtx, apiGroup, router, config)
	ActiveRouter(groupCtx, apiGroup, config)
	AuthRouter(groupCtx, apiGroup, config)
	UserRouter(groupCtx, apiGroup, config)
	ConversationRoutes(groupCtx, apiGroup, config)
	VisionRouter(groupCtx, apiGroup, router, config)
	RoleRouter(groupCtx, apiGroup, config)      // 添加角色管理路由
	AgentRoleRouter(groupCtx, apiGroup, config) // 添加智能体角色管理路由

	// 启动配置服务
	cfgServer, err := cfg.NewDefaultCfgService(config, nil)
	if err != nil {
		utils.WithError(groupCtx, err).Error("配置服务初始化失败")
		return err
	}
	if err := cfgServer.Start(groupCtx, router, apiGroup); err != nil {
		utils.WithError(groupCtx, err).Error("配置服务启动失败")
		return err
	}

	utils.Info(groupCtx, "所有路由注册完成")
	return nil
}
