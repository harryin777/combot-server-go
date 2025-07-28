package service

import (
	"errors"
	"fmt"
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

// GenerateDeviceVerificationCode 为设备生成验证码
func (s *DeviceService) GenerateDeviceVerificationCode(deviceID, clientID string) (string, int, error) {
	// 清理过期的验证码
	database.DB.Where("expires_at < ?", time.Now()).Delete(&models.DeviceVerificationCode{})

	// 检查是否已存在未过期的验证码
	var existingCode models.DeviceVerificationCode
	err := database.DB.Where("device_id = ? AND client_id = ? AND used = false AND expires_at > ?",
		deviceID, clientID, time.Now()).First(&existingCode).Error

	if err == nil {
		// 返回现有未过期的验证码
		expiresIn := int(time.Until(existingCode.ExpiresAt).Seconds())
		return existingCode.VerificationCode, expiresIn, nil
	}

	// 生成新的6位验证码
	verificationCode := models.GenerateActivationCode()
	expiresAt := time.Now().Add(10 * time.Minute) // 10分钟过期

	// 存储验证码
	deviceVerification := models.DeviceVerificationCode{
		DeviceID:         deviceID,
		ClientID:         clientID,
		VerificationCode: verificationCode,
		ExpiresAt:        expiresAt,
		Used:             false,
	}

	if err := database.DB.Create(&deviceVerification).Error; err != nil {
		return "", 0, fmt.Errorf("failed to create verification code: %w", err)
	}

	expiresIn := int(time.Until(expiresAt).Seconds())
	return verificationCode, expiresIn, nil
}

// BindDeviceToUser 验证验证码并绑定设备到用户
func (s *DeviceService) BindDeviceToUser(userID uint, verificationCode, deviceName string) (*models.Device, error) {
	// 查找验证码
	var deviceVerification models.DeviceVerificationCode
	err := database.DB.Where("verification_code = ? AND used = false AND expires_at > ?",
		verificationCode, time.Now()).First(&deviceVerification).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid or expired verification code")
		}
		return nil, fmt.Errorf("failed to find verification code: %w", err)
	}

	// 标记验证码为已使用
	deviceVerification.Used = true
	if err := database.DB.Save(&deviceVerification).Error; err != nil {
		return nil, fmt.Errorf("failed to mark verification code as used: %w", err)
	}

	// 查找或创建设备
	var device models.Device
	err = database.DB.Where("device_id = ? AND client_id = ?",
		deviceVerification.DeviceID, deviceVerification.ClientID).First(&device).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建新设备
		device = models.Device{
			DeviceID:          deviceVerification.DeviceID,
			ClientID:          deviceVerification.ClientID,
			UserID:            &userID,
			DeviceName:        deviceName,
			ActivationVersion: 1,
			ActivationCode:    models.GenerateActivationCode(),
			Challenge:         models.GenerateChallenge(),
			Token:             models.GenerateToken(),
			Activated:         true,
			ActivatedAt:       &time.Time{},
		}
		now := time.Now()
		device.ActivatedAt = &now

		if err := database.DB.Create(&device).Error; err != nil {
			return nil, fmt.Errorf("failed to create device: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to query device: %w", err)
	} else {
		// 更新现有设备
		updates := map[string]interface{}{
			"user_id":     userID,
			"device_name": deviceName,
			"activated":   true,
		}
		if !device.Activated {
			now := time.Now()
			updates["activated_at"] = &now
		}

		if err := database.DB.Model(&device).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("failed to update device: %w", err)
		}

		// 重新查询以获取更新后的数据
		if err := database.DB.Where("id = ?", device.ID).First(&device).Error; err != nil {
			return nil, fmt.Errorf("failed to reload device: %w", err)
		}
	}

	return &device, nil
}

// GetUserDevices 获取用户的设备列表
func (s *DeviceService) GetUserDevices(userID uint) ([]models.Device, error) {
	var devices []models.Device
	err := database.DB.Where("user_id = ?", userID).Find(&devices).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get user devices: %w", err)
	}
	return devices, nil
}
