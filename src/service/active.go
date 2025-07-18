package service

import (
	"errors"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/auth"
	"xiaozhi-server-go/src/models"

	"gorm.io/gorm"
)

type DeviceService struct {
	config *configs.Config
}

// NewDevice 创建一个新的 Device 实例
func NewDevice(config *configs.Config) *DeviceService {
	return &DeviceService{
		config: config,
	}
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

// GetDeviceByID 通过设备ID查询设备
func (s *DeviceService) GetDeviceByID(deviceID uint) (*models.Device, error) {
	var device models.Device
	if err := database.DB.Where("id = ?", deviceID).First(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
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

	// 从配置文件读取HMAC密钥
	hmacKey := s.config.Server.Device.HmacKey
	if hmacKey == "" {
		return errors.New("HMAC key not configured")
	}

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

// ActivateDeviceAndGetToken 激活设备并获取JWT token
func (s *DeviceService) ActivateDeviceAndGetToken(deviceID uint, challenge, hmacHex string) (string, error) {
	var device models.Device
	if err := database.DB.Where("id = ?", deviceID).First(&device).Error; err != nil {
		return "", err
	}

	// 验证challenge是否匹配
	if device.Challenge != challenge {
		return "", errors.New("invalid challenge")
	}

	// 从配置文件读取HMAC密钥
	hmacKey := s.config.Server.Device.HmacKey
	if hmacKey == "" {
		return "", errors.New("HMAC key not configured")
	}

	if !device.VerifyHMAC(challenge, hmacHex, hmacKey) {
		return "", errors.New("invalid HMAC")
	}

	// 激活设备
	now := time.Now()
	if err := database.DB.Model(&device).Updates(map[string]interface{}{
		"activated":    true,
		"activated_at": &now,
		"last_seen":    now,
	}).Error; err != nil {
		return "", err
	}

	// 生成JWT token
	authToken := auth.NewAuthToken(s.config.Server.Token)
	token, err := authToken.GenerateToken(device.DeviceID)
	if err != nil {
		return "", err
	}

	return token, nil
}

// GetDeviceToken 获取设备访问token
func (s *DeviceService) GetDeviceToken(deviceID, clientID, challenge, hmacHex string) (string, error) {
	// 根据设备ID或客户端ID查找设备
	device, err := s.IdentifyDevice("", deviceID, clientID)
	if err != nil {
		return "", err
	}

	// 检查设备是否已激活
	if !device.Activated {
		return "", errors.New("device not activated")
	}

	// 验证challenge是否匹配
	if device.Challenge != challenge {
		return "", errors.New("invalid challenge")
	}

	// 从配置文件读取HMAC密钥
	hmacKey := s.config.Server.Device.HmacKey
	if hmacKey == "" {
		return "", errors.New("HMAC key not configured")
	}

	if !device.VerifyHMAC(challenge, hmacHex, hmacKey) {
		return "", errors.New("invalid HMAC")
	}

	// 更新最后访问时间
	now := time.Now()
	database.DB.Model(device).Update("last_seen", now)

	// 生成JWT token
	authToken := auth.NewAuthToken(s.config.Server.Token)
	token, err := authToken.GenerateToken(device.DeviceID)
	if err != nil {
		return "", err
	}

	return token, nil
}
