package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mojocn/base64Captcha"
	"gorm.io/gorm"
)

// 内存存储短信验证码（生产环境应使用Redis）
var smsStore = map[string]string{}

type AuthService struct {
	config *configs.Config
}

// NewAuthService 创建认证服务
func NewAuthService(config *configs.Config) *AuthService {
	return &AuthService{config: config}
}

// GetCaptcha 生成图形验证码
func (s *AuthService) GetCaptcha(width, height int) (id, imageBase64 string, err error) {
	driver := base64Captcha.NewDriverDigit(height, width, 6, 0.7, 80)
	cp := base64Captcha.NewCaptcha(driver, base64Captcha.DefaultMemStore)
	id, imageBase64, _, err = cp.Generate()
	return
}

// SendSMS 校验图形码并下发短信验证码（模拟实现）
func (s *AuthService) SendSMS(countryCode, phone, captchaID, captchaValue string) error {
	// 验证图形验证码
	if !base64Captcha.DefaultMemStore.Verify(captchaID, captchaValue, true) {
		return errors.New("invalid captcha")
	}

	// 生成6位数字验证码
	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	key := countryCode + phone
	smsStore[key] = code

	// TODO: 接入真正的短信服务提供商
	// 这里仅模拟发送成功
	return nil
}

// PhoneAuth 手机号登录/注册，返回用户信息和JWT token
func (s *AuthService) PhoneAuth(countryCode, phone, smsCode string) (*models.User, string, error) {
	key := countryCode + phone
	ctx := context.Background()

	// 记录登录尝试
	utils.Infof(ctx, "用户尝试登录，手机号: %s", key)

	// 验证短信验证码
	if storedCode, exists := smsStore[key]; !exists || storedCode != smsCode {
		utils.Warnf(ctx, "短信验证码验证失败，手机号: %s", key)
		return nil, "", errors.New("invalid sms code")
	}

	// 删除已使用的验证码
	delete(smsStore, key)

	var user models.User
	err := database.DB.Where("phone = ?", key).First(&user).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 用户不存在，自动注册
		utils.Infof(ctx, "新用户注册，手机号: %s", key)
		user = models.User{
			Phone: key,
			Role:  models.UserRoleUser, // 使用枚举值：普通用户
		}
		if err = database.DB.Create(&user).Error; err != nil {
			utils.Errorf(ctx, "创建用户失败: %v", err)
			return nil, "", fmt.Errorf("failed to create user: %w", err)
		}
		utils.Infof(ctx, "用户注册成功，用户ID: %d", user.ID)
	} else if err != nil {
		utils.Errorf(ctx, "数据库查询用户失败: %v", err)
		return nil, "", fmt.Errorf("database error: %w", err)
	}

	utils.Infof(ctx, "用户登录成功，用户ID: %d", user.ID)

	// 生成JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"phone":   user.Phone,
		"iat":     time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(s.config.Server.Token))
	if err != nil {
		utils.Errorf(ctx, "生成JWT token失败: %v", err)
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	utils.Infof(ctx, "用户认证完成，用户ID: %d", user.ID)
	return &user, tokenString, nil
}
