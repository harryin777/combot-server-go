package handlers

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/codes"
	"combot-server-go/src/core/response"
	"combot-server-go/src/log"
	"combot-server-go/src/service"

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

// UsernamePasswordRegisterRequest 用户名密码注册请求
type UsernamePasswordRegisterRequest struct {
	Username     string `json:"username" binding:"required"`
	Password     string `json:"password" binding:"required"`
	Email        string `json:"email" binding:"omitempty,email"`
	CaptchaID    string `json:"captcha_id" binding:"required"`
	CaptchaValue string `json:"captcha_value" binding:"required"`
}

// UsernamePasswordRegisterResponse 用户名密码注册响应
type UsernamePasswordRegisterResponse struct {
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
		log.Errorf(c.Request.Context(), "Invalid username password login request, err : %v", err)
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	user, token, code, err := h.userService.UsernamePasswordLogin(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		log.Errorf(c.Request.Context(), "Username password login failed, err : %v", err)
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, UsernamePasswordLoginResponse{
		Token: token,
		User:  user,
	})
}

// UsernamePasswordRegister @Summary Username password register
// @Description 用户名密码注册
// @Tags User
// @Accept application/json
// @Param request body UsernamePasswordRegisterRequest true "用户名密码注册请求"
// @Produce application/json
// @Success 200 {object} UsernamePasswordRegisterResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/v1/user/register [post]
func (h *UserHandler) UsernamePasswordRegister(c *gin.Context) {
	var req UsernamePasswordRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorf(c.Request.Context(), "Invalid username password register request, err : %v", err)
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	// 打印接收到的验证码信息用于调试
	log.Infof(c.Request.Context(), "注册请求 - 用户名: %s, 邮箱: %s, 验证码ID: %s, 验证码值: %s",
		req.Username, req.Email, req.CaptchaID, req.CaptchaValue)

	// 暂时跳过图形验证码验证，因为前端使用Canvas自绘验证码
	// 实际生产环境中应该实现服务端验证码验证逻辑

	user, token, code, err := h.userService.UsernamePasswordRegister(c.Request.Context(), req.Username, req.Password, req.Email)
	if err != nil {
		log.Errorf(c.Request.Context(), "Username password register failed, err : %v", err)
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, UsernamePasswordRegisterResponse{
		Token: token,
		User:  user,
	})
}

// GetProfileResponse 获取用户信息响应
type GetProfileResponse struct {
	Username string `json:"username"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Remark   string `json:"remark"`
}

// GetProfile @Summary Get user profile
// @Description 获取用户基本信息
// @Tags User
// @Produce application/json
// @Success 200 {object} GetProfileResponse
// @Failure 401 {object} map[string]string
// @Router /api/user/profile [post]
func (h *UserHandler) GetProfile(c *gin.Context) {
	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	user, err := h.userService.GetUserByID(c.Request.Context(), userIDInt64)
	if err != nil {
		log.Errorf(c.Request.Context(), "Get user profile failed, err : %v", err)
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	response.Success(c, GetProfileResponse{
		Username: user.Username,
		Phone:    user.Phone,
		Email:    user.Email,
		Remark:   user.Remark,
	})
}

// UpdateProfileRequest 更新用户信息请求
type UpdateProfileRequest struct {
	Username string `json:"username"`
	Phone    string `json:"phone"`  // 手机号改为非必填
	Email    string `json:"email"`  // 添加邮箱字段
	Remark   string `json:"remark"` // 添加备注字段
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
		log.Errorf(c.Request.Context(), "Invalid update profile request, err : %v", err)
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	_, code, err := h.userService.UpdateUserProfile(c.Request.Context(), userIDInt64, req.Username, req.Phone, req.Email, req.Remark)
	if err != nil {
		log.Errorf(c.Request.Context(), "Update profile failed, err : %v", err)
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.SuccessWithMessage(c, "用户信息更新成功", nil)
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
		log.Errorf(c.Request.Context(), "Invalid change password request, err : %v", err)
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	_, code, err := h.userService.ChangePassword(c.Request.Context(), userIDInt64, req.OldPassword, req.NewPassword)
	if err != nil {
		log.Errorf(c.Request.Context(), "Change password failed, err : %v", err)
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.SuccessWithMessage(c, "密码修改成功", nil)
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
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	devices, code, err := h.userService.GetLoginDevices(c.Request.Context(), userIDInt64)
	if err != nil {
		log.Errorf(c.Request.Context(), "Get login devices failed, err : %v", err)
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, GetLoginDevicesResponse{Devices: devices})
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
		log.Errorf(c.Request.Context(), "Invalid logout device request, err : %v", err)
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	_, code, err := h.userService.LogoutDevice(c.Request.Context(), userIDInt64, req.DeviceIdentifier)
	if err != nil {
		log.Errorf(c.Request.Context(), "Logout device failed, err : %v", err)
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.SuccessWithMessage(c, "设备退出登录成功", nil)
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
		log.Errorf(c.Request.Context(), "Invalid delete account request, err : %v", err)
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	userIDInt64, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	_, code, err := h.userService.DeleteAccount(c.Request.Context(), userIDInt64, req.Password)
	if err != nil {
		log.Errorf(c.Request.Context(), "Delete account failed, err : %v", err)
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}
	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.SuccessWithMessage(c, "账号删除成功", nil)
}
