package service

import (
	"errors"
	"gorm.io/gorm"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/models"
)

type DeviceService struct {
}

// NewDevice 创建一个新的 Device 实例
func NewDevice(config *configs.Config) *DeviceService {
	return &DeviceService{}
}

// IdentifyDevice 根据请求头识别设备
func (s *DeviceService) IdentifyDevice(serialNumber, deviceID, clientID string) (*models.Device, error) {
	var device models.Device

	// 优先使用序列号查找
	if serialNumber != "" {
		err := database.DB.Where("serial_number = ?", serialNumber).First(&device).Error
		if err == nil {
			return &device, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	// 备用MAC地址查找
	if deviceID != "" {
		err := database.DB.Where("device_id = ?", deviceID).First(&device).Error
		if err == nil {
			return &device, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	// 最后使用UUID查找
	if clientID != "" {
		err := database.DB.Where("client_id = ?", clientID).First(&device).Error
		if err == nil {
			return &device, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	return nil, gorm.ErrRecordNotFound
}

// CreateOrUpdateDevice 创建或更新设备
func (s *DeviceService) CreateOrUpdateDevice(serialNumber, deviceID, clientID string, activationVersion int) (*models.Device, error) {
	device, err := s.IdentifyDevice(serialNumber, deviceID, clientID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if device == nil {
		// 创建新设备
		device = &models.Device{
			SerialNumber:      serialNumber,
			DeviceID:          deviceID,
			ClientID:          clientID,
			ActivationVersion: activationVersion,
			ActivationCode:    models.GenerateActivationCode(),
			Challenge:         models.GenerateChallenge(),
			Token:             models.GenerateToken(),
			Activated:         false,
		}

		if err := database.DB.Create(device).Error; err != nil {
			return nil, err
		}
	} else {
		// 更新现有设备信息
		updates := map[string]interface{}{
			"device_id":          deviceID,
			"client_id":          clientID,
			"activation_version": activationVersion,
			"last_seen":          time.Now(),
		}

		// 如果设备未激活，更新挑战和激活码
		if !device.Activated {
			updates["activation_code"] = models.GenerateActivationCode()
			updates["challenge"] = models.GenerateChallenge()
		}

		if err := database.DB.Model(device).Updates(updates).Error; err != nil {
			return nil, err
		}

		// 重新查询更新后的设备
		if err := database.DB.Where("id = ?", device.ID).First(device).Error; err != nil {
			return nil, err
		}
	}

	return device, nil
}

// ActivateDevice 激活设备
func (s *DeviceService) ActivateDevice(deviceID uint, challenge, hmacHex string) error {
	var device models.Device
	if err := database.DB.Where("id = ?", deviceID).First(&device).Error; err != nil {
		return err
	}

	// 验证challenge是否匹配
	if device.Challenge != challenge {
		return errors.New("invalid challenge")
	}

	// 验证HMAC (这里需要配置正确的HMAC密钥)
	hmacKey := "your-hmac-key-here" // 实际项目中应该从配置文件或环境变量获取
	if !device.VerifyHMAC(challenge, hmacHex, hmacKey) {
		return errors.New("invalid HMAC")
	}

	// 激活设备
	now := time.Now()
	return database.DB.Model(&device).Updates(map[string]interface{}{
		"activated":    true,
		"activated_at": &now,
		"last_seen":    now,
	}).Error
}
