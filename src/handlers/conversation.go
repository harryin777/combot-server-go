package handlers

import (
	"combot-server-go/src/core/codes"
	"combot-server-go/src/core/log"
	"combot-server-go/src/core/response"
	"combot-server-go/src/service"

	"github.com/gin-gonic/gin"
)

// ConversationHandler 对话处理器
type ConversationHandler struct {
	conversationService *service.ConversationServiceImpl
}

// NewConversationHandler 创建对话处理器
func NewConversationHandler() *ConversationHandler {
	return &ConversationHandler{
		conversationService: service.NewConversationService().(*service.ConversationServiceImpl),
	}
}

// GetUserConversationsRequest 获取用户对话列表请求参数
type GetUserConversationsRequest struct {
	UserID   uint   `json:"user_id" binding:"required" example:"1"`
	DeviceID string `json:"device_id" binding:"required" example:"device123"`
	Limit    int    `json:"limit" example:"20"`
	Offset   int    `json:"offset" example:"0"`
}

// GetUserConversationsResponse 获取用户对话列表响应
type GetUserConversationsResponse struct {
	Code    int                                   `json:"code" example:"200"`
	Message string                                `json:"message" example:"成功"`
	Data    []GetUserConversationsResponseSession `json:"data"`
}

// GetUserConversationsResponseSession 对话会话信息
type GetUserConversationsResponseSession struct {
	SessionID    string `json:"session_id" example:"session_123"`
	Title        string `json:"title" example:"关于天气的对话..."`
	CurrentRole  string `json:"current_role" example:"天气助手"`
	StartTime    string `json:"start_time" example:"2024-01-01T10:00:00Z"`
	LastActivity string `json:"last_activity" example:"2024-01-01T10:30:00Z"`
	Status       string `json:"status" example:"active"`
	MessageCount int    `json:"message_count" example:"10"`
}

// GetUserConversations 获取当前用户所有机器人的对话历史（左侧会话列表）
func (h *ConversationHandler) GetUserConversations(c *gin.Context) {
	var req GetUserConversationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(c.Request.Context(), "参数绑定失败: "+err.Error())
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	sessions, code, err := h.conversationService.GetUserConversations(c.Request.Context(), req.UserID, req.DeviceID, req.Limit, req.Offset)
	if err != nil {
		log.Error(c.Request.Context(), "获取用户对话列表失败: "+err.Error())
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	// 转换响应格式
	var responseSessions []GetUserConversationsResponseSession
	for _, session := range sessions {
		responseSessions = append(responseSessions, GetUserConversationsResponseSession{
			SessionID:    session.SessionID,
			Title:        session.Title,
			CurrentRole:  session.CombotName,
			StartTime:    session.StartTime.Format("2006-01-02T15:04:05Z"),
			LastActivity: session.LastActivity.Format("2006-01-02T15:04:05Z"),
			Status:       string(rune(session.Status + 48)), // 转换枚举为字符串
			MessageCount: session.MessageCount,
		})
	}

	response.Success(c, responseSessions)
}

// GetConversationMessagesRequest 获取对话详情请求参数
type GetConversationMessagesRequest struct {
	SessionID string `json:"session_id" binding:"required" example:"session_123"`
	Limit     int    `json:"limit" example:"50"`
	Offset    int    `json:"offset" example:"0"`
}

// GetConversationMessagesResponse 获取对话详情响应
type GetConversationMessagesResponse struct {
	Code    int                                      `json:"code" example:"200"`
	Message string                                   `json:"message" example:"成功"`
	Data    []GetConversationMessagesResponseMessage `json:"data"`
}

// GetConversationMessagesResponseMessage 对话消息信息
type GetConversationMessagesResponseMessage struct {
	ID          uint   `json:"id" example:"1"`
	SessionID   string `json:"session_id" example:"session_123"`
	Role        string `json:"role" example:"user"`
	Content     string `json:"content" example:"你好，今天天气怎么样？"`
	MessageType string `json:"message_type" example:"text"`
	Round       int    `json:"round" example:"1"`
	CreatedAt   string `json:"created_at" example:"2024-01-01T10:00:00Z"`
}

// GetConversationMessages 根据用户、机器人和sessionID获取详细对话历史（右侧对话内容）
func (h *ConversationHandler) GetConversationMessages(c *gin.Context) {
	var req GetConversationMessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Error(c.Request.Context(), "参数绑定失败: "+err.Error())
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	messages, code, err := h.conversationService.GetConversationMessages(c.Request.Context(), req.SessionID, req.Limit, req.Offset)
	if err != nil {
		log.Error(c.Request.Context(), "获取对话详情失败: "+err.Error())
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	// 转换响应格式
	var responseMessages []GetConversationMessagesResponseMessage
	for _, message := range messages {
		responseMessages = append(responseMessages, GetConversationMessagesResponseMessage{
			ID:          uint(message.ID),
			SessionID:   message.SessionID,
			Role:        string(rune(message.Role + 48)), // 转换枚举为字符串
			Content:     message.Content,
			MessageType: string(rune(message.MessageType + 48)), // 转换枚举为字符串
			Round:       1,                                      // 使用默认值，因为模型中没有这个字段
			CreatedAt:   message.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	response.Success(c, responseMessages)
}
