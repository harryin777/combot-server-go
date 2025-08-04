package service

import (
	"context"
	"xiaozhi-server-go/src/models"
)

// AuthService 定义认证服务接口
type AuthService interface {
	GetCaptcha(ctx context.Context, width, height int) (id, imageBase64 string, err error)
	SendSMS(ctx context.Context, countryCode, phone, captchaID, captchaValue string) error
	PhoneAuth(ctx context.Context, countryCode, phone, smsCode string) (*models.User, string, error)
}
