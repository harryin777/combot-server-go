package ota

import (
	"context"
	"xiaozhi-server-go/src/configs"

	"github.com/gin-gonic/gin"
)

type DefaultOTAService struct {
	UpdateURL string
	Config    *configs.Config
}

// NewDefaultOTAService 构造函数
func NewDefaultOTAService(updateURL string, config *configs.Config) *DefaultOTAService {
	return &DefaultOTAService{
		UpdateURL: updateURL,
		Config:    config,
	}
}

// Start 注册 OTA 相关路由
func (s *DefaultOTAService) Start(ctx context.Context, engine *gin.Engine, apiGroup *gin.RouterGroup) error {
	apiGroup.OPTIONS("/ota/", handleOtaOptions)
	apiGroup.GET("/ota/", func(c *gin.Context) { handleOtaGet(c, s.UpdateURL) })
	apiGroup.POST("/ota/", func(c *gin.Context) { handleOtaPost(c, s.UpdateURL, s.Config) })
	apiGroup.POST("/ota/activate", func(c *gin.Context) { handleOtaPost(c, s.UpdateURL, s.Config) })

	engine.GET("/ota_bin/:filename", handleOtaBinDownload)

	return nil
}
