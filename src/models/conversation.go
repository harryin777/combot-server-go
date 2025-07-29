package models

import (
	"time"

	"gorm.io/datatypes"
)

// ConversationHistory 对话历史记录
type ConversationHistory struct {
	ID          int64          `json:"id" gorm:"primaryKey;autoIncrement;column:id;comment:对话记录ID"`
	DeviceID    string         `json:"device_id" gorm:"column:device_id;type:varchar(100);index;not null;comment:设备ID"`
	ClientID    string         `json:"client_id" gorm:"column:client_id;type:varchar(100);index;comment:客户端ID"`
	UserID      *uint          `json:"user_id" gorm:"column:user_id;index;comment:关联用户ID（可选）"`
	SessionID   string         `json:"session_id" gorm:"column:session_id;type:varchar(100);index;not null;comment:会话ID"`
	Role        string         `json:"role" gorm:"column:role;type:varchar(20);not null;comment:消息角色（user/assistant/system）"`
	Content     string         `json:"content" gorm:"column:content;type:text;not null;comment:消息内容"`
	MessageType string         `json:"message_type" gorm:"column:message_type;type:varchar(20);default:'text';comment:消息类型（text/image/audio）"`
	AIRole      string         `json:"ai_role" gorm:"column:ai_role;type:varchar(50);default:'';comment:当前AI角色（小智/英语老师/陕西女友等）"`
	Round       int            `json:"round" gorm:"column:round;not null;default:0;comment:对话轮次"`
	Metadata    datatypes.JSON `json:"metadata" gorm:"column:metadata;type:json;comment:额外元数据（如图片信息、音频时长等）"`
	CreatedAt   time.Time      `json:"created_at" gorm:"column:created_at;comment:创建时间"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"column:updated_at;comment:更新时间"`
}

func (ConversationHistory) TableName() string {
	return "conversation_history"
}

// ConversationSession 对话会话信息
type ConversationSession struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement;column:id;comment:会话ID"`
	SessionID    string    `json:"session_id" gorm:"column:session_id;type:varchar(100);uniqueIndex;not null;comment:会话标识"`
	DeviceID     string    `json:"device_id" gorm:"column:device_id;type:varchar(100);index;not null;comment:设备ID"`
	ClientID     string    `json:"client_id" gorm:"column:client_id;type:varchar(100);index;comment:客户端ID"`
	UserID       *uint     `json:"user_id" gorm:"column:user_id;index;comment:关联用户ID（可选）"`
	CurrentRole  string    `json:"current_role" gorm:"column:current_role;type:varchar(50);default:'';comment:当前AI角色"`
	StartTime    time.Time `json:"start_time" gorm:"column:start_time;not null;comment:会话开始时间"`
	LastActivity time.Time `json:"last_activity" gorm:"column:last_activity;not null;comment:最后活动时间"`
	Status       string    `json:"status" gorm:"column:status;type:varchar(20);default:'active';comment:会话状态（active/closed）"`
	MessageCount int       `json:"message_count" gorm:"column:message_count;default:0;comment:消息总数"`
	CreatedAt    time.Time `json:"created_at" gorm:"column:created_at;comment:创建时间"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"column:updated_at;comment:更新时间"`
}

func (ConversationSession) TableName() string {
	return "conversation_sessions"
}
