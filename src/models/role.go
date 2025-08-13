package models

import (
	"time"
)

// RoleTemplate 角色模板结构 - 用于数据库存储
type RoleTemplate struct {
	ID              string    `json:"id" gorm:"primaryKey;type:varchar(100);comment:角色模板ID"`
	Name            string    `json:"name" gorm:"type:varchar(100);not null;comment:角色模板名称"`
	Description     string    `json:"description" gorm:"type:text;comment:角色描述"`
	AssistantName   string    `json:"assistant_name" gorm:"type:varchar(100);not null;comment:助手名称"`
	DefaultVoice    string    `json:"default_voice" gorm:"type:varchar(100);not null;comment:默认音色"`
	DefaultLanguage string    `json:"default_language" gorm:"type:varchar(50);not null;comment:默认语言"`
	Enabled         bool      `json:"enabled" gorm:"type:tinyint(1);not null;default:1;comment:是否启用"`
	SortOrder       int       `json:"sort_order" gorm:"type:int;not null;default:0;comment:排序序号"`
	CreatedAt       time.Time `json:"created_at" gorm:"autoCreateTime;comment:创建时间"`
	UpdatedAt       time.Time `json:"updated_at" gorm:"autoUpdateTime;comment:更新时间"`
}

func (RoleTemplate) TableName() string {
	return "role_templates"
}

// RoleConfig 角色配置结构
type RoleConfig struct {
	ID                   int64   `json:"id" gorm:"primaryKey"`
	UserID               int64   `json:"user_id" gorm:"not null"`
	DeviceID             string  `json:"device_id" gorm:"not null"`
	TemplateID           string  `json:"template_id" gorm:"not null"`
	AssistantName        string  `json:"assistant_name" gorm:"not null"`
	ConversationLanguage string  `json:"conversation_language" gorm:"not null"`
	RoleDescription      string  `json:"role_description" gorm:"type:text"`
	VoiceModel           string  `json:"voice_model" gorm:"not null"`
	CurrentMemory        string  `json:"current_memory" gorm:"type:text"`
	DetailedMemory       string  `json:"detailed_memory" gorm:"type:text"`
	Temperature          float64 `json:"temperature"`
	MaxLength            int     `json:"max_length"`
	CreatedAt            int64   `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt            int64   `json:"updated_at" gorm:"autoUpdateTime"`
}

// SaveRoleConfigRequest 保存角色配置请求
type SaveRoleConfigRequest struct {
	DeviceID             string  `json:"device_id"`
	TemplateID           string  `json:"template_id"`
	AssistantName        string  `json:"assistant_name"`
	ConversationLanguage string  `json:"conversation_language"`
	RoleDescription      string  `json:"role_description"`
	VoiceModel           string  `json:"voice_model"`
	CurrentMemory        string  `json:"current_memory"`
	DetailedMemory       string  `json:"detailed_memory"`
	Temperature          float64 `json:"temperature"`
	MaxLength            int     `json:"max_length"`
}
