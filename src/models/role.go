package models

// RoleTemplate 角色模板结构
type RoleTemplate struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	AssistantName   string `json:"assistant_name"`
	DefaultVoice    string `json:"default_voice"`
	DefaultLanguage string `json:"default_language"`
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
