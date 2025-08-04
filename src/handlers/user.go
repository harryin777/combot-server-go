package handlers

import (
	"net/http"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService service.UserService
}

// NewUserHandler 创建用户处理器
func NewUserHandler(config *configs.Config) *UserHandler {
	return &UserHandler{
		userService: service.NewUserService(config),
	}
}

// UsernamePasswordLoginRequest 用户名密码登录请求
type UsernamePasswordLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// UsernamePasswordLoginResponse 用户名密码登录响应
type UsernamePasswordLoginResponse struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}

// UsernamePasswordLogin @Summary Username password login
// @Description 用户名密码登录
// @Tags User
// @Accept application/json
// @Param request body UsernamePasswordLoginRequest true "用户名密码登录请求"
// @Produce application/json
// @Success 200 {object} UsernamePasswordLoginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/user/login [post]
func (h *UserHandler) UsernamePasswordLogin(c *gin.Context) {
	var req UsernamePasswordLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.WithError(c.Request.Context(), err).Error("Invalid username password login request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	user, token, err := h.userService.UsernamePasswordLogin(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("Username password login failed")
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, UsernamePasswordLoginResponse{
		Token: token,
		User:  user,
	})
}

// UpdateProfileRequest 更新用户信息请求
type UpdateProfileRequest struct {
	Username string `json:"username"`
	Phone    string `json:"phone"`
}

// UpdateProfile @Summary Update user profile
// @Description 更新用户基本信息
// @Tags User
// @Accept application/json
// @Param request body UpdateProfileRequest true "更新用户信息请求"
// @Produce application/json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/user/profile [put]
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.WithError(c.Request.Context(), err).Error("Invalid update profile request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户ID格式错误"})
		return
	}

	err := h.userService.UpdateUserProfile(c.Request.Context(), userIDInt64, req.Username, req.Phone)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("Update profile failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "用户信息更新成功"})
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePassword @Summary Change password
// @Description 修改用户密码
// @Tags User
// @Accept application/json
// @Param request body ChangePasswordRequest true "修改密码请求"
// @Produce application/json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/user/change-password [put]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.WithError(c.Request.Context(), err).Error("Invalid change password request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户ID格式错误"})
		return
	}

	err := h.userService.ChangePassword(c.Request.Context(), userIDInt64, req.OldPassword, req.NewPassword)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("Change password failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

// GetLoginDevicesResponse 获取登录设备响应
type GetLoginDevicesResponse struct {
	Devices []service.LoginDevice `json:"devices"`
}

// GetLoginDevices @Summary Get login devices
// @Description 获取用户登录设备列表
// @Tags User
// @Produce application/json
// @Success 200 {object} GetLoginDevicesResponse
// @Failure 401 {object} map[string]string
// @Router /api/v1/user/devices [get]
func (h *UserHandler) GetLoginDevices(c *gin.Context) {
	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户ID格式错误"})
		return
	}

	devices, err := h.userService.GetLoginDevices(c.Request.Context(), userIDInt64)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("Get login devices failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取登录设备失败"})
		return
	}

	c.JSON(http.StatusOK, GetLoginDevicesResponse{Devices: devices})
}

// LogoutDeviceRequest 退出登录设备请求
type LogoutDeviceRequest struct {
	DeviceIdentifier string `json:"device_identifier" binding:"required"`
}

// LogoutDevice @Summary Logout device
// @Description 退出登录指定设备
// @Tags User
// @Accept application/json
// @Param request body LogoutDeviceRequest true "退出登录设备请求"
// @Produce application/json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/user/logout-device [post]
func (h *UserHandler) LogoutDevice(c *gin.Context) {
	var req LogoutDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.WithError(c.Request.Context(), err).Error("Invalid logout device request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户ID格式错误"})
		return
	}

	err := h.userService.LogoutDevice(c.Request.Context(), userIDInt64, req.DeviceIdentifier)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("Logout device failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "设备退出登录成功"})
}

// DeleteAccountRequest 删除账号请求
type DeleteAccountRequest struct {
	Password string `json:"password" binding:"required"`
}

// DeleteAccount @Summary Delete account
// @Description 删除用户账号
// @Tags User
// @Accept application/json
// @Param request body DeleteAccountRequest true "删除账号请求"
// @Produce application/json
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/user/delete-account [delete]
func (h *UserHandler) DeleteAccount(c *gin.Context) {
	var req DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.WithError(c.Request.Context(), err).Error("Invalid delete account request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权访问"})
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户ID格式错误"})
		return
	}

	err := h.userService.DeleteAccount(c.Request.Context(), userIDInt64, req.Password)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("Delete account failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "账号删除成功"})
}
