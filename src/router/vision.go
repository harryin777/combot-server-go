package router

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/log"
	"combot-server-go/src/vision"
	"context"

	"github.com/gin-gonic/gin"
)

// VisionRouter 注册 Vision 相关路由
func VisionRouter(ctx context.Context, router *gin.RouterGroup, engine *gin.Engine, config *configs.Config) {
	visionService, err := vision.NewDefaultVisionService(config)
	if err != nil {
		log.WithError(ctx, err).Error("Vision 服务初始化失败")
		return
	}
	if err := visionService.Start(ctx, engine, router); err != nil {
		log.WithError(ctx, err).Error("Vision 服务启动失败")
		return
	}
}
