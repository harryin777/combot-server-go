package handlers

import (
	"net/http"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request parameters"})
		return
	}

	// 设置默认值
	if req.Width <= 0 {
		req.Width = 150
	}
	if req.Height <= 0 {
		req.Height = 40
	}

	id, img, err := h.authService.GetCaptcha(req.Width, req.Height)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("Failed to generate captcha")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate captcha"})
		return
	}

	c.JSON(http.StatusOK, CaptchaResponse{
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
		utils.WithError(c.Request.Context(), err).Error("Invalid SMS request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	err := h.authService.SendSMS(req.CountryCode, req.Phone, req.CaptchaID, req.CaptchaValue)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("Failed to send SMS")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, SMSResponse{
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
		utils.WithError(c.Request.Context(), err).Error("Invalid phone auth request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	user, token, err := h.authService.PhoneAuth(req.CountryCode, req.Phone, req.SMSCode)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("Phone authentication failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, PhoneAuthResponse{
		Token: token,
		User:  user,
	})
}
