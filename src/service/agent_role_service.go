package service

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/codes"
	"combot-server-go/src/core/utils"
	"combot-server-go/src/dao"
	"combot-server-go/src/models"
	"context"
	"time"
)

// AgentRoleService 智能角色服务接口
type AgentRoleService interface {
	// 角色模板管理（只读）
	GetRoleTemplates(ctx context.Context) ([]models.RoleTemplate, int, error)
	GetRoleTemplate(ctx context.Context, templateID string) (*models.RoleTemplate, int, error)

	// 智能体角色配置管理
	GetAgentRole(ctx context.Context, userID int64, deviceID string) (*models.AgentRole, int, error)
	CreateAgentRole(ctx context.Context, userID int64, req *models.CreateAgentRoleRequest) (*models.AgentRole, int, error)
	UpdateAgentRole(ctx context.Context, userID int64, req *models.UpdateAgentRoleRequest) (*models.AgentRole, int, error)
}

type agentRoleService struct {
	config  *configs.Config
	roleDAO *dao.RoleDAO
}

// NewAgentRoleService 创建智能角色服务实例
func NewAgentRoleService(config *configs.Config) AgentRoleService {
	return &agentRoleService{
		config:  config,
		roleDAO: dao.NewRoleDAO(),
	}
}

// GetRoleTemplates 获取角色模板列表（只读）
func (s *agentRoleService) GetRoleTemplates(ctx context.Context) ([]models.RoleTemplate, int, error) {
	templates, err := s.roleDAO.GetRoleTemplates(ctx)
	if err != nil {
		utils.Errorf(ctx, "获取角色模板列表失败: %v", err)
		return nil, codes.CodeInternalError, err
	}
	return templates, codes.CodeSuccess, nil
}

// GetRoleTemplate 获取角色模板详情（只读）
func (s *agentRoleService) GetRoleTemplate(ctx context.Context, templateID string) (*models.RoleTemplate, int, error) {
	template, err := s.roleDAO.GetRoleTemplateByID(ctx, templateID)
	if err != nil {
		utils.Errorf(ctx, "获取角色模板详情失败: %v", err)
		return nil, codes.CodeInternalError, err
	}
	if template == nil {
		return nil, codes.CodeNotFound, nil
	}
	return template, codes.CodeSuccess, nil
}

// GetAgentRole 获取用户智能体角色配置
func (s *agentRoleService) GetAgentRole(ctx context.Context, userID int64, deviceID string) (*models.AgentRole, int, error) {
	agentRole, err := s.roleDAO.GetAgentRole(ctx, userID, deviceID)
	if err != nil {
		utils.Errorf(ctx, "获取智能体角色配置失败: %v", err)
		return nil, codes.CodeInternalError, err
	}
	if agentRole == nil {
		return nil, codes.CodeNotFound, nil
	}
	return agentRole, codes.CodeSuccess, nil
}

// CreateAgentRole 创建智能体角色配置
func (s *agentRoleService) CreateAgentRole(ctx context.Context, userID int64, req *models.CreateAgentRoleRequest) (*models.AgentRole, int, error) {
	// 检查是否已存在该用户和设备的配置
	existing, err := s.roleDAO.GetAgentRole(ctx, userID, req.DeviceID)
	if err != nil {
		utils.Errorf(ctx, "检查智能体角色配置失败: %v", err)
		return nil, codes.CodeInternalError, err
	}
	if existing != nil {
		return nil, codes.CodeDuplicateKey, nil
	}

	// 如果提供了模板ID，验证模板是否存在（但不是必须的）
	if req.TemplateID != "" {
		template, err := s.roleDAO.GetRoleTemplateByID(ctx, req.TemplateID)
		if err != nil {
			utils.Errorf(ctx, "验证角色模板失败: %v", err)
			return nil, codes.CodeInternalError, err
		}
		if template == nil {
			return nil, codes.CodeNotFound, nil
		}
	}

	// 创建智能体角色配置
	now := time.Now()
	agentRole := &models.AgentRole{
		UserID:               userID,
		DeviceID:             req.DeviceID,
		AssistantName:        req.AssistantName,
		ConversationLanguage: req.ConversationLanguage,
		RoleDescription:      req.RoleDescription,
		VoiceModel:           req.VoiceModel,
		CurrentMemory:        req.CurrentMemory,
		DetailedMemory:       req.DetailedMemory,
		Temperature:          req.Temperature,
		MaxLength:            req.MaxLength,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	// 设置默认值
	if agentRole.Temperature == 0 {
		agentRole.Temperature = 0.7
	}
	if agentRole.MaxLength == 0 {
		agentRole.MaxLength = 2000
	}

	if err := s.roleDAO.SaveAgentRole(ctx, agentRole); err != nil {
		utils.Errorf(ctx, "创建智能体角色配置失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	utils.Infof(ctx, "成功创建智能体角色配置，用户ID: %d, 设备ID: %s", userID, req.DeviceID)
	return agentRole, codes.CodeSuccess, nil
}

// UpdateAgentRole 更新智能体角色配置
func (s *agentRoleService) UpdateAgentRole(ctx context.Context, userID int64, req *models.UpdateAgentRoleRequest) (*models.AgentRole, int, error) {
	// 检查配置是否存在
	existing, err := s.roleDAO.GetAgentRole(ctx, userID, req.DeviceID)
	if err != nil {
		utils.Errorf(ctx, "获取智能体角色配置失败: %v", err)
		return nil, codes.CodeInternalError, err
	}
	if existing == nil {
		return nil, codes.CodeNotFound, nil
	}

	// 如果提供了模板ID，验证模板是否存在（但不是必须的）
	if req.TemplateID != "" {
		template, err := s.roleDAO.GetRoleTemplateByID(ctx, req.TemplateID)
		if err != nil {
			utils.Errorf(ctx, "验证角色模板失败: %v", err)
			return nil, codes.CodeInternalError, err
		}
		if template == nil {
			return nil, codes.CodeNotFound, nil
		}
	}

	// 更新智能体角色配置
	now := time.Now()
	agentRole := &models.AgentRole{
		UserID:               userID,
		DeviceID:             req.DeviceID,
		AssistantName:        req.AssistantName,
		ConversationLanguage: req.ConversationLanguage,
		RoleDescription:      req.RoleDescription,
		VoiceModel:           req.VoiceModel,
		CurrentMemory:        req.CurrentMemory,
		DetailedMemory:       req.DetailedMemory,
		Temperature:          req.Temperature,
		MaxLength:            req.MaxLength,
		UpdatedAt:            now,
	}

	// 设置默认值
	if agentRole.Temperature == 0 {
		agentRole.Temperature = 0.7
	}
	if agentRole.MaxLength == 0 {
		agentRole.MaxLength = 2000
	}

	if err := s.roleDAO.SaveAgentRole(ctx, agentRole); err != nil {
		utils.Errorf(ctx, "更新智能体角色配置失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	utils.Infof(ctx, "成功更新智能体角色配置，用户ID: %d, 设备ID: %s", userID, req.DeviceID)

	// 重新获取更新后的配置返回
	updatedRole, err := s.roleDAO.GetAgentRole(ctx, userID, req.DeviceID)
	if err != nil {
		utils.Errorf(ctx, "获取更新后的智能体角色配置失败: %v", err)
		return nil, codes.CodeInternalError, err
	}

	return updatedRole, codes.CodeSuccess, nil
}
