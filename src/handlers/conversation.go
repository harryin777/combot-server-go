package handlers

import (
	"context"
	"net/http"
	"strconv"

	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/service"

	"github.com/gin-gonic/gin"
)

// ConversationHandler 对话历史API处理器
type ConversationHandler struct {
	conversationService *service.ConversationService
}

// NewConversationHandler 创建对话历史API处理器
func NewConversationHandler() *ConversationHandler {
	   return &ConversationHandler{
			   conversationService: service.NewConversationService(),
	   }
}
}

// GetConversationHistory 获取指定会话的对话历史
// GET /api/conversations/:sessionId/history
func (h *ConversationHandler) GetConversationHistory(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "session_id 参数不能为空",
		})
		return
	}

	// 获取分页参数
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "limit 参数格式错误",
		})
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "offset 参数格式错误",
		})
		return
	}

	// 获取对话历史
	histories, err := h.conversationService.GetConversationHistory(context.Background(), sessionID, limit, offset)
   if err != nil {
		   utils.Errorf(c, "获取对话历史失败: %v", err)
		   c.JSON(http.StatusInternalServerError, gin.H{
				   "error": "获取对话历史失败",
		   })
		   return
   }

	c.JSON(http.StatusOK, gin.H{
		"data": histories,
		"pagination": gin.H{
			"limit":  limit,
			"offset": offset,
			"count":  len(histories),
		},
	})
}

// GetDeviceConversations 获取设备的所有会话列表
// GET /api/devices/:deviceId/conversations
func (h *ConversationHandler) GetDeviceConversations(c *gin.Context) {
	deviceID := c.Param("deviceId")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "device_id 参数不能为空",
		})
		return
	}

	// 获取分页参数
	limitStr := c.DefaultQuery("limit", "20")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "limit 参数格式错误",
		})
		return
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "offset 参数格式错误",
		})
		return
	}

	// 获取设备会话列表
	sessions, err := h.conversationService.GetConversationsByDevice(context.Background(), deviceID, limit, offset)
   if err != nil {
		   utils.Errorf(c, "获取设备会话列表失败: %v", err)
		   c.JSON(http.StatusInternalServerError, gin.H{
				   "error": "获取设备会话列表失败",
		   })
		   return
   }

	c.JSON(http.StatusOK, gin.H{
		"data": sessions,
		"pagination": gin.H{
			"limit":  limit,
			"offset": offset,
			"count":  len(sessions),
		},
	})
}

// GetRecentConversation 获取最近的对话历史（用于恢复对话上下文）
// GET /api/conversations/:sessionId/recent
func (h *ConversationHandler) GetRecentConversation(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "session_id 参数不能为空",
		})
		return
	}

	// 获取最大消息数参数
	maxMessagesStr := c.DefaultQuery("max_messages", "10")
	maxMessages, err := strconv.Atoi(maxMessagesStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "max_messages 参数格式错误",
		})
		return
	}

	// 获取最近对话历史
	messages, err := h.conversationService.GetRecentConversationHistory(context.Background(), sessionID, maxMessages)
   if err != nil {
		   utils.Errorf(c, "获取最近对话历史失败: %v", err)
		   c.JSON(http.StatusInternalServerError, gin.H{
				   "error": "获取最近对话历史失败",
		   })
		   return
   }

	c.JSON(http.StatusOK, gin.H{
		"data":  messages,
		"count": len(messages),
	})
}

// CloseConversation 关闭会话
// POST /api/conversations/:sessionId/close
func (h *ConversationHandler) CloseConversation(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "session_id 参数不能为空",
		})
		return
	}

	// 关闭会话
	err := h.conversationService.CloseSession(context.Background(), sessionID)
   if err != nil {
		   utils.Errorf(c, "关闭会话失败: %v", err)
		   c.JSON(http.StatusInternalServerError, gin.H{
				   "error": "关闭会话失败",
		   })
		   return
   }

	c.JSON(http.StatusOK, gin.H{
		"message": "会话已关闭",
	})
}
