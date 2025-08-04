package ota

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/auth"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/service"

	"github.com/gin-gonic/gin"
)

// OtaFirmwareResponse 定义OTA固件接口返回结构
type OtaFirmwareResponse struct {
	ServerTime struct {
		Timestamp      int64 `json:"timestamp" example:"1688443200000"`
		TimezoneOffset int   `json:"timezone_offset" example:"480"`
	} `json:"server_time"`
	Firmware struct {
		Version string `json:"version" example:"1.0.3"`
		URL     string `json:"url" example:"/ota_bin/1.0.3.bin"`
	} `json:"firmware"`
	Websocket struct {
		URL   string `json:"url" example:"wss://example.com/ota"`
		Token string `json:"token,omitempty" example:"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	} `json:"websocket"`
	Activation *struct {
		Code      string `json:"code" example:"123456"`
		Challenge string `json:"challenge" example:"dummy_challenge"`
		Message   string `json:"message" example:"设备未激活，请输入验证码完成绑定"`
		TimeoutMs int64  `json:"timeout_ms" example:"300000"`
	} `json:"activation,omitempty"`
}

// ErrorResponse 定义错误返回结构
type ErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Message string `json:"message" example:"缺少 device-id"`
}

// @Summary OTA 预检请求
// @Description 处理 OTA 接口的 OPTIONS 预检请求，返回 200
// @Tags OTA
// @Accept */*
// @Produce plain
// @Success 200 {string} string "OK"
// @Router /ota/ [options]
func handleOtaOptions(c *gin.Context) {
	c.Status(http.StatusOK)
}

// @Summary 获取 OTA 状态
// @Description 获取 OTA 服务状态和 WebSocket 地址，供设备查询
// @Tags OTA
// @Produce plain
// @Success 200 {string} string "OTA interface is running, websocket address: ws://..."
// @Router /ota/ [get]
func handleOtaGet(c *gin.Context, updateURL string) {
	c.String(http.StatusOK, "OTA interface is running, websocket address: "+updateURL)
}

// 请求体结构体定义
type OtaRequest struct {
	Application struct {
		Version string `json:"version" example:"1.0.0"`
	} `json:"application"`
}

// @Summary 上传设备信息获取最新固件
// @Description 设备上传信息后，返回最新固件版本和下载地址
// @Tags OTA
// @Accept json
// @Produce json
// @Param device-id header string true "设备ID"
// @Param body body OtaRequest true "请求体"
// @Success 200 {object} OtaFirmwareResponse
// @Failure 400 {object} ErrorResponse
// @Router /ota/ [post]
func handleOtaPost(c *gin.Context, updateURL string, config *configs.Config) {
	// ComBot发送的是大写的请求头，需要正确读取
	deviceID := c.GetHeader("Device-Id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Success: false, Message: "缺少 Device-Id"})
		return
	}
	var body OtaRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Success: false, Message: "解析失败: " + err.Error()})
		return
	}

	version := body.Application.Version
	if version == "" {
		version = "1.0.0"
	}

	otaDir := filepath.Join(".", "ota_bin")
	_ = os.MkdirAll(otaDir, 0755)
	bins, _ := filepath.Glob(filepath.Join(otaDir, "*.bin"))
	firmwareURL := ""
	if len(bins) > 0 {
		sort.Slice(bins, func(i, j int) bool {
			return versionLess(bins[j], bins[i])
		})
		latest := filepath.Base(bins[0])
		version = strings.TrimSuffix(latest, ".bin")
		firmwareURL = "/ota_bin/" + latest
	}

	resp := OtaFirmwareResponse{}
	resp.ServerTime.Timestamp = time.Now().UnixNano() / 1e6
	resp.ServerTime.TimezoneOffset = 8 * 60
	resp.Firmware.Version = version
	resp.Firmware.URL = firmwareURL
	resp.Websocket.URL = updateURL

	// 为已激活的设备生成token，未激活的设备生成验证码
	deviceService := service.NewDevice(config)
	clientID := c.GetHeader("Client-Id")
	serialNumber := c.GetHeader("Serial-Number")

	if device, err := deviceService.IdentifyDevice(c.Request.Context(), serialNumber, deviceID, clientID); err == nil && device != nil && device.Activated {
		// 设备已激活，生成新的token
		authToken := auth.NewAuthToken(config.Server.Token)
		if token, err := authToken.GenerateToken(device.DeviceID); err == nil {
			resp.Websocket.Token = token
			utils.WithField(context.Background(), "device_id", deviceID).Info("为已激活设备生成了新token")
		} else {
			utils.WithError(context.Background(), err).WithField("device_id", deviceID).Warn("生成token失败")
		}
	} else {
		// 设备未激活或不存在，生成验证码
		code, expiresAt, err := deviceService.GenerateDeviceVerificationCode(c.Request.Context(), deviceID, clientID)
		if err != nil {
			utils.WithError(context.Background(), err).WithField("device_id", deviceID).Error("生成验证码失败")
		} else {
			resp.Activation = &struct {
				Code      string `json:"code" example:"123456"`
				Challenge string `json:"challenge" example:"dummy_challenge"`
				Message   string `json:"message" example:"设备未激活，请输入验证码完成绑定"`
				TimeoutMs int64  `json:"timeout_ms" example:"300000"`
			}{
				Code:      code,
				Challenge: "dummy_challenge", // combot需要的challenge字段
				Message:   "设备未激活，请输入验证码完成绑定",
				TimeoutMs: (expiresAt - time.Now().Unix()) * 1000,
			}
			utils.WithField(context.Background(), "device_id", deviceID).Info("为未激活设备生成了验证码")
		}
	}

	c.JSON(http.StatusOK, resp)
}

// @Summary 下载 OTA 固件文件
// @Description 根据文件名下载 OTA 固件
// @Tags OTA
// @Produce application/octet-stream
// @Param filename path string true "固件文件名"
// @Success 200 "文件流"
// @Failure 404 {object} ErrorResponse
// @Router /ota_bin/{filename} [get]
func handleOtaBinDownload(c *gin.Context) {
	fname := c.Param("filename")
	p := filepath.Join("ota_bin", fname)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, ErrorResponse{Success: false, Message: "file not found"})
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename="+fname)
	c.File(p)
}

// 激活请求体结构体定义 (对应combot的GetActivationPayload)
type ActivateRequest struct {
	Algorithm    string `json:"algorithm" example:"hmac-sha256"`
	SerialNumber string `json:"serial_number" example:"ABC123456"`
	Challenge    string `json:"challenge" example:"dummy_challenge"`
	Hmac         string `json:"hmac" example:"a1b2c3d4..."`
}

// @Summary 设备激活确认
// @Description 设备通过HMAC签名确认激活
// @Tags OTA
// @Accept json
// @Produce json
// @Param device-id header string true "设备ID"
// @Param client-id header string true "客户端ID"
// @Param body body ActivateRequest true "激活请求"
// @Success 200 {string} string "OK"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /ota/activate [post]
func handleOtaActivate(c *gin.Context, config *configs.Config) {
	deviceID := c.GetHeader("Device-Id")
	clientID := c.GetHeader("Client-Id")

	if deviceID == "" || clientID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Success: false, Message: "缺少必要的设备标识"})
		return
	}

	var req ActivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Success: false, Message: "解析激活请求失败: " + err.Error()})
		return
	}

	deviceService := service.NewDevice(config)

	// 查找设备
	device, err := deviceService.IdentifyDevice(c.Request.Context(), req.SerialNumber, deviceID, clientID)
	if err != nil || device == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Success: false, Message: "设备未找到"})
		return
	}

	// 检查设备是否已被用户绑定（有UserID表示已绑定）
	if device.UserID == nil {
		// 设备还未被用户绑定，返回202让ComBot继续重试
		utils.WithField(context.Background(), "device_id", deviceID).Debug("设备尚未绑定到用户，返回202状态码")
		c.Status(http.StatusAccepted) // 202 Accepted - ComBot会重试
		return
	}

	// 激活设备
	if err := deviceService.ActivateDevice(c.Request.Context(), device.ID, req.Challenge, req.Hmac); err != nil {
		utils.WithError(context.Background(), err).WithField("device_id", deviceID).Error("设备激活失败")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Success: false, Message: "激活失败: " + err.Error()})
		return
	}

	utils.WithField(context.Background(), "device_id", deviceID).Info("设备激活成功")
	c.JSON(http.StatusOK, gin.H{"message": "激活成功"})
}

// versionLess 比较版本号语义 a < b
func versionLess(a, b string) bool {
	aV := strings.Split(strings.TrimSuffix(filepath.Base(a), ".bin"), ".")
	bV := strings.Split(strings.TrimSuffix(filepath.Base(b), ".bin"), ".")
	for i := 0; i < len(aV) && i < len(bV); i++ {
		if aV[i] != bV[i] {
			return aV[i] < bV[i]
		}
	}
	return len(aV) < len(bV)
}
