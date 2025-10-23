package service

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/configs/database"
	"combot-server-go/src/core/codes"
	"combot-server-go/src/log"
	"combot-server-go/src/models"
	"context"
	"errors"
	"time"

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
func (s *UserServiceImpl) UsernamePasswordLogin(ctx context.Context, username, password string) (*models.User, string, int, error) {
	log.Infof(ctx, "[UsernamePasswordLogin] username: %s", username)
	// 记录登录尝试
	log.Infof(ctx, "用户尝试用户名密码登录，用户名: %s", username)

	var user models.User
	err := database.DB.WithContext(ctx).Where("username = ?", username).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Warnf(ctx, "用户名不存在: %s", username)
		return nil, "", codes.CodeInvalidUsernamePassword, nil
	} else if err != nil {
		log.Errorf(ctx, "数据库查询用户失败: %v", err)
		return nil, "", codes.CodeInternalError, err
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		s, err := bcrypt.GenerateFromPassword([]byte(password), 8)
		if err != nil {
			log.Errorf(ctx, "密码哈希生成失败: %v", err)
			return nil, "", codes.CodeInternalError, err
		}
		log.Infof(ctx, "用户密码哈希: %s", s)
		log.Warnf(ctx, "密码验证失败，用户名: %s", username)
		return nil, "", codes.CodeInvalidUsernamePassword, nil
	}

	log.Infof(ctx, "用户名密码登录成功，用户ID: %d", user.ID)

	// 生成JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"iat":      time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(s.config.Server.Token))
	if err != nil {
		log.Errorf(ctx, "生成JWT token失败: %v", err)
		return nil, "", codes.CodeInternalError, err
	}

	log.Infof(ctx, "用户名密码认证完成，用户ID: %d", user.ID)
	return &user, tokenString, codes.CodeSuccess, nil
}

// UsernamePasswordRegister 用户名密码注册
func (s *UserServiceImpl) UsernamePasswordRegister(ctx context.Context, username, password, email string) (*models.User, string, int, error) {
	log.Infof(ctx, "[UsernamePasswordRegister] username: %s, email: %s", username, email)
	log.Infof(ctx, "用户尝试用户名密码注册，用户名: %s, 邮箱: %s", username, email)

	// 检查用户名是否已存在
	var existingUser models.User
	err := database.DB.WithContext(ctx).Where("username = ?", username).First(&existingUser).Error
	if err == nil {
		log.Warnf(ctx, "用户名已存在: %s", username)
		return nil, "", codes.CodeUsernameExists, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf(ctx, "检查用户名唯一性失败: %v", err)
		return nil, "", codes.CodeInternalError, err
	}

	// 检查邮箱是否已存在（仅当邮箱不为空时检查）
	if email != "" {
		err = database.DB.WithContext(ctx).Where("email = ?", email).First(&existingUser).Error
		if err == nil {
			log.Warnf(ctx, "邮箱已存在: %s", email)
			return nil, "", codes.CodeEmailExists, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf(ctx, "检查邮箱唯一性失败: %v", err)
			return nil, "", codes.CodeInternalError, err
		}
	}

	// 生成密码哈希
	hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Errorf(ctx, "密码哈希生成失败: %v", err)
		return nil, "", codes.CodeInternalError, err
	}
	hashedPassword := string(hashedPasswordBytes)

	// 创建新用户
	user := models.User{
		Username: username,
		Email:    email,
		Password: hashedPassword,
		Role:     models.UserRoleUser, // 使用枚举值：普通用户
	}

	if err = database.DB.WithContext(ctx).Create(&user).Error; err != nil {
		log.Errorf(ctx, "创建用户失败: %v", err)
		return nil, "", codes.CodeInternalError, err
	}

	log.Infof(ctx, "用户注册成功，用户ID: %d", user.ID)

	// 生成JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"iat":      time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(s.config.Server.Token))
	if err != nil {
		log.Errorf(ctx, "生成JWT token失败: %v", err)
		return nil, "", codes.CodeInternalError, err
	}

	log.Infof(ctx, "用户注册认证完成，用户ID: %d", user.ID)
	return &user, tokenString, codes.CodeSuccess, nil
}

// GetUserByID 根据用户ID获取用户信息
func (s *UserServiceImpl) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	log.Infof(ctx, "[GetUserByID] userID: %d", userID)
	log.Infof(ctx, "获取用户信息，用户ID: %d", userID)

	var user models.User
	err := database.DB.WithContext(ctx).First(&user, userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf(ctx, "用户不存在，用户ID: %d", userID)
			return nil, errors.New("用户不存在")
		}
		log.Errorf(ctx, "查询用户信息失败: %v", err)
		return nil, err
	}

	return &user, nil
}

// UpdateUserProfile 更新用户基本信息（用户名、手机号、邮箱、备注）
func (s *UserServiceImpl) UpdateUserProfile(ctx context.Context, userID int64, username, phone, email, remark string) (interface{}, int, error) {
	log.Infof(ctx, "[UpdateUserProfile] userID: %d, username: %s, phone: %s, email: %s", userID, username, phone, email)
	log.Infof(ctx, "更新用户基本信息，用户ID: %d", userID)

	// 检查用户名是否已存在（排除当前用户）
	if username != "" {
		var existingUser models.User
		err := database.DB.WithContext(ctx).Where("username = ? AND id != ?", username, userID).First(&existingUser).Error
		if err == nil {
			return nil, codes.CodeUsernameExists, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf(ctx, "检查用户名唯一性失败: %v", err)
			return nil, codes.CodeInternalError, err
		}
	}

	// 检查手机号是否已存在（排除当前用户）- 手机号现在可以为空
	if phone != "" {
		var existingUser models.User
		err := database.DB.WithContext(ctx).Where("phone = ? AND id != ?", phone, userID).First(&existingUser).Error
		if err == nil {
			return nil, codes.CodePhoneExists, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf(ctx, "检查手机号唯一性失败: %v", err)
			return nil, codes.CodeInternalError, err
		}
	}

	// 检查邮箱是否已存在（排除当前用户）- 邮箱可以为空
	if email != "" {
		var existingUser models.User
		err := database.DB.WithContext(ctx).Where("email = ? AND id != ?", email, userID).First(&existingUser).Error
		if err == nil {
			return nil, codes.CodeEmailExists, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf(ctx, "检查邮箱唯一性失败: %v", err)
			return nil, codes.CodeInternalError, err
		}
	}

	// 构建更新数据
	updateData := make(map[string]interface{})
	if username != "" {
		updateData["username"] = username
	}
	// 手机号允许为空，如果传入空字符串则清空
	updateData["phone"] = phone
	// 邮箱允许为空，如果传入空字符串则清空
	updateData["email"] = email
	// 备注允许为空
	updateData["remark"] = remark

	// 用户名是必须的，如果为空则报错
	if username == "" {
		return nil, codes.CodeInvalidRequest, errors.New("用户名不能为空")
	}

	// 执行更新
	err := database.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Updates(updateData).Error
	if err != nil {
		log.Errorf(ctx, "更新用户信息失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	log.Infof(ctx, "用户基本信息更新成功，用户ID: %d", userID)
	return nil, codes.CodeSuccess, nil
}

// ChangePassword 修改用户密码
func (s *UserServiceImpl) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword string) (interface{}, int, error) {
	log.Infof(ctx, "[ChangePassword] userID: %d", userID)
	log.Infof(ctx, "修改用户密码，用户ID: %d", userID)

	// 获取用户信息
	var user models.User
	err := database.DB.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, codes.CodeUserNotFound, nil
		}
		log.Errorf(ctx, "查询用户失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	// 验证旧密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword))
	if err != nil {
		log.Infof(ctx, "用户密码验证失败，用户ID: %d", userID)
		return nil, codes.CodeInvalidOldPassword, nil
	}

	// 生成新密码哈希
	hashedPasswordBytes, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Errorf(ctx, "密码哈希生成失败: %v", err)
		return nil, codes.CodeInternalError, err
	}
	hashedPassword := string(hashedPasswordBytes)

	// 更新密码
	err = database.DB.WithContext(ctx).Model(&user).Update("password", hashedPassword).Error
	if err != nil {
		log.Errorf(ctx, "更新用户密码失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	log.Infof(ctx, "用户密码修改成功，用户ID: %d", userID)
	return nil, codes.CodeSuccess, nil
}

// GetLoginDevices 获取用户登录设备列表
func (s *UserServiceImpl) GetLoginDevices(ctx context.Context, userID int64) ([]LoginDevice, int, error) {
	log.Infof(ctx, "[GetLoginDevices] userID: %d", userID)
	log.Infof(ctx, "获取用户登录设备列表，用户ID: %d", userID)

	// 这里需要实现从实际的登录会话表或类似的地方获取设备信息
	// 暂时返回空列表，实际实现需要根据业务需求来设计
	var devices []LoginDevice

	// TODO: 实现实际的设备查询逻辑
	// 可能需要从 JWT sessions, 登录记录表等地方获取设备信息

	log.Infof(ctx, "成功获取用户登录设备列表，用户ID: %d，设备数量: %d", userID, len(devices))
	return devices, codes.CodeSuccess, nil
}

// LogoutDevice 退出登录指定设备
func (s *UserServiceImpl) LogoutDevice(ctx context.Context, userID int64, deviceIdentifier string) (interface{}, int, error) {
	log.Infof(ctx, "[LogoutDevice] userID: %d, deviceIdentifier: %s", userID, deviceIdentifier)
	log.Infof(ctx, "退出登录指定设备，用户ID: %d，设备标识: %s", userID, deviceIdentifier)

	// TODO: 实现实际的设备退出逻辑
	// 这可能包括：
	// 1. 使对应设备的 JWT token 失效
	// 2. 更新设备状态为非活跃
	// 3. 清理相关的会话信息

	// 暂时返回成功，实际实现需要根据业务需求来设计
	log.Infof(ctx, "设备退出登录成功，用户ID: %d，设备标识: %s", userID, deviceIdentifier)
	return nil, codes.CodeSuccess, nil
}

// DeleteAccount 删除用户账号
func (s *UserServiceImpl) DeleteAccount(ctx context.Context, userID int64, password string) (interface{}, int, error) {
	log.Infof(ctx, "[DeleteAccount] userID: %d", userID)
	log.Infof(ctx, "删除用户账号，用户ID: %d", userID)

	// 获取用户信息
	var user models.User
	err := database.DB.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, codes.CodeUserNotFound, nil
		}
		log.Errorf(ctx, "查询用户失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		log.Infof(ctx, "用户密码验证失败，用户ID: %d", userID)
		return nil, codes.CodeInvalidUsernamePassword, nil
	}

	// 执行软删除（或硬删除，根据业务需求）
	err = database.DB.WithContext(ctx).Delete(&user).Error
	if err != nil {
		log.Errorf(ctx, "删除用户账号失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	// TODO: 清理用户相关的数据
	// 1. 删除或归档用户的对话记录
	// 2. 清理用户的设备关联
	// 3. 使所有相关的 JWT token 失效
	// 4. 清理其他用户相关数据

	log.Infof(ctx, "用户账号删除成功，用户ID: %d", userID)
	return nil, codes.CodeSuccess, nil
}
