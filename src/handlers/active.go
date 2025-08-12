package handlers

import (
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/codes"
	"xiaozhi-server-go/src/core/response"
	"xiaozhi-server-go/src/service"

	"github.com/gin-gonic/gin"
)

type ActiveHandler struct {
	deviceService service.ActiveService
	config        *configs.Config
}

func NewActiveHandler() *ActiveHandler {
	config, _, _ := configs.LoadConfig()
	return &ActiveHandler{
		deviceService: service.NewDevice(config),
		config:        config,
	}
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

// BindDevice 绑定设备到用户
// @Summary 绑定设备到用户
// @Description 通过验证码将设备绑定到当前登录用户
// @Tags 设备管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body BindDeviceRequest true "绑定设备请求参数"
// @Success 200 {object} BindDeviceResponse "成功绑定设备"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Router /api/devices/bind [post]
func (h *ActiveHandler) BindDevice(c *gin.Context) {
	var req BindDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	device, code, err := h.deviceService.BindDeviceToUser(c.Request.Context(), uint(userID.(int64)), req.VerificationCode, req.DeviceName)
	if err != nil {
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, BindDeviceResponse{
		DeviceID:   device.DeviceID,
		DeviceName: device.DeviceName,
	})
}

// 获取用户设备列表
type GetUserDevicesResponse struct {
	Devices []UserDevice `json:"devices"`
}

type UserDevice struct {
	ID         uint      `json:"id"`
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Activated  bool      `json:"activated"`
	CreatedAt  time.Time `json:"created_at"`
}

// GetUserDevices 获取用户的设备列表
// @Summary 获取用户的设备列表
// @Description 获取当前登录用户的所有设备
// @Tags 设备管理
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Success 200 {object} GetUserDevicesResponse "成功返回设备列表"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/devices [get]
func (h *ActiveHandler) GetUserDevices(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	devices, code, err := h.deviceService.GetUserDevices(c.Request.Context(), uint(userID.(int64)))
	if err != nil {
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	userDevices := make([]UserDevice, len(devices))
	for i, device := range devices {
		userDevices[i] = UserDevice{
			ID:         device.ID,
			DeviceID:   device.DeviceID,
			DeviceName: device.DeviceName,
			Activated:  device.Activated,
			CreatedAt:  device.CreatedAt,
		}
	}

	response.Success(c, GetUserDevicesResponse{
		Devices: userDevices,
	})
}
