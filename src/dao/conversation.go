package dao

import (
	"context"
	"fmt"
	"time"

	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/models"

	"gorm.io/gorm"
)

// ConversationDAO 对话数据访问对象
type ConversationDAO struct {
	base *BaseDAO
}

// NewConversationDAO 创建对话DAO实例
func NewConversationDAO() *ConversationDAO {
	return &ConversationDAO{
		base: NewBaseDAO(),
	}
}

// SaveMessages 批量保存对话消息
func (dao *ConversationDAO) SaveMessages(ctx context.Context, messages []models.ConversationMessage) error {
	return dao.base.CreateInBatches(ctx, messages, 100)
}

// GetUserConversations 获取用户的对话会话列表
func (dao *ConversationDAO) GetUserConversations(ctx context.Context, userID uint, deviceID string, limit, offset int) ([]models.ConversationSession, error) {
	var sessions []models.ConversationSession

	options := QueryOptions{
		OrWhere: []map[string]interface{}{
			{"user_id": userID},
			{"device_id": deviceID},
		},
		OrderBy: []string{"last_activity DESC"},
		Limit:   limit,
		Offset:  offset,
	}

	err := dao.base.Query(ctx, &sessions, options)
	return sessions, err
}

// GetConversationMessages 获取会话的消息列表
func (dao *ConversationDAO) GetConversationMessages(ctx context.Context, sessionID string, limit, offset int) ([]models.ConversationMessage, error) {
	var messages []models.ConversationMessage

	options := QueryOptions{
		Where: map[string]interface{}{
			"session_id": sessionID,
		},
		OrderBy: []string{"created_at ASC"},
		Limit:   limit,
		Offset:  offset,
	}

	err := dao.base.Query(ctx, &messages, options)
	return messages, err
}

// UpdateOrCreateSession 更新会话或创建新会话（原子操作）
func (dao *ConversationDAO) UpdateOrCreateSession(ctx context.Context, sessionID, deviceID, clientID string, userID *uint, currentRole, firstUserMessage string) error {
	return dao.base.Transaction(ctx, func(tx *gorm.DB) error {
		now := time.Now()

		// 尝试更新现有会话
		result := tx.Model(&models.ConversationSession{}).
			Where("session_id = ?", sessionID).
			Updates(map[string]interface{}{
				"last_activity": now,
				"current_role":  currentRole,
				"message_count": gorm.Expr("message_count + 2"), // 用户+AI两条消息
			})

		// 如果会话不存在，创建新会话
		if result.RowsAffected == 0 {
			// 生成会话标题（取用户第一条消息的前20个字符）
			title := firstUserMessage
			if len(title) > 20 {
				title = title[:20] + "..."
			}

			session := models.ConversationSession{
				SessionID:    sessionID,
				DeviceID:     deviceID,
				ClientID:     clientID,
				UserID:       userID,
				Title:        title,
				CurrentRole:  currentRole,
				StartTime:    now,
				LastActivity: now,
				Status:       "active",
				MessageCount: 2, // 第一轮对话就是2条消息
				CreatedAt:    now,
				UpdatedAt:    now,
			}

			if err := tx.Create(&session).Error; err != nil {
				utils.Error(ctx, fmt.Sprintf("创建新会话失败: %v", err))
				return fmt.Errorf("创建新会话失败: %w", err)
			} else {
				utils.Debug(ctx, fmt.Sprintf("创建新会话成功: sessionID=%s, title=%s", sessionID, title))
			}
		}

		return nil
	})
}
