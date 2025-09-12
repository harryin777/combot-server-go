package service

import (
	"combot-server-go/src/models"
	"context"
)

// AuthService 定义认证服务接口
type AuthService interface {
	GetCaptcha(ctx context.Context, width, height int) (id, imageBase64 string, code int, err error)
	SendSMS(ctx context.Context, countryCode, phone, captchaID, captchaValue string) (interface{}, int, error)
	PhoneAuth(ctx context.Context, countryCode, phone, smsCode string) (*models.User, string, int, error)
	SendEmailVerification(ctx context.Context, email string) (interface{}, int, error)
	VerifyEmail(ctx context.Context, email, verificationCode string) (interface{}, int, error)
}
