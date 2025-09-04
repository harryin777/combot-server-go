package handlers

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/codes"
	"combot-server-go/src/core/response"
	"combot-server-go/src/log"
	"combot-server-go/src/models"
	"combot-server-go/src/service"

	"github.com/gin-gonic/gin"
)

type RoleHandler struct {
	roleService service.RoleService
}

// NewRoleHandler 创建角色处理器
func NewRoleHandler(config *configs.Config) *RoleHandler {
	return &RoleHandler{
		roleService: service.NewRoleService(config),
	}
}

// GetRoleTemplatesResponse 获取角色模板列表响应
type GetRoleTemplatesResponse struct {
	Templates []models.RoleTemplate `json:"templates"`
}

// GetRoleTemplates 获取角色模板列表
// @Summary 获取角色模板列表
// @Description 获取所有可用的角色模板
// @Tags 角色管理
// @Produce json
// @Success 200 {object} GetRoleTemplatesResponse "成功返回角色模板列表"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/roles/templates [get]
func (h *RoleHandler) GetRoleTemplates(c *gin.Context) {
	templates, code, err := h.roleService.GetRoleTemplates(c.Request.Context())
	if err != nil {
		log.WithError(c.Request.Context(), err).Error("获取角色模板列表失败")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, GetRoleTemplatesResponse{Templates: templates})
}

// GetRoleTemplate 获取角色模板详情
// @Summary 获取角色模板详情
// @Description 根据模板ID获取角色模板的详细信息
// @Tags 角色管理
// @Param templateId path string true "角色模板ID"
// @Produce json
// @Success 200 {object} models.RoleTemplate "成功返回角色模板详情"
// @Failure 404 {object} map[string]interface{} "模板不存在"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/roles/templates/{templateId} [get]
func (h *RoleHandler) GetRoleTemplate(c *gin.Context) {
	templateID := c.Param("templateId")
	if templateID == "" {
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	template, code, err := h.roleService.GetRoleTemplate(c.Request.Context(), templateID)
	if err != nil {
		log.WithError(c.Request.Context(), err).Error("获取角色模板详情失败")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, template)
}

// SaveRoleConfigResponse 保存角色配置响应
type SaveRoleConfigResponse struct {
	Message string `json:"message"`
}

// SaveRoleConfig 保存角色配置
// @Summary 保存角色配置
// @Description 保存设备的角色配置信息
// @Tags 角色管理
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer token"
// @Param request body models.SaveRoleConfigRequest true "角色配置信息"
// @Success 200 {object} SaveRoleConfigResponse "成功保存角色配置"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Failure 500 {object} map[string]interface{} "服务器内部错误"
// @Router /api/roles/config [post]
func (h *RoleHandler) SaveRoleConfig(c *gin.Context) {
	var req models.SaveRoleConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.WithError(c.Request.Context(), err).Error("无效的角色配置请求")
		response.Failed(c, codes.CodeInvalidRequest, nil)
		return
	}

	// 从JWT token中获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Failed(c, codes.CodeUnauthorized, nil)
		return
	}

	code, err := h.roleService.SaveRoleConfig(c.Request.Context(), userID.(int64), &req)
	if err != nil {
		log.WithError(c.Request.Context(), err).Error("保存角色配置失败")
		response.Failed(c, codes.CodeInternalError, nil)
		return
	}

	if code != codes.CodeSuccess {
		response.Failed(c, code, nil)
		return
	}

	response.Success(c, SaveRoleConfigResponse{Message: "角色配置保存成功"})
}
