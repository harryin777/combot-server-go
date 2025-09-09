package vlllm

import (
	"combot-server-go/src/log"
	"context"
	"fmt"

	"combot-server-go/src/configs"
)

// Factory VLLLM工厂函数类型
type Factory func(config *Config) (*Provider, error)

var (
	factories = make(map[string]Factory)
)

// Register 注册VLLLM提供者工厂
func Register(name string, factory Factory) {
	factories[name] = factory
}

// Create 创建VLLLM提供者实例
func Create(ctx context.Context, name string, vlllmConfig *configs.VLLMConfig) (*Provider, error) {
	factory, ok := factories[name]
	if !ok {
		return nil, fmt.Errorf("未知的VLLLM提供者: %s", name)
	}

	// 转换配置格式
	config := &Config{
		Type:        vlllmConfig.Type,
		ModelName:   vlllmConfig.ModelName,
		BaseURL:     vlllmConfig.BaseURL,
		APIKey:      vlllmConfig.APIKey,
		Temperature: vlllmConfig.Temperature,
		MaxTokens:   vlllmConfig.MaxTokens,
		TopP:        vlllmConfig.TopP,
		Security:    vlllmConfig.Security,
		Data:        vlllmConfig.Extra,
	}

	// 创建提供者实例
	provider, err := factory(config)
	if err != nil {
		return nil, fmt.Errorf("创建VLLLM提供者失败: %v", err)
	}

	// 初始化提供者
	if err := provider.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("初始化VLLLM提供者失败: %v", err)
	}

	log.Infof(ctx, "VLLLM提供者创建成功，名称: %s, 类型: %s, 模型: %s",
		name, config.Type, config.ModelName)

	return provider, nil
}

// GetRegisteredProviders 获取已注册的提供者列表
func GetRegisteredProviders() []string {
	var providers []string
	for name := range factories {
		providers = append(providers, name)
	}
	return providers
}
