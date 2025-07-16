package ollama

import (
	"xiaozhi-server-go/src/core/providers/vlllm"

	"github.com/sirupsen/logrus"
)

// OllamaVLLMProvider Ollama类型的VLLLM提供者
type OllamaVLLMProvider struct {
	*vlllm.Provider
}

// NewProvider 创建Ollama VLLLM提供者实例
func NewProvider(config *vlllm.Config) (*vlllm.Provider, error) {
	// 直接使用基础VLLLM Provider，因为它已经复用了LLM架构
	// Ollama类型的VLLLM只需要确保使用正确的模型名称（如qwen2-vl:7b）
	provider, err := vlllm.NewProvider(config)
	if err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"model_name": config.ModelName,
		"base_url":   config.BaseURL,
	}).Debug("Ollama VLLLM Provider创建成功")

	return provider, nil
}

// init 注册Ollama VLLLM提供者
func init() {
	vlllm.Register("ollama", NewProvider)
}
