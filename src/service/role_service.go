package service

import (
	"context"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/codes"
	"xiaozhi-server-go/src/models"
)

type roleService struct {
	config *configs.Config
}

// NewRoleService 创建角色服务实例
func NewRoleService(config *configs.Config) RoleService {
	return &roleService{
		config: config,
	}
}

// GetRoleTemplates 获取角色模板列表
func (s *roleService) GetRoleTemplates(ctx context.Context) ([]models.RoleTemplate, int, error) {
	// 目前返回预定义的模板列表，后续可以从数据库读取
	templates := []models.RoleTemplate{
		{
			ID:              "taiwan_girlfriend",
			Name:            "台湾女友",
			Description:     "我是一个名叫【assistant_name】的8岁小女友。\n别看我年纪小，我可是有着潜满的好奇心呢。\n我特别喜爱双蛋（红烧肉大马力），里面的每一个章节都让我着迷。\n我最喜欢男人敏锐智慧的问答，定会给你注过怍做好各种对策的任务。\n还有可爱的天天，它可以受喜爱直升机在天空中翱翔，执行救援任务的时候特别帅呢。",
			AssistantName:   "小雪",
			DefaultVoice:    "longhua_female_high",
			DefaultLanguage: "普通话",
		},
		{
			ID:              "shangguan_zi",
			Name:            "上官子",
			Description:     "我是上官子，一个温文尔雅、知书达理的古风美女。擅长诗词歌赋，喜欢与人分享古典文化的魅力。",
			AssistantName:   "上官子",
			DefaultVoice:    "longhua_female_high",
			DefaultLanguage: "普通话",
		},
		{
			ID:              "english_tutor",
			Name:            "English Tutor",
			Description:     "I am your English tutor, dedicated to helping you improve your English skills through engaging conversations and practical exercises.",
			AssistantName:   "Emma",
			DefaultVoice:    "english_female",
			DefaultLanguage: "English",
		},
		{
			ID:              "curious_teacher",
			Name:            "好奇小朋友",
			Description:     "我是一个充满好奇心的小朋友，喜欢问各种有趣的问题，和你一起探索这个奇妙的世界！",
			AssistantName:   "小好奇",
			DefaultVoice:    "child_voice",
			DefaultLanguage: "普通话",
		},
		{
			ID:              "wang_sulong_assistant",
			Name:            "汪苏泷队长",
			Description:     "我是汪苏泷的专属助手，负责处理日常事务和与粉丝的互动。",
			AssistantName:   "小泷",
			DefaultVoice:    "male_voice",
			DefaultLanguage: "普通话",
		},
	}

	return templates, codes.CodeSuccess, nil
}

// GetRoleTemplate 获取角色模板详情
func (s *roleService) GetRoleTemplate(ctx context.Context, templateID string) (*models.RoleTemplate, int, error) {
	templates, _, _ := s.GetRoleTemplates(ctx)

	for _, template := range templates {
		if template.ID == templateID {
			return &template, codes.CodeSuccess, nil
		}
	}

	return nil, codes.CodeNotFound, nil
}

// SaveRoleConfig 保存角色配置
func (s *roleService) SaveRoleConfig(ctx context.Context, userID int64, config *models.SaveRoleConfigRequest) (int, error) {
	// TODO: 这里应该保存到数据库
	// 目前只是模拟成功
	//
	// 实际实现时需要：
	// 1. 验证用户是否有权限配置该设备
	// 2. 将配置保存到数据库
	// 3. 可能需要通知设备更新配置

	return codes.CodeSuccess, nil
}
