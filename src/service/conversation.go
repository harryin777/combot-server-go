package service

import (
	"context"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"time"

	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/chat"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/models"

	"gorm.io/gorm"
)

// ConversationService 对话历史服务
type ConversationService struct {
}

// NewConversationService 创建对话历史服务实例
func NewConversationService() *ConversationService {
	return &ConversationService{}
}

// SaveMessage 保存单条消息到历史记录
func (s *ConversationService) SaveMessage(ctx context.Context, sessionID, deviceID, clientID string, userID *uint, message chat.Message, aiRole string, round int, messageType string, metadata map[string]interface{}) error {
	// 准备元数据
	var metadataJSON []byte
	if metadata != nil {
		var err error
		metadataJSON, err = jsoniter.Marshal(metadata)
		if err != nil {
			utils.Error(context.Background(), fmt.Sprintf("序列化元数据失败: %v", err))
			metadataJSON = []byte("{}")
		}
	}

	// 创建历史记录
	history := models.ConversationHistory{
		DeviceID:    deviceID,
		ClientID:    clientID,
		UserID:      userID,
		SessionID:   sessionID,
		Role:        message.Role,
		Content:     message.Content,
		MessageType: messageType,
		AIRole:      aiRole,
		Round:       round,
		Metadata:    metadataJSON,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 保存到数据库
	if err := database.DB.Create(&history).Error; err != nil {
		utils.Error(ctx, fmt.Sprintf("保存对话历史失败: %v", err))
		return fmt.Errorf("保存对话历史失败: %w", err)
	}

	// 更新会话活动时间和消息计数
	s.updateSessionActivity(sessionID, deviceID, clientID, userID, aiRole)

	utils.Debug(ctx, fmt.Sprintf("保存对话历史成功: sessionID=%s, role=%s, round=%d", sessionID, message.Role, round))
	return nil
}

// SaveConversationBatch 批量保存对话历史
func (s *ConversationService) SaveConversationBatch(ctx context.Context, sessionID, deviceID, clientID string, userID *uint, messages []chat.Message, aiRole string, round int) error {
	histories := make([]models.ConversationHistory, 0, len(messages))
	now := time.Now()

	for _, message := range messages {
		history := models.ConversationHistory{
			DeviceID:    deviceID,
			ClientID:    clientID,
			UserID:      userID,
			SessionID:   sessionID,
			Role:        message.Role,
			Content:     message.Content,
			MessageType: "text",
			AIRole:      aiRole,
			Round:       round,
			Metadata:    []byte("{}"),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		histories = append(histories, history)
	}

	// 批量插入
	if err := database.DB.CreateInBatches(histories, 100).Error; err != nil {
		utils.Error(context.Background(), fmt.Sprintf("批量保存对话历史失败: %v", err))
		return fmt.Errorf("批量保存对话历史失败: %w", err)
	}

	// 更新会话活动时间
	s.updateSessionActivity(sessionID, deviceID, clientID, userID, aiRole)

	utils.Debug(context.Background(), fmt.Sprintf("批量保存对话历史成功: sessionID=%s, messages=%d, round=%d", sessionID, len(messages), round))
	return nil
}

// GetConversationHistory 获取指定会话的对话历史
func (s *ConversationService) GetConversationHistory(ctx context.Context, sessionID string, limit int, offset int) ([]models.ConversationHistory, error) {
	var histories []models.ConversationHistory

	query := database.DB.Where("session_id = ?", sessionID).
		Order("created_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&histories).Error; err != nil {
		utils.Error(context.Background(), fmt.Sprintf("获取对话历史失败: %v", err))
		return nil, fmt.Errorf("获取对话历史失败: %w", err)
	}

	return histories, nil
}

// GetRecentConversationHistory 获取最近的对话历史（用于加载到DialogueManager）
func (s *ConversationService) GetRecentConversationHistory(ctx context.Context, sessionID string, maxMessages int) ([]chat.Message, error) {
	var histories []models.ConversationHistory

	err := database.DB.Where("session_id = ?", sessionID).
		Order("created_at DESC").
		Limit(maxMessages).
		Find(&histories).Error

	if err != nil {
		utils.Error(context.Background(), fmt.Sprintf("获取最近对话历史失败: %v", err))
		return nil, fmt.Errorf("获取最近对话历史失败: %w", err)
	}

	// 转换为chat.Message格式，并反向排序（最旧的在前）
	messages := make([]chat.Message, 0, len(histories))
	for i := len(histories) - 1; i >= 0; i-- {
		history := histories[i]
		messages = append(messages, chat.Message{
			Role:    history.Role,
			Content: history.Content,
		})
	}

	return messages, nil
}

// GetConversationsByDevice 获取设备的所有会话
func (s *ConversationService) GetConversationsByDevice(ctx context.Context, deviceID string, limit int, offset int) ([]models.ConversationSession, error) {
	var sessions []models.ConversationSession

	query := database.DB.Where("device_id = ?", deviceID).
		Order("last_activity DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&sessions).Error; err != nil {
		utils.Error(context.Background(), fmt.Sprintf("获取设备会话列表失败: %v", err))
		return nil, fmt.Errorf("获取设备会话列表失败: %w", err)
	}

	return sessions, nil
}

// UpdateSessionRole 更新会话当前角色
func (s *ConversationService) UpdateSessionRole(ctx context.Context, sessionID string, newRole string) error {
	err := database.DB.Model(&models.ConversationSession{}).
		Where("session_id = ?", sessionID).
		Update("current_role", newRole).Error

	if err != nil {
		utils.Error(context.Background(), fmt.Sprintf("更新会话角色失败: %v", err))
		return fmt.Errorf("更新会话角色失败: %w", err)
	}

	utils.Debug(context.Background(), fmt.Sprintf("更新会话角色成功: sessionID=%s, role=%s", sessionID, newRole))
	return nil
}

// CloseSession 关闭会话
func (s *ConversationService) CloseSession(ctx context.Context, sessionID string) error {
	err := database.DB.Model(&models.ConversationSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"status":        "closed",
			"last_activity": time.Now(),
		}).Error

	if err != nil {
		utils.Error(context.Background(), fmt.Sprintf("关闭会话失败: %v", err))
		return fmt.Errorf("关闭会话失败: %w", err)
	}

	utils.Debug(context.Background(), fmt.Sprintf("关闭会话成功: sessionID=%s", sessionID))
	return nil
}

// updateSessionActivity 更新会话活动时间和消息计数（内部方法）
func (s *ConversationService) updateSessionActivity(sessionID, deviceID, clientID string, userID *uint, currentRole string) {
	now := time.Now()

	// 尝试更新现有会话
	result := database.DB.Model(&models.ConversationSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]interface{}{
			"last_activity": now,
			"current_role":  currentRole,
			"message_count": gorm.Expr("message_count + 1"),
		})

	// 如果会话不存在，创建新会话
	if result.RowsAffected == 0 {
		session := models.ConversationSession{
			SessionID:    sessionID,
			DeviceID:     deviceID,
			ClientID:     clientID,
			UserID:       userID,
			CurrentRole:  currentRole,
			StartTime:    now,
			LastActivity: now,
			Status:       "active",
			MessageCount: 1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := database.DB.Create(&session).Error; err != nil {
			utils.Error(context.Background(), fmt.Sprintf("创建新会话失败: %v", err))
		} else {
			utils.Debug(context.Background(), fmt.Sprintf("创建新会话成功: sessionID=%s", sessionID))
		}
	}
}
