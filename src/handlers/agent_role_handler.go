package handlers

import (
	"net/http"

	"combot-server-go/src/configs"
	"combot-server-go/src/core/codes"
	"combot-server-go/src/core/log"
	"combot-server-go/src/core/response"
	"combot-server-go/src/models"
	"combot-server-go/src/service"

	"github.com/gin-gonic/gin"
)

// AgentRoleHandler 智能角色处理器
type AgentRoleHandler struct {
	agentRoleService service.AgentRoleService
}

// NewAgentRoleHandler 创建智能角色处理器实例
func NewAgentRoleHandler(config *configs.Config) *AgentRoleHandler {
	return &AgentRoleHandler{
		agentRoleService: service.NewAgentRoleService(config),
	}
}

// GetRoleTemplates 获取角色模板列表
// @Summary 获取角色模板列表
// @Description 获取所有可用的角色模板（只读参考数据）
// @Tags 角色模板
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Response{data=[]models.RoleTemplate} "成功"
// @Failure 401 {object} response.Response "未授权"
// @Failure 500 {object} response.Response "服务器错误"
// @Router /api/agent-role/templates [post]
func (h *AgentRoleHandler) GetRoleTemplates(c *gin.Context) {
	ctx := c.Request.Context()

	templates, code, err := h.agentRoleService.GetRoleTemplates(ctx)
	if err != nil {
		log.Errorf(ctx, "获取角色模板列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, response.Response{
			Code:    code,
			Message: "获取角色模板列表失败",
		})
		return
	}

	c.JSON(http.StatusOK, response.Response{
		Code:    codes.CodeSuccess,
		Message: "获取角色模板列表成功",
		Data:    templates,
	})
}

// GetRoleTemplate 获取角色模板详情
// @Summary 获取角色模板详情
// @Description 根据ID获取指定角色模板的详细信息
// @Tags 角色模板
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body models.GetRoleTemplateRequest true "获取角色模板详情请求"
// @Success 200 {object} response.Response{data=models.RoleTemplate} "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 401 {object} response.Response "未授权"
// @Failure 404 {object} response.Response "角色模板不存在"
// @Failure 500 {object} response.Response "服务器错误"
// @Router /api/agent-role/template/detail [post]
func (h *AgentRoleHandler) GetRoleTemplate(c *gin.Context) {
	ctx := c.Request.Context()

	var req models.GetRoleTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorf(ctx, "参数绑定失败: %v", err)
		c.JSON(http.StatusBadRequest, response.Response{
			Code:    codes.CodeInvalidRequest,
			Message: "请求参数格式错误",
		})
		return
	}

	if req.TemplateID == "" {
		c.JSON(http.StatusBadRequest, response.Response{
			Code:    codes.CodeInvalidRequest,
			Message: "模板ID不能为空",
		})
		return
	}

	template, code, err := h.agentRoleService.GetRoleTemplate(ctx, req.TemplateID)
	if err != nil {
		log.Errorf(ctx, "获取角色模板详情失败: %v", err)
		c.JSON(http.StatusInternalServerError, response.Response{
			Code:    code,
			Message: "获取角色模板详情失败",
		})
		return
	}

	if template == nil {
		c.JSON(http.StatusNotFound, response.Response{
			Code:    codes.CodeNotFound,
			Message: "角色模板不存在",
		})
		return
	}

	c.JSON(http.StatusOK, response.Response{
		Code:    codes.CodeSuccess,
		Message: "获取角色模板详情成功",
		Data:    template,
	})
}

// GetAgentRole 获取用户智能体角色配置
// @Summary 获取用户智能体角色配置
// @Description 获取指定用户和设备的智能体角色配置
// @Tags 智能体角色
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body models.GetAgentRoleRequest true "获取智能体角色配置请求"
// @Success 200 {object} response.Response{data=models.AgentRole} "成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 401 {object} response.Response "未授权"
// @Failure 404 {object} response.Response "智能体角色配置不存在"
// @Failure 500 {object} response.Response "服务器错误"
// @Router /api/agent-role/config/get [post]
func (h *AgentRoleHandler) GetAgentRole(c *gin.Context) {
	ctx := c.Request.Context()

	// 从token中获取用户ID
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, response.Response{
			Code:    codes.CodeUnauthorized,
			Message: "用户未登录",
		})
		return
	}

	userID, ok := userIDInterface.(int64)
	if !ok {
		log.Errorf(ctx, "用户ID类型转换失败")
		c.JSON(http.StatusUnauthorized, response.Response{
			Code:    codes.CodeUnauthorized,
			Message: "用户身份验证失败",
		})
		return
	}

	var req models.GetAgentRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorf(ctx, "参数绑定失败: %v", err)
		c.JSON(http.StatusBadRequest, response.Response{
			Code:    codes.CodeInvalidRequest,
			Message: "请求参数格式错误",
		})
		return
	}

	if req.DeviceID == "" {
		c.JSON(http.StatusBadRequest, response.Response{
			Code:    codes.CodeInvalidRequest,
			Message: "设备ID不能为空",
		})
		return
	}

	agentRole, code, err := h.agentRoleService.GetAgentRole(ctx, userID, req.DeviceID)
	if err != nil {
		log.Errorf(ctx, "获取智能体角色配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, response.Response{
			Code:    code,
			Message: "获取智能体角色配置失败",
		})
		return
	}

	if agentRole == nil {
		c.JSON(http.StatusNotFound, response.Response{
			Code:    codes.CodeNotFound,
			Message: "智能体角色配置不存在",
		})
		return
	}

	c.JSON(http.StatusOK, response.Response{
		Code:    codes.CodeSuccess,
		Message: "获取智能体角色配置成功",
		Data:    agentRole,
	})
}

// CreateAgentRole 创建智能体角色配置
// @Summary 创建智能体角色配置
// @Description 为用户创建新的智能体角色配置，可选择角色模板作为参考
// @Tags 智能体角色
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body models.CreateAgentRoleRequest true "创建智能体角色配置请求"
// @Success 201 {object} response.Response{data=models.AgentRole} "创建成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 401 {object} response.Response "未授权"
// @Failure 409 {object} response.Response "配置已存在"
// @Failure 500 {object} response.Response "服务器错误"
// @Router /api/agent-role/config/create [post]
func (h *AgentRoleHandler) CreateAgentRole(c *gin.Context) {
	ctx := c.Request.Context()

	// 从token中获取用户ID
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, response.Response{
			Code:    codes.CodeUnauthorized,
			Message: "用户未登录",
		})
		return
	}

	userID, ok := userIDInterface.(int64)
	if !ok {
		log.Errorf(ctx, "用户ID类型转换失败")
		c.JSON(http.StatusUnauthorized, response.Response{
			Code:    codes.CodeUnauthorized,
			Message: "用户身份验证失败",
		})
		return
	}

	var req models.CreateAgentRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorf(ctx, "参数绑定失败: %v", err)
		c.JSON(http.StatusBadRequest, response.Response{
			Code:    codes.CodeInvalidRequest,
			Message: "请求参数格式错误",
		})
		return
	}

	// 验证必填字段
	if req.DeviceID == "" {
		c.JSON(http.StatusBadRequest, response.Response{
			Code:    codes.CodeInvalidRequest,
			Message: "设备ID不能为空",
		})
		return
	}

	if req.AssistantName == "" {
		c.JSON(http.StatusBadRequest, response.Response{
			Code:    codes.CodeInvalidRequest,
			Message: "助手名称不能为空",
		})
		return
	}

	agentRole, code, err := h.agentRoleService.CreateAgentRole(ctx, userID, &req)
	if err != nil {
		switch code {
		case codes.CodeDuplicateKey:
			c.JSON(http.StatusConflict, response.Response{
				Code:    code,
				Message: "该设备已存在智能体角色配置",
			})
		case codes.CodeNotFound:
			c.JSON(http.StatusBadRequest, response.Response{
				Code:    code,
				Message: "指定的角色模板不存在",
			})
		default:
			log.Errorf(ctx, "创建智能体角色配置失败: %v", err)
			c.JSON(http.StatusInternalServerError, response.Response{
				Code:    code,
				Message: "创建智能体角色配置失败",
			})
		}
		return
	}

	c.JSON(http.StatusCreated, response.Response{
		Code:    codes.CodeSuccess,
		Message: "创建智能体角色配置成功",
		Data:    agentRole,
	})
}

// UpdateAgentRole 更新智能体角色配置
// @Summary 更新智能体角色配置
// @Description 更新用户的智能体角色配置，可选择角色模板作为参考
// @Tags 智能体角色
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body models.UpdateAgentRoleRequest true "更新智能体角色配置请求"
// @Success 200 {object} response.Response{data=models.AgentRole} "更新成功"
// @Failure 400 {object} response.Response "请求参数错误"
// @Failure 401 {object} response.Response "未授权"
// @Failure 404 {object} response.Response "配置不存在"
// @Failure 500 {object} response.Response "服务器错误"
// @Router /api/agent-role/config/update [post]
func (h *AgentRoleHandler) UpdateAgentRole(c *gin.Context) {
	ctx := c.Request.Context()

	// 从token中获取用户ID
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, response.Response{
			Code:    codes.CodeUnauthorized,
			Message: "用户未登录",
		})
		return
	}

	userID, ok := userIDInterface.(int64)
	if !ok {
		log.Errorf(ctx, "用户ID类型转换失败")
		c.JSON(http.StatusUnauthorized, response.Response{
			Code:    codes.CodeUnauthorized,
			Message: "用户身份验证失败",
		})
		return
	}

	var req models.UpdateAgentRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorf(ctx, "参数绑定失败: %v", err)
		c.JSON(http.StatusBadRequest, response.Response{
			Code:    codes.CodeInvalidRequest,
			Message: "请求参数格式错误",
		})
		return
	}

	// 验证必填字段
	if req.DeviceID == "" {
		c.JSON(http.StatusBadRequest, response.Response{
			Code:    codes.CodeInvalidRequest,
			Message: "设备ID不能为空",
		})
		return
	}

	if req.AssistantName == "" {
		c.JSON(http.StatusBadRequest, response.Response{
			Code:    codes.CodeInvalidRequest,
			Message: "助手名称不能为空",
		})
		return
	}

	agentRole, code, err := h.agentRoleService.UpdateAgentRole(ctx, userID, &req)
	if err != nil {
		switch code {
		case codes.CodeNotFound:
			c.JSON(http.StatusNotFound, response.Response{
				Code:    code,
				Message: "智能体角色配置不存在",
			})
		default:
			log.Errorf(ctx, "更新智能体角色配置失败: %v", err)
			c.JSON(http.StatusInternalServerError, response.Response{
				Code:    code,
				Message: "更新智能体角色配置失败",
			})
		}
		return
	}

	c.JSON(http.StatusOK, response.Response{
		Code:    codes.CodeSuccess,
		Message: "更新智能体角色配置成功",
		Data:    agentRole,
	})
}
