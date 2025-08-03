package service

import (
	"context"
	"fmt"
	"time"

	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/dao"
	"xiaozhi-server-go/src/models"
)

// ConversationService 对话服务
type ConversationService struct {
	dao *dao.ConversationDAO
}

// NewConversationService 创建对话服务实例
func NewConversationService() *ConversationService {
	return &ConversationService{
		dao: dao.NewConversationDAO(),
	}
}

// SaveConversation 保存用户和AI的对话（在每次交流后立即调用）
func (s *ConversationService) SaveConversation(ctx context.Context, sessionID, deviceID, clientID string, userID *uint, userMessage, aiMessage string, aiRole string, round int) error {
	utils.Infof(ctx, "SaveConversation 保存对话: sessionID=%s, userID=%v, round=%d", sessionID, userID, round)
	now := time.Now()

	// 创建两条消息记录：用户消息 + AI回复
	messages := []models.ConversationMessage{
		{
			SessionID:   sessionID,
			Role:        "user",
			Content:     userMessage,
			MessageType: "text",
			Round:       round,
			Metadata:    []byte("{}"),
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			SessionID:   sessionID,
			Role:        "assistant",
			Content:     aiMessage,
			MessageType: "text",
			Round:       round,
			Metadata:    []byte("{}"),
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	// 批量保存消息
	if err := s.dao.SaveMessages(ctx, messages); err != nil {
		utils.Error(ctx, fmt.Sprintf("保存对话失败: %v", err))
		return fmt.Errorf("保存对话失败: %w", err)
	}

	// 更新或创建会话
	if err := s.dao.UpdateOrCreateSession(ctx, sessionID, deviceID, clientID, userID, aiRole, userMessage); err != nil {
		utils.Error(ctx, fmt.Sprintf("更新会话失败: %v", err))
		return fmt.Errorf("更新会话失败: %w", err)
	}

	utils.Debug(ctx, fmt.Sprintf("保存对话成功: sessionID=%s, round=%d", sessionID, round))
	return nil
}

// GetUserConversations 获取当前用户所有机器人的对话历史（左侧列表）
func (s *ConversationService) GetUserConversations(ctx context.Context, userID uint, deviceID string, limit int, offset int) ([]models.ConversationSession, error) {
	utils.Infof(ctx, "GetUserConversations 获取用户对话列表: userID=%d, deviceID=%s, limit=%d, offset=%d", userID, deviceID, limit, offset)
	sessions, err := s.dao.GetUserConversations(ctx, userID, deviceID, limit, offset)
	if err != nil {
		utils.Error(ctx, fmt.Sprintf("获取用户对话列表失败: %v", err))
		return nil, fmt.Errorf("获取用户对话列表失败: %w", err)
	}

	return sessions, nil
}

// GetConversationMessages 根据用户、机器人和sessionID获取详细对话历史（右侧对话内容）
func (s *ConversationService) GetConversationMessages(ctx context.Context, sessionID string, limit int, offset int) ([]models.ConversationMessage, error) {
	utils.Infof(ctx, "GetConversationMessages 获取对话详情: sessionID=%s, limit=%d, offset=%d", sessionID, limit, offset)
	messages, err := s.dao.GetConversationMessages(ctx, sessionID, limit, offset)
	if err != nil {
		utils.Error(ctx, fmt.Sprintf("获取对话详情失败: %v", err))
		return nil, fmt.Errorf("获取对话详情失败: %w", err)
	}

	return messages, nil
}
