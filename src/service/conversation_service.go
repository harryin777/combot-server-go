package service

import (
	"context"
	"time"

	"xiaozhi-server-go/src/core/codes"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/dao"
	"xiaozhi-server-go/src/models"
)

// ConversationServiceImpl 对话服务
type ConversationServiceImpl struct {
	dao *dao.ConversationDAO
}

// NewConversationService 创建对话服务实例
func NewConversationService() ConversationService {
	return &ConversationServiceImpl{
		dao: dao.NewConversationDAO(),
	}
}

// SaveConversation 保存用户和AI的对话（在每次交流后立即调用）
func (s *ConversationServiceImpl) SaveConversation(ctx context.Context,
	sessionID, deviceID, clientID string, userID int, message string, role, messageType int, combotName string) (interface{}, int, error) {
	utils.Infof(ctx, "SaveConversation 保存对话: sessionID=%s, userID=%v,", sessionID, userID)
	now := time.Now()

	// 创建两条消息记录：用户消息 + AI回复
	messages := []models.ConversationMessage{
		{
			SessionID:   sessionID,
			Role:        models.MessageRole(role),
			Content:     message,
			MessageType: models.MessageType(messageType),
			Metadata:    []byte("{}"),
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	// 批量保存消息
	if err := s.dao.SaveMessages(ctx, messages); err != nil {
		utils.Errorf(ctx, "保存对话失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	// 更新或创建会话
	if err := s.dao.UpdateOrCreateSession(ctx, sessionID, deviceID, clientID, userID, combotName, message); err != nil {
		utils.Errorf(ctx, "更新会话失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	utils.Debugf(ctx, "保存对话成功: sessionID=%s", sessionID)
	return nil, codes.CodeSuccess, nil
}

// GetUserConversations 获取当前用户所有机器人的对话历史（左侧列表）
func (s *ConversationServiceImpl) GetUserConversations(ctx context.Context, userID uint, deviceID string, limit int, offset int) ([]models.ConversationSession, int, error) {
	utils.Infof(ctx, "GetUserConversations 获取用户对话列表: userID=%d, deviceID=%s, limit=%d, offset=%d", userID, deviceID, limit, offset)
	sessions, err := s.dao.GetUserConversations(ctx, userID, deviceID, limit, offset)
	if err != nil {
		utils.Errorf(ctx, "获取用户对话列表失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	return sessions, codes.CodeSuccess, nil
}

// GetConversationMessages 根据用户、机器人和sessionID获取详细对话历史（右侧对话内容）
func (s *ConversationServiceImpl) GetConversationMessages(ctx context.Context, sessionID string, limit int, offset int) ([]models.ConversationMessage, int, error) {
	utils.Infof(ctx, "GetConversationMessages 获取对话详情: sessionID=%s, limit=%d, offset=%d", sessionID, limit, offset)
	messages, err := s.dao.GetConversationMessages(ctx, sessionID, limit, offset)
	if err != nil {
		utils.Errorf(ctx, "获取对话详情失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	return messages, codes.CodeSuccess, nil
}
