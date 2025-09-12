package models

import (
	"time"
	//"gorm.io/gorm"
	"gorm.io/datatypes"
)

// 用户角色枚举
const (
	UserRoleUser  int8 = 1 // 普通用户
	UserRoleAdmin int8 = 2 // 管理员
)

// SystemConfig 系统全局配置（只保存一条记录）
type SystemConfig struct {
	ID               int64          `json:"id" gorm:"primaryKey;autoIncrement;column:id;comment:主键ID"`
	SelectedASR      string         `json:"selected_asr" gorm:"column:selected_asr;type:varchar(100);not null;default:'';comment:选中的ASR服务"`
	SelectedTTS      string         `json:"selected_tts" gorm:"column:selected_tts;type:varchar(100);not null;default:'';comment:选中的TTS服务"`
	SelectedLLM      string         `json:"selected_llm" gorm:"column:selected_llm;type:varchar(100);not null;default:'';comment:选中的LLM服务"`
	SelectedVLLLM    string         `json:"selected_vlllm" gorm:"column:selected_vlllm;type:varchar(100);not null;default:'';comment:选中的VLLLM服务"`
	Prompt           string         `json:"prompt" gorm:"column:prompt;type:text;comment:系统提示词"`
	QuickReplyWords  datatypes.JSON `json:"quick_reply_words" gorm:"column:quick_reply_words;type:json;comment:快速回复词列表"`
	DeleteAudio      bool           `json:"delete_audio" gorm:"column:delete_audio;type:tinyint(1);not null;default:0;comment:是否删除音频文件"`
	UsePrivateConfig bool           `json:"use_private_config" gorm:"column:use_private_config;type:tinyint(1);not null;default:0;comment:是否使用私有配置"`
}

func (SystemConfig) TableName() string {
	return "system_config"
}

// User 用户
type User struct {
	ID       int64       `json:"id" gorm:"primaryKey;autoIncrement;column:id;comment:用户ID"`
	Phone    string      `json:"phone" gorm:"column:phone;type:varchar(20);uniqueIndex;comment:手机号"`
	Email    string      `json:"email" gorm:"column:email;type:varchar(255);uniqueIndex;comment:邮箱"`
	Username string      `json:"username" gorm:"column:username;type:varchar(50);uniqueIndex;comment:用户名"`
	Password string      `json:"-" gorm:"column:password;type:varchar(255);comment:密码哈希"`
	Role     int8        `json:"role" gorm:"column:role;type:tinyint;not null;default:1;comment:用户角色（1=用户，2=管理员）"`
	Remark   string      `json:"remark" gorm:"column:remark;type:text;comment:备注"`
	Setting  UserSetting `json:"setting" gorm:"foreignKey:UserID;references:ID"`
}

func (User) TableName() string {
	return "users"
}

// UserSetting 用户设置
type UserSetting struct {
	ID              int64          `json:"id" gorm:"primaryKey;autoIncrement;column:id;comment:设置ID"`
	UserID          int64          `json:"user_id" gorm:"column:user_id;uniqueIndex;not null;comment:用户ID（一对一关联）"`
	SelectedASR     string         `json:"selected_asr" gorm:"column:selected_asr;type:varchar(100);not null;default:'';comment:选中的ASR服务"`
	SelectedTTS     string         `json:"selected_tts" gorm:"column:selected_tts;type:varchar(100);not null;default:'';comment:选中的TTS服务"`
	SelectedLLM     string         `json:"selected_llm" gorm:"column:selected_llm;type:varchar(100);not null;default:'';comment:选中的LLM服务"`
	SelectedVLLLM   string         `json:"selected_vlllm" gorm:"column:selected_vlllm;type:varchar(100);not null;default:'';comment:选中的VLLLM服务"`
	PromptOverride  string         `json:"prompt_override" gorm:"column:prompt_override;type:text;comment:用户自定义提示词"`
	QuickReplyWords datatypes.JSON `json:"quick_reply_words" gorm:"column:quick_reply_words;type:json;comment:用户快速回复词列表"`
}

func (UserSetting) TableName() string {
	return "user_settings"
}

// ModuleConfig 模块配置（可选）
type ModuleConfig struct {
	ID          int64          `json:"id" gorm:"primaryKey;autoIncrement;column:id;comment:配置ID"`
	Name        string         `json:"name" gorm:"column:name;type:varchar(100);uniqueIndex;not null;comment:模块名"`
	Type        string         `json:"type" gorm:"column:type;type:varchar(50);not null;default:'';comment:模块类型"`
	ConfigJSON  datatypes.JSON `json:"config_json" gorm:"column:config_json;type:json;comment:配置JSON数据"`
	Public      bool           `json:"public" gorm:"column:public;type:tinyint(1);not null;default:0;comment:是否公开"`
	Description string         `json:"description" gorm:"column:description;type:varchar(500);not null;default:'';comment:模块描述"`
	Enabled     bool           `json:"enabled" gorm:"column:enabled;type:tinyint(1);not null;default:1;comment:是否启用"`
}

func (ModuleConfig) TableName() string {
	return "module_configs"
}

// UserSession 用户登录会话
type UserSession struct {
	ID         int64     `json:"id" gorm:"primaryKey;autoIncrement;column:id;comment:会话ID"`
	UserID     int64     `json:"user_id" gorm:"column:user_id;not null;index;comment:用户ID"`
	Token      string    `json:"token" gorm:"column:token;type:varchar(500);uniqueIndex;not null;comment:JWT Token"`
	UserAgent  string    `json:"user_agent" gorm:"column:user_agent;type:varchar(500);not null;default:'';comment:用户代理"`
	IP         string    `json:"ip" gorm:"column:ip;type:varchar(45);not null;default:'';comment:IP地址"`
	DeviceType string    `json:"device_type" gorm:"column:device_type;type:varchar(100);not null;default:'';comment:设备类型"`
	CreatedAt  time.Time `json:"created_at" gorm:"column:created_at;not null;comment:创建时间"`
	ExpiresAt  time.Time `json:"expires_at" gorm:"column:expires_at;not null;comment:过期时间"`
	IsActive   bool      `json:"is_active" gorm:"column:is_active;type:tinyint(1);not null;default:1;comment:是否活跃"`
}

func (UserSession) TableName() string {
	return "user_sessions"
}
