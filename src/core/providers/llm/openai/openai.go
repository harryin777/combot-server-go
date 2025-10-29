package openai

import (
	"combot-server-go/src/core/providers/llm"
	"combot-server-go/src/core/types"
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// Provider OpenAI LLM提供者
type Provider struct {
	*llm.BaseProvider
	client    *openai.Client
	maxTokens int
}

// 注册提供者
func init() {
	llm.Register("openai", NewProvider)
}

// NewProvider 创建OpenAI提供者
func NewProvider(config *llm.Config) (llm.Provider, error) {
	base := llm.NewBaseProvider(config)
	provider := &Provider{
		BaseProvider: base,
		maxTokens:    config.MaxTokens,
	}
	if provider.maxTokens <= 0 {
		provider.maxTokens = 500
	}

	return provider, nil
}

// Initialize 初始化提供者
func (p *Provider) Initialize(ctx context.Context) error {
	config := p.Config()
	if config.APIKey == "" {
		return fmt.Errorf("missing OpenAI API key")
	}

	clientConfig := openai.DefaultConfig(config.APIKey)
	if config.BaseURL != "" {
		clientConfig.BaseURL = config.BaseURL
	}

	p.client = openai.NewClientWithConfig(clientConfig)
	return nil
}

// Cleanup 清理资源
func (p *Provider) Cleanup(ctx context.Context) error {
	return nil
}

// Response types.LLMProvider接口实现
func (p *Provider) Response(ctx context.Context, sessionID string, messages []types.Message) (<-chan string, error) {
	responseChan := make(chan string, 10)

	go func() {
		defer close(responseChan)

		// 转换消息格式
		chatMessages := make([]openai.ChatCompletionMessage, len(messages))
		for i, msg := range messages {
			chatMessage := openai.ChatCompletionMessage{
				Role:    msg.Role,
				Content: msg.Content,
			}

			// 处理tool_call_id字段（tool消息必需）
			if msg.ToolCallID != "" {
				chatMessage.ToolCallID = msg.ToolCallID
			}

			// 处理tool_calls字段（assistant消息中的工具调用）
			if len(msg.ToolCalls) > 0 {
				openaiToolCalls := make([]openai.ToolCall, len(msg.ToolCalls))
				for j, tc := range msg.ToolCalls {
					openaiToolCalls[j] = openai.ToolCall{
						ID:   tc.ID,
						Type: openai.ToolType(tc.Type),
						Function: openai.FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
				chatMessage.ToolCalls = openaiToolCalls
			}

			chatMessages[i] = chatMessage
		}

		stream, err := p.client.CreateChatCompletionStream(
			ctx,
			openai.ChatCompletionRequest{
				Model:     p.Config().ModelName,
				Messages:  chatMessages,
				Stream:    true,
				MaxTokens: p.maxTokens,
			},
		)
		if err != nil {
			responseChan <- fmt.Sprintf("【OpenAI服务响应异常: %v】", err)
			return
		}
		defer stream.Close()

		isActive := true
		for {
			response, err := stream.Recv()
			if err != nil {
				break
			}

			if len(response.Choices) > 0 {
				content := response.Choices[0].Delta.Content
				if content != "" {
					// 处理思考标签
					if content, isActive = handleThinkTags(content, isActive); content != "" {
						responseChan <- content
					}
				}
			}
		}
	}()

	return responseChan, nil
}

// ResponseWithFunctions types.LLMProvider接口实现
// 支持工具调用的流式聊天完成接口，返回一个channel供调用方逐块接收响应
func (p *Provider) ResponseWithFunctions(ctx context.Context, sessionID string, messages []types.Message, tools []openai.Tool) (<-chan types.Response, error) {
	// 创建带缓冲的响应通道，避免阻塞
	responseChan := make(chan types.Response, 10)

	// 启动异步协程处理流式响应，避免阻塞调用方
	go func() {
		// 确保通道在协程结束时关闭，通知调用方数据传输完成
		defer close(responseChan)

		// ========== 步骤1: 消息格式转换 ==========
		// 将内部Message格式转换为OpenAI SDK要求的ChatCompletionMessage格式
		chatMessages := make([]openai.ChatCompletionMessage, len(messages))
		for i, msg := range messages {
			chatMessage := openai.ChatCompletionMessage{
				Role:    msg.Role,    // 角色: system/user/assistant/tool
				Content: msg.Content, // 消息内容
			}

			// 处理工具调用相关字段
			// tool_call_id: tool消息回复assistant的工具调用时必需
			if msg.ToolCallID != "" {
				chatMessage.ToolCallID = msg.ToolCallID
			}

			// tool_calls: assistant消息中包含的工具调用请求
			if len(msg.ToolCalls) > 0 {
				openaiToolCalls := make([]openai.ToolCall, len(msg.ToolCalls))
				for j, tc := range msg.ToolCalls {
					openaiToolCalls[j] = openai.ToolCall{
						ID:   tc.ID,
						Type: openai.ToolType(tc.Type),
						Function: openai.FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
				chatMessage.ToolCalls = openaiToolCalls
			}

			chatMessages[i] = chatMessage
		}

		// ========== 步骤2: 创建流式请求 ==========
		// 使用OpenAI SDK创建支持工具调用的流式聊天完成请求
		stream, err := p.client.CreateChatCompletionStream(
			ctx,
			openai.ChatCompletionRequest{
				Model:    p.Config().ModelName, // 模型名称 (如: gpt-4, gpt-3.5-turbo)
				Messages: chatMessages,         // 转换后的消息列表
				Tools:    tools,                // 可用的工具函数列表
				Stream:   true,                 // 启用流式响应
			},
		)
		if err != nil {
			// 连接失败时发送错误响应并退出
			responseChan <- types.Response{
				Content: fmt.Sprintf("【OpenAI服务响应异常: %v】", err),
				Error:   err.Error(),
			}
			return
		}
		// 确保流在协程结束时关闭
		defer stream.Close()

		// ========== 步骤3: 处理流式响应 ==========
		// 持续接收OpenAI返回的数据块，直到流结束
		for {
			response, err := stream.Recv()
			if err != nil {
				// 流结束或出错时退出循环
				break
			}

			// 检查响应是否包含有效的选择项
			if len(response.Choices) > 0 {
				// 提取增量数据 (delta)，包含本次新增的内容
				delta := response.Choices[0].Delta

				// 构造统一的响应格式
				chunk := types.Response{
					Content: delta.Content, // 文本内容（可能为空，特别是工具调用时）
				}

				// ========== 步骤4: 处理工具调用 ==========
				// 检查是否包含工具调用请求
				if len(delta.ToolCalls) > 0 {
					// 转换工具调用格式为内部格式
					toolCalls := make([]types.ToolCall, len(delta.ToolCalls))
					for i, tc := range delta.ToolCalls {
						toolCalls[i] = types.ToolCall{
							ID:   tc.ID,           // 工具调用唯一标识
							Type: string(tc.Type), // 调用类型 (通常为 "function")
							Function: types.FunctionCall{
								Name:      tc.Function.Name,      // 函数名称
								Arguments: tc.Function.Arguments, // 函数参数 (JSON字符串)
							},
						}
					}
					chunk.ToolCalls = toolCalls
				}

				// ========== 步骤5: 发送响应块 ==========
				// 将处理后的响应块发送到通道，供外部消费者处理
				responseChan <- chunk
			}
		}
	}()

	// 立即返回通道，调用方可以开始接收数据
	return responseChan, nil
}

// handleThinkTags 处理思考标签
func handleThinkTags(content string, isActive bool) (string, bool) {
	if content == "" {
		return "", isActive
	}

	if content == "<think>" {
		return "", false
	}
	if content == "</think>" {
		return "", true
	}

	if !isActive {
		return "", isActive
	}

	return content, isActive
}
