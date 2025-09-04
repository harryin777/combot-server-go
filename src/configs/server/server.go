package server

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/log"
	"context"

	"github.com/gin-gonic/gin"
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

	log.Info(ctx, "Cfg HTTP服务路由注册完成")
	return nil
}

// handleGet 获取配置服务状态
// @Summary 获取配置服务状态
// @Description 获取配置服务的运行状态
// @Tags 配置管理
// @Produce json
// @Success 200 {object} map[string]interface{} "配置服务状态"
// @Router /api/configs [get]
func (s *DefaultCfgService) handleGet(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Cfg service is running",
	})
}

// handlePost 配置服务POST请求
// @Summary 配置服务POST请求
// @Description 处理配置相关的POST请求
// @Tags 配置管理
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "配置服务状态"
// @Router /api/configs [post]
func (s *DefaultCfgService) handlePost(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Cfg service is running",
	})
}

// handleOptions 配置服务预检请求
// @Summary 配置服务预检请求
// @Description 处理配置服务的OPTIONS预检请求
// @Tags 配置管理
// @Success 200 "OK"
// @Router /api/configs [options]
func (s *DefaultCfgService) handleOptions(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type")
	c.Status(204) // No Content
}
