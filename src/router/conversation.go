package router

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/handlers"
	"combot-server-go/src/middleware"
	"context"

	"github.com/gin-gonic/gin"
)

// ConversationRoutes 设置对话相关路由
func ConversationRoutes(ctx context.Context, apiGroup *gin.RouterGroup, config *configs.Config) {
	conversationHandler := handlers.NewConversationHandler()
	// 对话相关路由
	conversation := apiGroup.Group("/conversation")
	conversation.Use(middleware.JWTAuthMiddleware(config.Server.Token))

	{
		// 获取用户所有机器人的对话历史（左侧会话列表）
		conversation.POST("/user-conversations", conversationHandler.GetUserConversations)

		// 根据sessionID获取详细对话历史（右侧对话内容）
		conversation.POST("/messages", conversationHandler.GetConversationMessages)
	}
}
