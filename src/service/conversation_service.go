package service

import (
	"combot-server-go/src/log"
	"context"
	"time"

	"combot-server-go/src/configs/database"
	"combot-server-go/src/core/codes"
	"combot-server-go/src/dao"
	"combot-server-go/src/models"
	"errors"

	"gorm.io/gorm"
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

// getUserIDByClientID 根据 clientID 查找用户ID
func (s *ConversationServiceImpl) getUserIDByClientID(ctx context.Context, clientID string) (int, error) {
	if clientID == "" {
		return 0, nil // 如果 clientID 为空，返回 0 表示未找到用户
	}

	var device models.Device
	err := database.DB.WithContext(ctx).Where("client_id = ?", clientID).First(&device).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Debugf(ctx, "未找到 clientID 对应的设备: %s", clientID)
		return 0, nil // 设备未找到，返回 0
	}

	if err != nil {
		log.Errorf(ctx, "查询设备失败: %v", err)
		return 0, err
	}

	// 检查设备是否已绑定用户
	if device.UserID == nil {
		log.Debugf(ctx, "设备 %s 未绑定用户", clientID)
		return 0, nil // 设备未绑定用户，返回 0
	}

	log.Debugf(ctx, "根据 clientID %s 找到用户ID: %d", clientID, *device.UserID)
	return int(*device.UserID), nil
}

// SaveConversation 保存用户和AI的对话（在每次交流后立即调用）
func (s *ConversationServiceImpl) SaveConversation(ctx context.Context,
	sessionID, deviceID, clientID string, message string, role, messageType int, combotName string) (err error) {

	// 如果传入的 userID 为 0，则尝试根据 clientID 查找用户ID
	var foundUserID int
	if clientID != "" {
		foundUserID, err = s.getUserIDByClientID(ctx, clientID)
		if err != nil {
			log.Errorf(ctx, "根据 clientID 查找用户ID失败: %v", err)
			// 这里不返回错误，继续使用原来的 userID (0)，确保对话能正常保存
		}
	} else {
		log.Errorf(ctx, "clientID 为空，无法查找用户ID")
		return errors.New("clientID 为空，无法查找用户ID")
	}

	log.Infof(ctx, "SaveConversation 保存对话: sessionID=%s, userID=%v, clientID=%s", sessionID, foundUserID, clientID)
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
		log.Errorf(ctx, "保存对话失败: %v", err)
		return err
	}

	// 更新或创建会话
	if err := s.dao.UpdateOrCreateSession(ctx, sessionID, deviceID, clientID, foundUserID, combotName, message); err != nil {
		log.Errorf(ctx, "更新会话失败: %v", err)
		return err
	}

	return nil
}

// GetUserConversations 获取当前用户所有机器人的对话历史（左侧列表）
func (s *ConversationServiceImpl) GetUserConversations(ctx context.Context, userID uint, deviceID string, limit int, offset int) ([]models.ConversationSession, int, error) {
	log.Infof(ctx, "GetUserConversations 获取用户对话列表: userID=%d, deviceID=%s, limit=%d, offset=%d", userID, deviceID, limit, offset)
	sessions, err := s.dao.GetUserConversations(ctx, userID, deviceID, limit, offset)
	if err != nil {
		log.Errorf(ctx, "获取用户对话列表失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	return sessions, codes.CodeSuccess, nil
}

// GetConversationMessages 根据用户、机器人和sessionID获取详细对话历史（右侧对话内容）
func (s *ConversationServiceImpl) GetConversationMessages(ctx context.Context, sessionID string, limit int, offset int) ([]models.ConversationMessage, int, error) {
	log.Infof(ctx, "GetConversationMessages 获取对话详情: sessionID=%s, limit=%d, offset=%d", sessionID, limit, offset)
	messages, err := s.dao.GetConversationMessages(ctx, sessionID, limit, offset)
	if err != nil {
		log.Errorf(ctx, "获取对话详情失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	return messages, codes.CodeSuccess, nil
}
