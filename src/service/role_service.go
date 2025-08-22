package service

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/codes"
	"combot-server-go/src/core/utils"
	"combot-server-go/src/dao"
	"combot-server-go/src/models"
	"context"
)

type roleService struct {
	config  *configs.Config
	roleDAO *dao.RoleDAO
}

// NewRoleService 创建角色服务实例
func NewRoleService(config *configs.Config) RoleService {
	return &roleService{
		config:  config,
		roleDAO: dao.NewRoleDAO(),
	}
}

// GetRoleTemplates 获取角色模板列表
func (s *roleService) GetRoleTemplates(ctx context.Context) ([]models.RoleTemplate, int, error) {
	templates, err := s.roleDAO.GetRoleTemplates(ctx)
	if err != nil {
		utils.Errorf(ctx, "GetRoleTemplates error: %v", err)
		return nil, codes.CodeInternalError, err
	}
	return templates, codes.CodeSuccess, nil
}

// GetRoleTemplate 获取角色模板详情
func (s *roleService) GetRoleTemplate(ctx context.Context, templateID string) (*models.RoleTemplate, int, error) {
	template, err := s.roleDAO.GetRoleTemplateByID(ctx, templateID)
	if err != nil {
		utils.Errorf(ctx, "GetRoleTemplate error: %v", err)
		return nil, codes.CodeInternalError, err
	}
	if template == nil {
		return nil, codes.CodeNotFound, nil
	}
	return template, codes.CodeSuccess, nil
}

// SaveRoleConfig 保存角色配置
func (s *roleService) SaveRoleConfig(ctx context.Context, userID int64, config *models.SaveRoleConfigRequest) (int, error) {
	// 验证模板是否存在
	template, err := s.roleDAO.GetRoleTemplateByID(ctx, config.TemplateID)
	if err != nil {
		utils.Errorf(ctx, "GetRoleTemplateByID error: %v", err)
		return codes.CodeInternalError, err
	}
	if template == nil {
		return codes.CodeNotFound, nil
	}

	// 转换为数据库模型
	roleConfig := &models.RoleConfig{
		UserID:               userID,
		DeviceID:             config.DeviceID,
		TemplateID:           config.TemplateID,
		AssistantName:        config.AssistantName,
		ConversationLanguage: config.ConversationLanguage,
		RoleDescription:      config.RoleDescription,
		VoiceModel:           config.VoiceModel,
		CurrentMemory:        config.CurrentMemory,
		DetailedMemory:       config.DetailedMemory,
		Temperature:          config.Temperature,
		MaxLength:            int(config.MaxLength),
	}

	// 保存到数据库
	if err := s.roleDAO.SaveRoleConfig(ctx, roleConfig); err != nil {
		utils.Errorf(ctx, "SaveRoleConfig error: %v", err)
		return codes.CodeInternalError, err
	}

	return codes.CodeSuccess, nil
}
