package service

import (
	"combot-server-go/src/models"
	"context"
)

// ActiveService 定义设备激活服务接口
type ActiveService interface {
	// IdentifyDevice 设备识别和查找
	IdentifyDevice(ctx context.Context, serialNumber, deviceID, clientID string) (*models.Device, int, error)

	// GenerateDeviceVerificationCode 验证码管理
	GenerateDeviceVerificationCode(ctx context.Context, serialNumber, deviceID, clientID string) (string, int64, int, error) // 生成设备验证码
	ValidateVerificationCode(ctx context.Context, code string) (*models.DeviceVerificationCode, int, error)                  // 验证验证码

	// ActivateDevice 设备激活管理
	ActivateDevice(ctx context.Context, deviceID uint, challenge, hmac string) (interface{}, int, error)                 // 激活设备
	BindDeviceToUser(ctx context.Context, userID uint, verificationCode, deviceName string) (*models.Device, int, error) // 绑定设备到用户

	// GetUserDevices 设备查询
	GetUserDevices(ctx context.Context, userID uint) ([]models.Device, int, error) // 获取用户设备列表

	// VerifyHMAC 安全相关
	VerifyHMAC(ctx context.Context, challenge, hmacHex, hmacKey string) bool // 验证HMAC
	GenerateActivationCode(ctx context.Context) string                       // 生成激活码
	GenerateChallenge(ctx context.Context) string                            // 生成随机挑战码
	GenerateToken(ctx context.Context) string                                // 生成Token
}
