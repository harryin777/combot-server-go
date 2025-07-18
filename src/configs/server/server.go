package server

import (
	"context"
	"xiaozhi-server-go/src/configs"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type DefaultCfgService struct {
	config *configs.Config
}

// NewDefaultCfgService 构造函数
func NewDefaultCfgService(config *configs.Config, logger interface{}) (*DefaultCfgService, error) {
	service := &DefaultCfgService{
		config: config,
	}

	return service, nil
}

// Start 实现 CfgService 接口，注册所有 Cfg 相关路由
func (s *DefaultCfgService) Start(ctx context.Context, engine *gin.Engine, apiGroup *gin.RouterGroup) error {

	apiGroup.GET("/cfg", s.handleGet)
	apiGroup.POST("/cfg", s.handlePost)
	apiGroup.OPTIONS("/cfg", s.handleOptions)

	logrus.Info("Cfg HTTP服务路由注册完成")
	return nil
}

func (s *DefaultCfgService) handleGet(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Cfg service is running",
	})
}

func (s *DefaultCfgService) handlePost(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Cfg service is running",
	})
}

func (s *DefaultCfgService) handleOptions(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type")
	c.Status(204) // No Content
}
