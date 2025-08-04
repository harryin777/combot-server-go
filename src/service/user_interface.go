package service

import (
	"context"
	"xiaozhi-server-go/src/models"
)

// LoginDevice 表示登录设备信息
type LoginDevice struct {
	UserAgent  string `json:"user_agent"`
	IP         string `json:"ip"`
	LastLogin  string `json:"last_login"`
	DeviceType string `json:"device_type"` // macOS-Chrome, Windows-Chrome等
}

// UserService 定义用户服务接口
type UserService interface {
	// UsernamePasswordLogin 用户名密码登录
	UsernamePasswordLogin(ctx context.Context, username, password string) (*models.User, string, error)

	// UpdateUserProfile 更新用户基本信息（用户名、手机号）
	UpdateUserProfile(ctx context.Context, userID int64, username, phone string) error

	// ChangePassword 修改密码
	ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error

	// GetLoginDevices 获取登录设备列表
	GetLoginDevices(ctx context.Context, userID int64) ([]LoginDevice, error)

	// LogoutDevice 退出登录指定设备
	LogoutDevice(ctx context.Context, userID int64, deviceIdentifier string) error

	// DeleteAccount 删除账号
	DeleteAccount(ctx context.Context, userID int64, password string) error
}
