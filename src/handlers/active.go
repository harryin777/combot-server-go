package handlers

import (
	"net/http"
	"time"

	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/service"

	"github.com/gin-gonic/gin"
)

type ActiveHandler struct {
	deviceService *service.DeviceService
	config        *configs.Config
}

func NewActiveHandler() *ActiveHandler {
	config, _, _ := configs.LoadConfig()
	return &ActiveHandler{
		deviceService: service.NewDevice(config),
		config:        config,
	}
}

// CheckVersion 处理设备的版本检查请求 (对应combot的CheckVersion调用)
// 如果设备未激活，返回包含验证码的activation响应
func (h *ActiveHandler) CheckVersion(c *gin.Context) {
	// 从头部获取设备信息
	deviceID := c.GetHeader("Device-Id")
	clientID := c.GetHeader("Client-Id")
	serialNumber := c.GetHeader("Serial-Number")

	if deviceID == "" || clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要的设备标识"})
		return
	}

	// 检查设备是否已激活
	device, err := h.deviceService.IdentifyDevice(serialNumber, deviceID, clientID)
	if err != nil || device == nil || !device.Activated {
		// 设备未激活，生成验证码
		code, expiresAt, err := h.deviceService.GenerateDeviceVerificationCode(deviceID, clientID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 返回包含验证码的响应 (匹配combot期望的格式)
		c.JSON(http.StatusOK, gin.H{
			"activation": gin.H{
				"code":       code,
				"challenge":  "dummy_challenge", // combot需要的challenge字段
				"message":    "设备未激活，请输入验证码完成绑定",
				"timeout_ms": int64(expiresAt-int(time.Now().Unix())) * 1000,
			},
		})
		return
	}

	// 设备已激活，返回正常配置
	c.JSON(http.StatusOK, gin.H{
		"websocket": gin.H{
			"url": h.config.Web.Websocket, // 从配置文件读取WebSocket URL
		},
	})
}

// Activate 处理设备激活请求 (对应combot的Activate调用)
func (h *ActiveHandler) Activate(c *gin.Context) {
	// 从头部获取设备信息
	deviceID := c.GetHeader("Device-Id")
	clientID := c.GetHeader("Client-Id")

	if deviceID == "" || clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少必要的设备标识"})
		return
	}

	// 解析请求体中的激活数据
	var req struct {
		Algorithm    string `json:"algorithm"`
		SerialNumber string `json:"serial_number"`
		Challenge    string `json:"challenge"`
		Hmac         string `json:"hmac"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 简化版激活逻辑：先找到设备，然后激活
	// 在实际生产环境中，这里应该验证HMAC签名
	device, err := h.deviceService.IdentifyDevice("", deviceID, clientID)
	if err != nil || device == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备未找到"})
		return
	}

	err = h.deviceService.ActivateDevice(device.ID, req.Challenge, req.Hmac)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "激活成功",
	})
}

// 绑定设备 (用户输入验证码)
type BindDeviceRequest struct {
	VerificationCode string `json:"verification_code" binding:"required"`
	DeviceName       string `json:"device_name" binding:"required"`
}

type BindDeviceResponse struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
}

func (h *ActiveHandler) BindDevice(c *gin.Context) {
	var req BindDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	device, err := h.deviceService.BindDeviceToUser(userID.(uint), req.VerificationCode, req.DeviceName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, BindDeviceResponse{
		DeviceID:   device.DeviceID,
		DeviceName: device.DeviceName,
	})
}

// 获取用户设备列表
type GetUserDevicesResponse struct {
	Devices []UserDevice `json:"devices"`
}

type UserDevice struct {
	ID         uint   `json:"id"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Activated  bool   `json:"activated"`
}

func (h *ActiveHandler) GetUserDevices(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	devices, err := h.deviceService.GetUserDevices(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	userDevices := make([]UserDevice, len(devices))
	for i, device := range devices {
		userDevices[i] = UserDevice{
			ID:         device.ID,
			DeviceID:   device.DeviceID,
			DeviceName: device.DeviceName,
			Activated:  device.Activated,
		}
	}

	c.JSON(http.StatusOK, GetUserDevicesResponse{
		Devices: userDevices,
	})
}
