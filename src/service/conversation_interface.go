package service

import (
	"context"
	"xiaozhi-server-go/src/models"
)

// ConversationService 定义对话服务接口
type ConversationService interface {
	SaveConversation(ctx context.Context, sessionID, deviceID, clientID string, userID int, message string, role, messageType int, combotName string) error
	GetUserConversations(ctx context.Context, userID uint, deviceID string, limit int, offset int) ([]models.ConversationSession, error)
	GetConversationMessages(ctx context.Context, sessionID string, limit int, offset int) ([]models.ConversationMessage, error)
}
