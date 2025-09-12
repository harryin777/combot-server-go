package handlers

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/codes"
	"combot-server-go/src/core/response"
	"combot-server-go/src/log"
	"combot-server-go/src/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(config *configs.Config) *AuthHandler {
	return &AuthHandler{
		authService: service.NewAuthService(config),
	}
}

// CaptchaRequest 获取验证码请求
type CaptchaRequest struct {
	Width  int `form:"width"`
	Height int `form:"height"`
}

// CaptchaResponse 验证码响应
type CaptchaResponse struct {
	CaptchaID   string `json:"captcha_id"`
	ImageBase64 string `json:"image_base64"`
}

// SMSRequest 发送短信请求
type SMSRequest struct {
	CountryCode  string `json:"country_code" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	CaptchaID    string `json:"captcha_id" binding:"required"`
	CaptchaValue string `json:"captcha_value" binding:"required"`
}

// SMSResponse 短信发送响应
type SMSResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// PhoneAuthRequest 手机号登录请求
type PhoneAuthRequest struct {
	CountryCode string `json:"country_code" binding:"required"`
	Phone       string `json:"phone" binding:"required"`
	SMSCode     string `json:"sms_code" binding:"required"`
}

// PhoneAuthResponse 手机号登录响应
type PhoneAuthResponse struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}

// GetCaptcha @Summary Get captcha image
// @Description 获取图形验证码
// @Tags Auth
// @Param width query int false "验证码宽度" default(150)
// @Param height query int false "验证码高度" default(40)
// @Produce application/json
// @Success 200 {object} CaptchaResponse
// @Failure 500 {object} map[string]string
// @Router /api/v1/captcha/image [get]
func (h *AuthHandler) GetCaptcha(c *gin.Context) {
	var req CaptchaRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	// 设置默认值
	if req.Width <= 0 {
		req.Width = 150
	}
	if req.Height <= 0 {
		req.Height = 40
	}

	id, img, code, err := h.authService.GetCaptcha(c.Request.Context(), req.Width, req.Height)
	if err != nil {
		log.WithError(c.Request.Context(), err).Error("Failed to generate captcha")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, CaptchaResponse{
		CaptchaID:   id,
		ImageBase64: img,
	})
}

// SendSMS @Summary Send SMS verification code
// @Description 发送短信验证码
// @Tags Auth
// @Accept application/json
// @Param request body SMSRequest true "短信发送请求"
// @Produce application/json
// @Success 200 {object} SMSResponse
// @Failure 400 {object} map[string]string
// @Router /api/v1/sms/send [post]
func (h *AuthHandler) SendSMS(c *gin.Context) {
	var req SMSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.WithError(c.Request.Context(), err).Error("Invalid SMS request")
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	_, code, err := h.authService.SendSMS(c.Request.Context(), req.CountryCode, req.Phone, req.CaptchaID, req.CaptchaValue)
	if err != nil {
		log.WithError(c.Request.Context(), err).Error("Failed to send SMS")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, SMSResponse{
		Success: true,
		Message: "SMS sent successfully",
	})
}

// PhoneAuth @Summary Phone number login/register
// @Description 手机号登录或注册
// @Tags Auth
// @Accept application/json
// @Param request body PhoneAuthRequest true "手机号认证请求"
// @Produce application/json
// @Success 200 {object} PhoneAuthResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/phone [post]
func (h *AuthHandler) PhoneAuth(c *gin.Context) {
	var req PhoneAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.WithError(c.Request.Context(), err).Error("Invalid phone auth request")
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	user, token, code, err := h.authService.PhoneAuth(c.Request.Context(), req.CountryCode, req.Phone, req.SMSCode)
	if err != nil {
		log.WithError(c.Request.Context(), err).Error("Phone authentication failed")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, PhoneAuthResponse{
		Token: token,
		User:  user,
	})
}

// EmailVerificationRequest 邮箱验证码发送请求
type EmailVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// EmailVerificationResponse 邮箱验证码发送响应
type EmailVerificationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// VerifyEmailRequest 邮箱验证请求
type VerifyEmailRequest struct {
	Email            string `json:"email" binding:"required,email"`
	VerificationCode string `json:"verification_code" binding:"required"`
}

// VerifyEmailResponse 邮箱验证响应
type VerifyEmailResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// SendEmailVerification @Summary Send email verification code
// @Description 发送邮箱验证码
// @Tags Auth
// @Accept application/json
// @Param request body EmailVerificationRequest true "邮箱验证码发送请求"
// @Produce application/json
// @Success 200 {object} EmailVerificationResponse
// @Failure 400 {object} map[string]string
// @Router /api/v1/email/verification [post]
func (h *AuthHandler) SendEmailVerification(c *gin.Context) {
	var req EmailVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.WithError(c.Request.Context(), err).Error("Invalid email verification request")
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	result, code, err := h.authService.SendEmailVerification(c.Request.Context(), req.Email)
	if err != nil {
		log.WithError(c.Request.Context(), err).Error("Send email verification failed")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, result)
}

// VerifyEmail @Summary Verify email code
// @Description 验证邮箱验证码
// @Tags Auth
// @Accept application/json
// @Param request body VerifyEmailRequest true "邮箱验证请求"
// @Produce application/json
// @Success 200 {object} VerifyEmailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/email/verify [post]
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.WithError(c.Request.Context(), err).Error("Invalid verify email request")
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	result, code, err := h.authService.VerifyEmail(c.Request.Context(), req.Email, req.VerificationCode)
	if err != nil {
		log.WithError(c.Request.Context(), err).Error("Verify email failed")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, result)
}
