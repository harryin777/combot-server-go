package router

import (
	"context"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/handlers"
	"xiaozhi-server-go/src/middleware"

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
