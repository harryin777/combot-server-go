package models

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Device represents a device in the system.
type Device struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	SerialNumber      string     `gorm:"uniqueIndex;size:64" json:"serial_number"`
	DeviceID          string     `gorm:"index;size:17" json:"device_id"` // MAC地址
	ClientID          string     `gorm:"index;size:36" json:"client_id"` // UUID
	Token             string     `gorm:"size:256" json:"token"`
	ActivationCode    string     `gorm:"size:32" json:"activation_code"`
	Challenge         string     `gorm:"size:64" json:"challenge"`
	ActivationVersion int        `gorm:"default:1" json:"activation_version"`
	Activated         bool       `gorm:"default:false" json:"activated"`
	ActivatedAt       *time.Time `json:"activated_at"`
	LastSeen          time.Time  `gorm:"autoUpdateTime" json:"last_seen"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// TableName ...
func (Device) TableName() string {
	return "devices"
}

// GenerateActivationCode 生成随机激活码
func GenerateActivationCode() string {
	const digits = "0123456789"
	b := make([]byte, 6)
	rand.Read(b)

	code := make([]byte, 6)
	for i := range b {
		code[i] = digits[b[i]%10]
	}
	return string(code)
}

// GenerateChallenge 生成随机challenge
func GenerateChallenge() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateToken 生成JWT token
func GenerateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// VerifyHMAC 验证HMAC
func (d *Device) VerifyHMAC(challenge, hmacHex, hmacKey string) bool {
	// 这里需要使用与ESP32相同的HMAC密钥
	// 实际项目中应该从安全的地方获取密钥
	mac := hmac.New(sha256.New, []byte(hmacKey))
	mac.Write([]byte(challenge))
	expectedMAC := mac.Sum(nil)
	expectedHex := hex.EncodeToString(expectedMAC)

	return hmac.Equal([]byte(hmacHex), []byte(expectedHex))
}
