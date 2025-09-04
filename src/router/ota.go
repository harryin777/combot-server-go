package router

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/log"
	"combot-server-go/src/ota"
	"context"

	"github.com/gin-gonic/gin"
)

// OtaRouter 注册OTA相关路由
func OtaRouter(ctx context.Context, router *gin.RouterGroup, engine *gin.Engine, config *configs.Config) {
	otaService := ota.NewDefaultOTAService(config.Web.Websocket, config)
	if err := otaService.Start(ctx, engine, router); err != nil {
		log.WithError(ctx, err).Error("OTA 服务启动失败")
		return
	}
}
