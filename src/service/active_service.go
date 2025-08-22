package service

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/configs/database"
	"combot-server-go/src/core/auth"
	"combot-server-go/src/core/codes"
	"combot-server-go/src/core/utils"
	"combot-server-go/src/models"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type DeviceService struct {
	config *configs.Config
}

// NewDevice 创建一个新的设备服务实例
func NewDevice(config *configs.Config) ActiveService {
	return &DeviceService{
		config: config,
	}
}

// IdentifyDevice 根据请求头识别设备
func (s *DeviceService) IdentifyDevice(ctx context.Context, serialNumber, deviceID, clientID string) (*models.Device, int, error) {
	var device models.Device

	// 优先使用序列号查找
	if serialNumber != "" {
		err := database.DB.WithContext(ctx).Where("serial_number = ?", serialNumber).First(&device).Error
		if err == nil {
			return &device, codes.CodeSuccess, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Errorf(ctx, "查询设备失败: %v", err)
			return nil, codes.CodeInternalError, err
		}
	}

	// 备用MAC地址查找
	if deviceID != "" {
		err := database.DB.WithContext(ctx).Where("device_id = ?", deviceID).First(&device).Error
		if err == nil {
			return &device, codes.CodeSuccess, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Errorf(ctx, "查询设备失败: %v", err)
			return nil, codes.CodeInternalError, err
		}
	}

	// 最后使用UUID查找
	if clientID != "" {
		err := database.DB.WithContext(ctx).Where("client_id = ?", clientID).First(&device).Error
		if err == nil {
			return &device, codes.CodeSuccess, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Errorf(ctx, "查询设备失败: %v", err)
			return nil, codes.CodeInternalError, err
		}
	}

	return nil, codes.CodeDeviceNotFound, nil
}

// GenerateDeviceVerificationCode 生成设备验证码并确保设备记录存在
func (s *DeviceService) GenerateDeviceVerificationCode(ctx context.Context, deviceID, clientID string) (string, int64, int, error) {
	// 生成6位数字验证码
	code := s.GenerateActivationCode(ctx)
	expiresAt := time.Now().Add(5 * time.Minute) // 5分钟有效期

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// 清理该设备的旧验证码
		tx.Where("device_id = ? OR client_id = ?", deviceID, clientID).
			Delete(&models.DeviceVerificationCode{})

		// 确保设备记录存在（如果不存在则创建，但不激活）
		var device models.Device
		err := tx.Where("device_id = ? OR client_id = ?", deviceID, clientID).
			First(&device).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建未激活的设备记录
			device = models.Device{
				DeviceID:   deviceID,
				ClientID:   clientID,
				UserID:     nil,   // 尚未绑定用户
				DeviceName: "",    // 尚未命名
				Activated:  false, // 尚未激活
				Token:      "",    // 尚无token
				LastSeen:   time.Now(),
			}

			if err := tx.Create(&device).Error; err != nil {
				utils.Errorf(ctx, "创建设备记录失败: %v", err)
				return err
			}
		} else if err != nil {
			utils.Errorf(ctx, "查询设备失败: %v", err)
			return err
		}

		// 存储新验证码
		verificationCode := &models.DeviceVerificationCode{
			DeviceID:         deviceID,
			ClientID:         clientID,
			VerificationCode: code,
			ExpiresAt:        expiresAt,
			Used:             false,
		}

		if err := tx.Create(verificationCode).Error; err != nil {
			utils.Errorf(ctx, "保存验证码失败: %v", err)
			return err
		}

		return nil
	})

	if err != nil {
		utils.Errorf(ctx, "生成设备验证码失败: %v", err)
		return "", 0, codes.CodeInternalError, err
	}

	// 验证码直接在HTTP响应中返回给智能体，智能体收到后会立即播报给用户
	utils.Info(nil, fmt.Sprintf("为设备 %s 生成验证码: %s，有效期至: %s",
		deviceID, code, expiresAt.Format("2006-01-02 15:04:05")))

	return code, expiresAt.Unix(), codes.CodeSuccess, nil
}

// ValidateVerificationCode 验证验证码
func (s *DeviceService) ValidateVerificationCode(ctx context.Context, code string) (*models.DeviceVerificationCode, int, error) {
	var verificationCode models.DeviceVerificationCode

	err := database.DB.WithContext(ctx).Where("verification_code = ? AND used = false", code).
		First(&verificationCode).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Warnf(ctx, "验证码不存在或已使用: %s", code)
			return nil, codes.CodeInvalidRequest, nil
		}
		utils.Errorf(ctx, "查询验证码失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	// 检查验证码是否过期
	if verificationCode.IsExpired() {
		utils.Warnf(ctx, "验证码已过期: %s", code)
		return nil, codes.CodeInvalidRequest, nil
	}

	return &verificationCode, codes.CodeSuccess, nil
}

// ActivateDevice 激活设备
func (s *DeviceService) ActivateDevice(ctx context.Context, deviceID uint, challenge, hmac string) (interface{}, int, error) {
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var device models.Device
		if err := tx.Where("id = ?", deviceID).First(&device).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				utils.Warnf(ctx, "设备不存在: %d", deviceID)
				return fmt.Errorf("设备不存在: %w", err)
			}
			utils.Errorf(ctx, "查询设备失败: %v", err)
			return fmt.Errorf("设备不存在: %w", err)
		}

		// 在生产环境中，这里应该验证HMAC签名
		if !s.VerifyHMAC(ctx, challenge, hmac, s.config.Server.Device.HmacKey) {
			return errors.New("HMAC验证失败")
		}

		// 生成新的Token
		authToken := auth.NewAuthToken(s.config.Server.Token)
		token, err := authToken.GenerateToken(device.DeviceID)
		if err != nil {
			utils.Errorf(ctx, "生成Token失败: %v", err)
			return fmt.Errorf("生成Token失败: %w", err)
		}

		// 更新设备激活状态
		now := time.Now()
		updates := map[string]interface{}{
			"activated":          true,
			"activated_at":       &now,
			"token":              token,
			"challenge":          challenge,
			"activation_version": device.ActivationVersion + 1,
			"last_seen":          now,
		}

		if err := tx.Model(&device).Updates(updates).Error; err != nil {
			utils.Errorf(ctx, "更新设备激活状态失败: %v", err)
			return fmt.Errorf("更新设备激活状态失败: %w", err)
		}

		utils.Infof(ctx, "设备 %s 激活成功", device.DeviceID)
		return nil
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, codes.CodeDeviceNotFound, nil
		}
		return nil, codes.CodeInternalError, err
	}

	return nil, codes.CodeSuccess, nil
}

// BindDeviceToUser 绑定设备到用户
func (s *DeviceService) BindDeviceToUser(ctx context.Context, userID uint, verificationCode, deviceName string) (
	*models.Device, int, error) {
	// 验证验证码
	vcRecord, code, err := s.ValidateVerificationCode(ctx, verificationCode)
	if err != nil {
		utils.Errorf(ctx, "验证验证码失败: %v", err)
		return nil, codes.CodeInternalError, err
	}
	if code != codes.CodeSuccess {
		return nil, code, nil
	}

	var device models.Device
	err = database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 标记验证码为已使用
		if err := tx.Model(vcRecord).Update("used", true).Error; err != nil {
			utils.Errorf(ctx, "更新验证码状态失败: %v", err)
			return fmt.Errorf("更新验证码状态失败: %w", err)
		}

		// 查找或创建设备
		err := tx.Where("device_id = ? OR client_id = ?", vcRecord.DeviceID, vcRecord.ClientID).
			First(&device).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 创建新设备
			device = models.Device{
				DeviceID:    vcRecord.DeviceID,
				ClientID:    vcRecord.ClientID,
				UserID:      &userID,
				DeviceName:  deviceName,
				Activated:   true,
				ActivatedAt: &[]time.Time{time.Now()}[0],
				Token:       s.GenerateToken(ctx),
				LastSeen:    time.Now(),
			}

			if err := tx.Create(&device).Error; err != nil {
				utils.Errorf(ctx, "创建设备失败: %v", err)
				return fmt.Errorf("创建设备失败: %w", err)
			}
		} else if err != nil {
			utils.Errorf(ctx, "查询设备失败: %v", err)
			return fmt.Errorf("查询设备失败: %w", err)
		} else {
			// 更新现有设备
			now := time.Now()
			updates := map[string]interface{}{
				"user_id":      &userID,
				"device_name":  deviceName,
				"activated":    true,
				"activated_at": &now,
				"last_seen":    now,
			}

			if err := tx.Model(&device).Updates(updates).Error; err != nil {
				utils.Errorf(ctx, "更新设备失败: %v", err)
				return fmt.Errorf("更新设备失败: %w", err)
			}
		}

		utils.Info(nil, fmt.Sprintf("设备 %s 成功绑定到用户 %d", device.DeviceID, userID))
		return nil
	})

	if err != nil {
		utils.Errorf(ctx, "绑定设备到用户失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	return &device, codes.CodeSuccess, nil
}

// GetUserDevices 获取用户设备列表
func (s *DeviceService) GetUserDevices(ctx context.Context, userID uint) ([]models.Device, int, error) {
	var devices []models.Device
	err := database.DB.WithContext(ctx).Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&devices).Error

	if err != nil {
		utils.Errorf(ctx, "查询用户设备列表失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	return devices, codes.CodeSuccess, nil
}

// VerifyHMAC 验证HMAC签名
func (s *DeviceService) VerifyHMAC(ctx context.Context, challenge, hmacHex, hmacKey string) bool {
	mac := hmac.New(sha256.New, []byte(hmacKey))
	mac.Write([]byte(challenge))
	expectedMAC := mac.Sum(nil)
	expectedHex := hex.EncodeToString(expectedMAC)

	return hmac.Equal([]byte(hmacHex), []byte(expectedHex))
}

// GenerateActivationCode 生成6位数字激活码
func (s *DeviceService) GenerateActivationCode(ctx context.Context) string {
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
func (s *DeviceService) GenerateChallenge(ctx context.Context) string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateToken 生成随机token
func (s *DeviceService) GenerateToken(ctx context.Context) string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
