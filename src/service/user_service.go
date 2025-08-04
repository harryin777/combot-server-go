package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserServiceImpl struct {
	config *configs.Config
}

// NewUserService 创建用户服务
func NewUserService(config *configs.Config) UserService {
	return &UserServiceImpl{config: config}
}

// UsernamePasswordLogin 用户名密码登录
func (s *UserServiceImpl) UsernamePasswordLogin(ctx context.Context, username, password string) (*models.User, string, error) {
	// 记录登录尝试
	utils.Infof(ctx, "用户尝试用户名密码登录，用户名: %s", username)

	var user models.User
	err := database.DB.WithContext(ctx).Where("username = ?", username).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.Warnf(ctx, "用户名不存在: %s", username)
		return nil, "", errors.New("invalid username or password")
	} else if err != nil {
		utils.Errorf(ctx, "数据库查询用户失败: %v", err)
		return nil, "", fmt.Errorf("database error: %w", err)
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		utils.Warnf(ctx, "密码验证失败，用户名: %s", username)
		return nil, "", errors.New("invalid username or password")
	}

	utils.Infof(ctx, "用户名密码登录成功，用户ID: %d", user.ID)

	// 生成JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"iat":      time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(s.config.Server.Token))
	if err != nil {
		utils.Errorf(ctx, "生成JWT token失败: %v", err)
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	utils.Infof(ctx, "用户名密码认证完成，用户ID: %d", user.ID)
	return &user, tokenString, nil
}

// UpdateUserProfile 更新用户基本信息（用户名、手机号）
func (s *UserServiceImpl) UpdateUserProfile(ctx context.Context, userID int64, username, phone string) error {
	utils.Infof(ctx, "更新用户基本信息，用户ID: %d", userID)

	// 检查用户名是否已存在（排除当前用户）
	if username != "" {
		var existingUser models.User
		err := database.DB.WithContext(ctx).Where("username = ? AND id != ?", username, userID).First(&existingUser).Error
		if err == nil {
			return errors.New("用户名已存在")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Errorf(ctx, "检查用户名唯一性失败: %v", err)
			return fmt.Errorf("database error: %w", err)
		}
	}

	// 检查手机号是否已存在（排除当前用户）
	if phone != "" {
		var existingUser models.User
		err := database.DB.WithContext(ctx).Where("phone = ? AND id != ?", phone, userID).First(&existingUser).Error
		if err == nil {
			return errors.New("手机号已存在")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Errorf(ctx, "检查手机号唯一性失败: %v", err)
			return fmt.Errorf("database error: %w", err)
		}
	}

	// 构建更新数据
	updateData := make(map[string]interface{})
	if username != "" {
		updateData["username"] = username
	}
	if phone != "" {
		updateData["phone"] = phone
	}

	if len(updateData) == 0 {
		return errors.New("没有提供要更新的数据")
	}

	// 执行更新
	err := database.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Updates(updateData).Error
	if err != nil {
		utils.Errorf(ctx, "更新用户信息失败: %v", err)
		return fmt.Errorf("failed to update user profile: %w", err)
	}

	utils.Infof(ctx, "用户基本信息更新成功，用户ID: %d", userID)
	return nil
}

// ChangePassword 修改密码
func (s *UserServiceImpl) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) error {
	utils.Infof(ctx, "用户修改密码，用户ID: %d", userID)

	// 获取用户当前信息
	var user models.User
	err := database.DB.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("用户不存在")
	} else if err != nil {
		utils.Errorf(ctx, "查询用户信息失败: %v", err)
		return fmt.Errorf("database error: %w", err)
	}

	// 验证旧密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword))
	if err != nil {
		utils.Warnf(ctx, "旧密码验证失败，用户ID: %d", userID)
		return errors.New("旧密码不正确")
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.Errorf(ctx, "加密新密码失败: %v", err)
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// 更新密码
	err = database.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("password", string(hashedPassword)).Error
	if err != nil {
		utils.Errorf(ctx, "更新密码失败: %v", err)
		return fmt.Errorf("failed to update password: %w", err)
	}

	// 使所有现有会话失效（强制重新登录）
	err = database.DB.WithContext(ctx).Model(&models.UserSession{}).Where("user_id = ?", userID).Update("is_active", false).Error
	if err != nil {
		utils.Warnf(ctx, "使现有会话失效失败: %v", err)
		// 这不是致命错误，继续执行
	}

	utils.Infof(ctx, "密码修改成功，用户ID: %d", userID)
	return nil
}

// GetLoginDevices 获取登录设备列表
func (s *UserServiceImpl) GetLoginDevices(ctx context.Context, userID int64) ([]LoginDevice, error) {
	utils.Infof(ctx, "获取用户登录设备列表，用户ID: %d", userID)

	var sessions []models.UserSession
	err := database.DB.WithContext(ctx).Where("user_id = ? AND is_active = ? AND expires_at > ?", userID, true, time.Now()).
		Order("created_at DESC").Find(&sessions).Error
	if err != nil {
		utils.Errorf(ctx, "查询登录会话失败: %v", err)
		return nil, fmt.Errorf("failed to query login sessions: %w", err)
	}

	devices := make([]LoginDevice, len(sessions))
	for i, session := range sessions {
		devices[i] = LoginDevice{
			UserAgent:  session.UserAgent,
			IP:         session.IP,
			LastLogin:  session.CreatedAt.Format("2006/1/2 15:04:05"),
			DeviceType: session.DeviceType,
		}
	}

	utils.Infof(ctx, "获取到 %d 个活跃登录设备，用户ID: %d", len(devices), userID)
	return devices, nil
}

// LogoutDevice 退出登录指定设备
func (s *UserServiceImpl) LogoutDevice(ctx context.Context, userID int64, deviceIdentifier string) error {
	utils.Infof(ctx, "退出登录设备，用户ID: %d, 设备标识: %s", userID, deviceIdentifier)

	// deviceIdentifier 可以是 IP 或者 UserAgent 的一部分
	err := database.DB.WithContext(ctx).Model(&models.UserSession{}).
		Where("user_id = ? AND (ip = ? OR user_agent LIKE ?)", userID, deviceIdentifier, "%"+deviceIdentifier+"%").
		Update("is_active", false).Error

	if err != nil {
		utils.Errorf(ctx, "退出登录设备失败: %v", err)
		return fmt.Errorf("failed to logout device: %w", err)
	}

	utils.Infof(ctx, "设备退出登录成功，用户ID: %d", userID)
	return nil
}

// DeleteAccount 删除账号
func (s *UserServiceImpl) DeleteAccount(ctx context.Context, userID int64, password string) error {
	utils.Infof(ctx, "用户请求删除账号，用户ID: %d", userID)

	// 获取用户信息并验证密码
	var user models.User
	err := database.DB.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("用户不存在")
	} else if err != nil {
		utils.Errorf(ctx, "查询用户信息失败: %v", err)
		return fmt.Errorf("database error: %w", err)
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		utils.Warnf(ctx, "删除账号密码验证失败，用户ID: %d", userID)
		return errors.New("密码不正确")
	}

	// 开启事务删除相关数据
	err = database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除用户会话
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserSession{}).Error; err != nil {
			return err
		}

		// 删除用户设置
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserSetting{}).Error; err != nil {
			return err
		}

		// 删除用户
		if err := tx.Delete(&models.User{}, userID).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		utils.Errorf(ctx, "删除账号失败: %v", err)
		return fmt.Errorf("failed to delete account: %w", err)
	}

	utils.Infof(ctx, "账号删除成功，用户ID: %d", userID)
	return nil
}
