package router

import (
	"context"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/ota"

	"github.com/gin-gonic/gin"
)

// OtaRouter 注册OTA相关路由
func OtaRouter(ctx context.Context, router *gin.RouterGroup, engine *gin.Engine, config *configs.Config) {
	otaService := ota.NewDefaultOTAService(config.Web.Websocket, config)
	if err := otaService.Start(ctx, engine, router); err != nil {
		utils.WithError(ctx, err).Error("OTA 服务启动失败")
		return
	}
}
