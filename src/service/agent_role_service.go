package service

import (
	"context"
	"errors"
	"xiaozhi-server-go/src/dao"
	"xiaozhi-server-go/src/models"
)

type AgentRoleService struct {
	roleDAO *dao.RoleDAO
}

func NewAgentRoleService() *AgentRoleService {
	return &AgentRoleService{
		roleDAO: dao.NewRoleDAO(),
	}
}

// GetRoleTemplates 获取角色模板列表
func (s *AgentRoleService) GetRoleTemplates(ctx context.Context) ([]models.RoleTemplate, error) {
	return s.roleDAO.GetRoleTemplates(ctx)
}

// GetAgentRole 获取智能体角色配置
func (s *AgentRoleService) GetAgentRole(ctx context.Context, userID int64, deviceID string) (*models.AgentRole, error) {
	if userID <= 0 || deviceID == "" {
		return nil, errors.New("用户ID和设备ID不能为空")
	}
	return s.roleDAO.GetAgentRole(ctx, userID, deviceID)
}

// SaveAgentRole 保存智能体角色配置
func (s *AgentRoleService) SaveAgentRole(ctx context.Context, userID int64, req *models.SaveRoleConfigRequest) error {
	if userID <= 0 {
		return errors.New("用户ID不能为空")
	}
	if req.DeviceID == "" {
		return errors.New("设备ID不能为空")
	}
	if req.AssistantName == "" {
		return errors.New("助手名称不能为空")
	}

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
	}

	return s.roleDAO.SaveAgentRole(ctx, agentRole)
}

// GetUserAgentRoles 获取用户的所有智能体角色配置
func (s *AgentRoleService) GetUserAgentRoles(ctx context.Context, userID int64) ([]models.AgentRole, error) {
	if userID <= 0 {
		return nil, errors.New("用户ID不能为空")
	}
	return s.roleDAO.GetUserAgentRoles(ctx, userID)
}

// DeleteAgentRole 删除智能体角色配置
func (s *AgentRoleService) DeleteAgentRole(ctx context.Context, userID int64, deviceID string) error {
	if userID <= 0 || deviceID == "" {
		return errors.New("用户ID和设备ID不能为空")
	}
	return s.roleDAO.DeleteAgentRole(ctx, userID, deviceID)
}

// CreateAgentRoleFromTemplate 基于模板创建智能体角色
func (s *AgentRoleService) CreateAgentRoleFromTemplate(ctx context.Context, userID int64, deviceID, templateID string) (*models.AgentRole, error) {
	if userID <= 0 || deviceID == "" || templateID == "" {
		return nil, errors.New("用户ID、设备ID和模板ID不能为空")
	}

	// 获取模板信息
	template, err := s.roleDAO.GetRoleTemplateByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errors.New("角色模板不存在")
	}

	// 检查是否已存在配置
	existingRole, err := s.roleDAO.GetAgentRole(ctx, userID, deviceID)
	if err != nil {
		return nil, err
	}
	if existingRole != nil {
		return existingRole, nil // 已存在，直接返回
	}

	// 基于模板创建新的智能体角色配置
	agentRole := &models.AgentRole{
		UserID:               userID,
		DeviceID:             deviceID,
		AssistantName:        template.AssistantName,
		ConversationLanguage: template.DefaultLanguage,
		RoleDescription:      template.Description,
		VoiceModel:           template.DefaultVoice,
		CurrentMemory:        "",
		DetailedMemory:       "",
		Temperature:          0.7,
		MaxLength:            2000,
	}

	err = s.roleDAO.CreateAgentRole(ctx, agentRole)
	if err != nil {
		return nil, err
	}

	return agentRole, nil
}
