package models

import (
	"time"

	"gorm.io/datatypes"
)

// ConversationSession 对话会话（左侧会话列表）
// 对应界面左侧的会话列表，每个会话是一个独立的对话主题
type ConversationSession struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement;column:id;comment:会话自增ID"`
	SessionID    string    `json:"session_id" gorm:"column:session_id;type:varchar(100);uniqueIndex;not null;comment:会话唯一标识"`
	DeviceID     string    `json:"device_id" gorm:"column:device_id;type:varchar(100);index;not null;comment:设备ID"`
	ClientID     string    `json:"client_id" gorm:"column:client_id;type:varchar(100);index;comment:客户端ID"`
	UserID       *uint     `json:"user_id" gorm:"column:user_id;index;comment:关联用户ID（可选）"`
	Title        string    `json:"title" gorm:"column:title;type:varchar(200);default:'';comment:会话标题（显示在左侧列表）"`
	CurrentRole  string    `json:"current_role" gorm:"column:current_role;type:varchar(50);default:'小智';comment:当前AI角色"`
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

// ConversationMessage 对话消息详情（右侧对话内容）
// 对应界面右侧的具体对话内容，包含用户和AI的所有消息
type ConversationMessage struct {
	ID          int64          `json:"id" gorm:"primaryKey;autoIncrement;column:id;comment:消息自增ID"`
	SessionID   string         `json:"session_id" gorm:"column:session_id;type:varchar(100);index;not null;comment:所属会话ID"`
	Role        string         `json:"role" gorm:"column:role;type:varchar(20);not null;comment:消息角色（user/assistant/system）"`
	Content     string         `json:"content" gorm:"column:content;type:text;not null;comment:消息内容"`
	MessageType string         `json:"message_type" gorm:"column:message_type;type:varchar(20);default:'text';comment:消息类型（text/image/audio）"`
	Round       int            `json:"round" gorm:"column:round;not null;default:0;comment:对话轮次（一问一答为一轮）"`
	Metadata    datatypes.JSON `json:"metadata" gorm:"column:metadata;type:json;comment:额外元数据（如图片信息、音频时长等）"`
	CreatedAt   time.Time      `json:"created_at" gorm:"column:created_at;comment:创建时间"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"column:updated_at;comment:更新时间"`
}

func (ConversationMessage) TableName() string {
	return "conversation_messages"
}
