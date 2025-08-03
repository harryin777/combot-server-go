package service

import (
	"xiaozhi-server-go/src/models"
)

// ActiveService 定义设备激活服务接口
type ActiveService interface {
	// 设备识别和查找
	IdentifyDevice(serialNumber, deviceID, clientID string) (*models.Device, error)

	// 验证码管理
	GenerateDeviceVerificationCode(deviceID, clientID string) (string, int64, error) // 生成设备验证码
	ValidateVerificationCode(code string) (*models.DeviceVerificationCode, error)    // 验证验证码

	// 设备激活管理
	ActivateDevice(deviceID uint, challenge, hmac string) error                                // 激活设备
	BindDeviceToUser(userID uint, verificationCode, deviceName string) (*models.Device, error) // 绑定设备到用户

	// 设备查询
	GetUserDevices(userID uint) ([]models.Device, error) // 获取用户设备列表

	// 安全相关
	VerifyHMAC(challenge, hmacHex, hmacKey string) bool // 验证HMAC
	GenerateActivationCode() string                     // 生成激活码
	GenerateChallenge() string                          // 生成随机挑战码
	GenerateToken() string                              // 生成Token
}
