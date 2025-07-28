package router

import (
	"context"

	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	cfg "xiaozhi-server-go/src/configs/server"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/user"
	"xiaozhi-server-go/src/vision"

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

	// 启动用户管理服务
	userService := user.NewUserService(config, nil, database.DB)
	if err := userService.Start(groupCtx, router, apiGroup); err != nil {
		utils.WithError(context.Background(), err).Error("用户管理服务启动失败")
		return err
	}

	// 启动Vision服务
	visionService, err := vision.NewDefaultVisionService(config)
	if err != nil {
		utils.WithError(context.Background(), err).Error("Vision 服务初始化失败")
		return err
	}
	if err := visionService.Start(groupCtx, router, apiGroup); err != nil {
		utils.WithError(context.Background(), err).Error("Vision 服务启动失败")
		return err
	}

	// 启动配置服务
	cfgServer, err := cfg.NewDefaultCfgService(config, nil)
	if err != nil {
		utils.WithError(context.Background(), err).Error("配置服务初始化失败")
		return err
	}
	if err := cfgServer.Start(groupCtx, router, apiGroup); err != nil {
		utils.WithError(context.Background(), err).Error("配置服务启动失败")
		return err
	}

	utils.Info(context.Background(), "所有路由注册完成")
	return nil
}
