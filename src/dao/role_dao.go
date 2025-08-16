package dao

import (
	"context"
	"xiaozhi-server-go/src/models"

	"gorm.io/gorm"
)

type RoleDAO struct {
	*BaseDAO
}

func NewRoleDAO() *RoleDAO {
	return &RoleDAO{
		BaseDAO: NewBaseDAO(),
	}
}

// GetRoleTemplates 获取所有启用的角色模板
func (dao *RoleDAO) GetRoleTemplates(ctx context.Context) ([]models.RoleTemplate, error) {
	var templates []models.RoleTemplate
	err := dao.Query(ctx, &templates, QueryOptions{
		Table: "role_templates",
		Where: map[string]interface{}{
			"enabled": true,
		},
		OrderBy: []string{"sort_order ASC", "created_at ASC"},
	})
	return templates, err
}

// GetRoleTemplateByID 根据ID获取角色模板
func (dao *RoleDAO) GetRoleTemplateByID(ctx context.Context, templateID string) (*models.RoleTemplate, error) {
	var template models.RoleTemplate
	err := dao.First(ctx, &template, QueryOptions{
		Table: "role_templates",
		Where: map[string]interface{}{
			"id":      templateID,
			"enabled": true,
		},
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // 记录不存在返回nil
		}
		return nil, err
	}
	return &template, nil
}

// ===== AgentRole 相关方法 =====

// GetAgentRole 获取用户设备的智能体角色配置
func (dao *RoleDAO) GetAgentRole(ctx context.Context, userID int64, deviceID string) (*models.AgentRole, error) {
	var agentRole models.AgentRole
	err := dao.First(ctx, &agentRole, QueryOptions{
		Table: "agent_roles",
		Where: map[string]interface{}{
			"user_id":   userID,
			"device_id": deviceID,
		},
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // 记录不存在返回nil
		}
		return nil, err
	}
	return &agentRole, nil
}

// CreateAgentRole 创建智能体角色配置
func (dao *RoleDAO) CreateAgentRole(ctx context.Context, agentRole *models.AgentRole) error {
	return dao.Create(ctx, agentRole)
}

// UpdateAgentRole 更新智能体角色配置
func (dao *RoleDAO) UpdateAgentRole(ctx context.Context, userID int64, deviceID string, updates map[string]interface{}) error {
	return dao.Update(ctx, "agent_roles", map[string]interface{}{
		"user_id":   userID,
		"device_id": deviceID,
	}, updates)
}

// SaveAgentRole 保存智能体角色配置（存在则更新，不存在则创建）
func (dao *RoleDAO) SaveAgentRole(ctx context.Context, agentRole *models.AgentRole) error {
	// 先检查是否已存在该用户和设备的配置
	existing, err := dao.GetAgentRole(ctx, agentRole.UserID, agentRole.DeviceID)
	if err != nil {
		return err
	}

	if existing == nil {
		// 不存在，创建新记录
		return dao.CreateAgentRole(ctx, agentRole)
	} else {
		// 已存在，更新记录
		updates := map[string]interface{}{
			"assistant_name":        agentRole.AssistantName,
			"conversation_language": agentRole.ConversationLanguage,
			"role_description":      agentRole.RoleDescription,
			"voice_model":           agentRole.VoiceModel,
			"current_memory":        agentRole.CurrentMemory,
			"detailed_memory":       agentRole.DetailedMemory,
			"temperature":           agentRole.Temperature,
			"max_length":            agentRole.MaxLength,
			"updated_at":            agentRole.UpdatedAt,
		}
		return dao.UpdateAgentRole(ctx, agentRole.UserID, agentRole.DeviceID, updates)
	}
}

// ===== 保留原有的 RoleConfig 方法用于向后兼容 =====

// CreateRoleTemplate 创建角色模板
func (dao *RoleDAO) CreateRoleTemplate(ctx context.Context, template *models.RoleTemplate) error {
	return dao.Create(ctx, template)
}

// UpdateRoleTemplate 更新角色模板
func (dao *RoleDAO) UpdateRoleTemplate(ctx context.Context, templateID string, updates map[string]interface{}) error {
	return dao.Update(ctx, "role_templates", map[string]interface{}{
		"id": templateID,
	}, updates)
}

// DeleteRoleTemplate 软删除角色模板（设置enabled为false）
func (dao *RoleDAO) DeleteRoleTemplate(ctx context.Context, templateID string) error {
	return dao.Update(ctx, "role_templates", map[string]interface{}{
		"id": templateID,
	}, map[string]interface{}{
		"enabled": false,
	})
}

// SaveRoleConfig 保存用户角色配置
func (dao *RoleDAO) SaveRoleConfig(ctx context.Context, config *models.RoleConfig) error {
	// 先检查是否已存在该用户和设备的配置
	var existingConfig models.RoleConfig
	err := dao.First(ctx, &existingConfig, QueryOptions{
		Table: "role_configs",
		Where: map[string]interface{}{
			"user_id":   config.UserID,
			"device_id": config.DeviceID,
		},
	})

	if err != nil && err != gorm.ErrRecordNotFound {
		return err // 查询出错
	}

	if err == gorm.ErrRecordNotFound {
		// 不存在，创建新记录
		return dao.Create(ctx, config)
	} else {
		// 已存在，更新记录
		updates := map[string]interface{}{
			"template_id":           config.TemplateID,
			"assistant_name":        config.AssistantName,
			"conversation_language": config.ConversationLanguage,
			"role_description":      config.RoleDescription,
			"voice_model":           config.VoiceModel,
			"current_memory":        config.CurrentMemory,
			"detailed_memory":       config.DetailedMemory,
			"temperature":           config.Temperature,
			"max_length":            config.MaxLength,
		}
		return dao.Update(ctx, "role_configs", map[string]interface{}{
			"user_id":   config.UserID,
			"device_id": config.DeviceID,
		}, updates)
	}
}

// GetRoleConfig 获取用户设备的角色配置
func (dao *RoleDAO) GetRoleConfig(ctx context.Context, userID int64, deviceID string) (*models.RoleConfig, error) {
	var config models.RoleConfig
	err := dao.First(ctx, &config, QueryOptions{
		Table: "role_configs",
		Where: map[string]interface{}{
			"user_id":   userID,
			"device_id": deviceID,
		},
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // 记录不存在返回nil
		}
		return nil, err
	}
	return &config, nil
}
