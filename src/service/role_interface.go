package service

import (
	"combot-server-go/src/models"
	"context"
)

// RoleService 定义角色服务接口
type RoleService interface {
	// GetRoleTemplates 获取角色模板列表
	GetRoleTemplates(ctx context.Context) ([]models.RoleTemplate, int, error)

	// GetRoleTemplate 获取角色模板详情
	GetRoleTemplate(ctx context.Context, templateID string) (*models.RoleTemplate, int, error)

	// SaveRoleConfig 保存角色配置
	SaveRoleConfig(ctx context.Context, userID int64, config *models.SaveRoleConfigRequest) (int, error)
}
