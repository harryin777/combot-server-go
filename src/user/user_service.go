package user

import (
	"context"
	"xiaozhi-server-go/src/configs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UserService 用户服务
type UserService struct {
	config *configs.Config
	db     *gorm.DB
}

// NewUserService 创建用户服务实例
func NewUserService(config *configs.Config, logger interface{}, db *gorm.DB) *UserService {
	return &UserService{
		config: config,
		db:     db,
	}
}

// Start 启动用户服务
func (us *UserService) Start(ctx context.Context, router *gin.Engine, apiGroup *gin.RouterGroup) error {
	// 注册用户相关的路由
	userGroup := apiGroup.Group("/user")
	{
		userGroup.POST("/login", us.LoginWithNewLogger)
	}
	return nil
}
