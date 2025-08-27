package models

import (
	"time"
)

// DeviceVerificationCode 设备验证码临时存储
type DeviceVerificationCode struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	SerialNumber     string    `gorm:"index;size:64" json:"serial_number"`          // 设备序列号
	DeviceID         string    `gorm:"index;size:17" json:"device_id"`              // MAC地址
	ClientID         string    `gorm:"index;size:36" json:"client_id"`              // UUID
	VerificationCode string    `gorm:"uniqueIndex;size:6" json:"verification_code"` // 6位验证码
	ExpiresAt        time.Time `gorm:"index" json:"expires_at"`                     // 过期时间
	Used             bool      `gorm:"default:false" json:"used"`                   // 是否已使用
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName 设置表名
func (DeviceVerificationCode) TableName() string {
	return "device_verification_codes"
}

// IsExpired 检查验证码是否过期
func (dvc *DeviceVerificationCode) IsExpired() bool {
	return time.Now().After(dvc.ExpiresAt)
}
