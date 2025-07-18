package handlers

import (
	"net/http"
	"strconv"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/service"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type ActiveHandler struct {
	deviceService *service.DeviceService
}

func NewActiveHandler(config *configs.Config) *ActiveHandler {
	return &ActiveHandler{
		deviceService: service.NewDevice(config),
	}
}

// RegisterRequest 设备注册请求
type RegisterRequest struct {
	SerialNumber      string `json:"serial_number"`
	DeviceID          string `json:"device_id"`          // MAC地址
	ClientID          string `json:"client_id"`          // UUID
	ActivationVersion int    `json:"activation_version"` // 激活版本号
}

// RegisterResponse 设备注册响应
type RegisterResponse struct {
	DeviceID       uint   `json:"device_id"`
	ActivationCode string `json:"activation_code"`
	Challenge      string `json:"challenge"`
	Token          string `json:"token"`
}

// TokenRequest 获取token请求
type TokenRequest struct {
	DeviceID  string `json:"device_id"` // MAC地址
	ClientID  string `json:"client_id"` // UUID
	Challenge string `json:"challenge"` // 当前挑战
	HMAC      string `json:"hmac"`      // HMAC签名
}

// TokenResponse 获取token响应
type TokenResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

// LoginRequest 设备登录请求
type LoginRequest struct {
	DeviceID  uint   `json:"device_id"`
	Challenge string `json:"challenge"`
	HMAC      string `json:"hmac"`
}

// LoginResponse 设备登录响应
type LoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Message string `json:"message,omitempty"`
}

// Register 处理设备注册
func (h *ActiveHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Error("Invalid register request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 创建或更新设备
	device, err := h.deviceService.CreateOrUpdateDevice(
		req.SerialNumber,
		req.DeviceID,
		req.ClientID,
		req.ActivationVersion,
	)
	if err != nil {
		logrus.WithError(err).Error("Failed to create or update device")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register device"})
		return
	}

	// 返回激活信息
	resp := RegisterResponse{
		DeviceID:       device.ID,
		ActivationCode: device.ActivationCode,
		Challenge:      device.Challenge,
		Token:          device.Token,
	}

	c.JSON(http.StatusOK, resp)
}

// Login 处理设备登录（激活）
func (h *ActiveHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Error("Invalid login request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 激活设备并获取JWT token
	token, err := h.deviceService.ActivateDeviceAndGetToken(req.DeviceID, req.Challenge, req.HMAC)
	if err != nil {
		logrus.WithError(err).Error("Failed to activate device")
		c.JSON(http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "Device activation failed: " + err.Error(),
		})
		return
	}

	// 激活成功，返回JWT token
	c.JSON(http.StatusOK, LoginResponse{
		Success: true,
		Token:   token,
		Message: "Device activated successfully",
	})
}

// Logout 处理设备登出
func (h *ActiveHandler) Logout(c *gin.Context) {
	// 获取设备ID
	deviceIDStr := c.Query("device_id")
	if deviceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id is required"})
		return
	}

	deviceID, err := strconv.ParseUint(deviceIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device_id format"})
		return
	}

	// 这里可以添加登出逻辑，比如清除session等
	// 目前只返回成功响应
	logrus.WithField("device_id", deviceID).Info("Device logout")
	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

// Info 获取设备信息
func (h *ActiveHandler) Info(c *gin.Context) {
	// 获取设备ID
	deviceIDStr := c.Query("device_id")
	if deviceIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id is required"})
		return
	}

	deviceID, err := strconv.ParseUint(deviceIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid device_id format"})
		return
	}

	// 查询设备信息
	device, err := h.deviceService.GetDeviceByID(uint(deviceID))
	if err != nil {
		logrus.WithError(err).Error("Failed to get device info")
		c.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
		return
	}

	// 返回设备信息
	c.JSON(http.StatusOK, gin.H{
		"device_id":          device.ID,
		"serial_number":      device.SerialNumber,
		"device_id_mac":      device.DeviceID,
		"client_id":          device.ClientID,
		"activated":          device.Activated,
		"activated_at":       device.ActivatedAt,
		"last_seen":          device.LastSeen,
		"activation_version": device.ActivationVersion,
	})
}

// GetToken 获取访问token
func (h *ActiveHandler) GetToken(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logrus.WithError(err).Error("Invalid token request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 获取设备访问token
	token, err := h.deviceService.GetDeviceToken(req.DeviceID, req.ClientID, req.Challenge, req.HMAC)
	if err != nil {
		logrus.WithError(err).Error("Failed to get device token")
		c.JSON(http.StatusUnauthorized, TokenResponse{
			Success: false,
			Message: "Failed to get token: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		Success: true,
		Token:   token,
		Message: "Token retrieved successfully",
	})
}
