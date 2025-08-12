package handlers

import (
	"xiaozhi-server-go/src/core/codes"
	"xiaozhi-server-go/src/core/response"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/models"
	"xiaozhi-server-go/src/service"

	"github.com/gin-gonic/gin"
)

type AgentRoleHandler struct {
	agentRoleService *service.AgentRoleService
}

func NewAgentRoleHandler() *AgentRoleHandler {
	return &AgentRoleHandler{
		agentRoleService: service.NewAgentRoleService(),
	}
}

// AgentRoleTemplatesResponse 获取角色模板列表响应
type AgentRoleTemplatesResponse struct {
	Templates []models.RoleTemplate `json:"templates"`
}

// GetAgentRoleResponse 获取智能体角色配置响应
type GetAgentRoleResponse struct {
	AgentRole *models.AgentRole `json:"agent_role"`
	Message   string            `json:"message,omitempty"`
}

// GetUserAgentRolesResponse 获取用户智能体角色列表响应
type GetUserAgentRolesResponse struct {
	AgentRoles []models.AgentRole `json:"agent_roles"`
}

// GetRoleTemplates 获取角色模板列表
// @Router /api/agent/role/templates [get]
func (h *AgentRoleHandler) GetRoleTemplates(c *gin.Context) {
	templates, err := h.agentRoleService.GetRoleTemplates(c.Request.Context())
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("获取角色模板失败")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	response.Success(c, AgentRoleTemplatesResponse{Templates: templates})
}

// GetAgentRole 获取智能体角色配置
// @Router /api/agent/role [get]
func (h *AgentRoleHandler) GetAgentRole(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	deviceID := c.Query("device_id")
	if deviceID == "" {
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	agentRole, err := h.agentRoleService.GetAgentRole(c.Request.Context(), uid, deviceID)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("获取智能体角色配置失败")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	if agentRole == nil {
		response.Success(c, GetAgentRoleResponse{
			AgentRole: nil,
			Message:   "智能体角色配置不存在",
		})
		return
	}

	response.Success(c, GetAgentRoleResponse{AgentRole: agentRole})
}

// SaveAgentRole 保存智能体角色配置
// @Router /api/agent/role [post]
func (h *AgentRoleHandler) SaveAgentRole(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	var req models.SaveRoleConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	err := h.agentRoleService.SaveAgentRole(c.Request.Context(), uid, &req)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("保存智能体角色配置失败")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	response.Success(c, gin.H{"message": "智能体角色配置保存成功"})
}

// GetUserAgentRoles 获取用户的所有智能体角色配置
// @Router /api/user/agent-roles [get]
func (h *AgentRoleHandler) GetUserAgentRoles(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	agentRoles, err := h.agentRoleService.GetUserAgentRoles(c.Request.Context(), uid)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("获取用户智能体角色配置失败")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	response.Success(c, GetUserAgentRolesResponse{AgentRoles: agentRoles})
}

// DeleteAgentRole 删除智能体角色配置
// @Router /api/agent/role [delete]
func (h *AgentRoleHandler) DeleteAgentRole(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	deviceID := c.Query("device_id")
	if deviceID == "" {
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	err := h.agentRoleService.DeleteAgentRole(c.Request.Context(), uid, deviceID)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("删除智能体角色配置失败")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	response.Success(c, gin.H{"message": "智能体角色配置删除成功"})
}

// CreateAgentRoleFromTemplate 基于模板创建智能体角色
// @Router /api/agent/role/from-template [post]
func (h *AgentRoleHandler) CreateAgentRoleFromTemplate(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	var req struct {
		DeviceID   string `json:"device_id" binding:"required"`
		TemplateID string `json:"template_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	agentRole, err := h.agentRoleService.CreateAgentRoleFromTemplate(c.Request.Context(), uid, req.DeviceID, req.TemplateID)
	if err != nil {
		utils.WithError(c.Request.Context(), err).Error("创建智能体角色配置失败")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	response.Success(c, gin.H{
		"agent_role": agentRole,
		"message":    "智能体角色配置创建成功",
	})
}
