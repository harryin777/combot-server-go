package mcp

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/log"
	"context"
	"fmt"
	"sync"

	"github.com/sashabaranov/go-openai"
)

const (
	MFuncNameExit        = "exit"
	MFuncNameTime        = "time"
	MFuncNameChangeVoice = "change_voice"
	MFuncNameChangeRole  = "change_role"
	MFuncNamePlayMusic   = "play_music"
)

var FuncNamesMap = new(sync.Map)

type HandlerFunc func(ctx context.Context, args map[string]interface{}) (interface{}, error)

type LocalClient struct {
	tools   []Tool
	mu      sync.RWMutex
	handler map[string]HandlerFunc
	cfg     *configs.Config
}

func NewLocalClient(cfg *configs.Config) *LocalClient {
	c := &LocalClient{
		tools:   make([]Tool, 0),
		handler: make(map[string]HandlerFunc),
		mu:      sync.RWMutex{},
		cfg:     cfg,
	}
	return c
}

func (c *LocalClient) RegisterTools(ctx context.Context) {
	if c.cfg == nil {
		log.Error(ctx, "RegisterTools: config is nil")
		return
	}

	if c.cfg.LocalMCPFun == nil {
		log.Warn(ctx, "RegisterTools: LocalMCPFun is nil")
		return
	}

	if len(c.cfg.LocalMCPFun) == 0 {
		log.Info(ctx, "RegisterTools: LocalMCPFun is empty")
		return
	}

	for _, funcName := range c.cfg.LocalMCPFun {
		FuncNamesMap.Store(funcName, 1)
		if funcName == MFuncNameExit {
			c.AddToolExit(ctx)
			log.Info(ctx, "RegisterTools: exit tool registered")
		} else if funcName == MFuncNameTime {
			c.AddToolTime(ctx)
			log.Info(ctx, "RegisterTools: time tool registered")
		} else if funcName == MFuncNameChangeVoice {
			c.AddToolChangeVoice(ctx)
			log.Info(ctx, "RegisterTools: change_voice tool registered")
		} else if funcName == MFuncNameChangeRole {
			c.AddToolChangeRole(ctx)
			log.Info(ctx, "RegisterTools: change_role tool registered")
		} else if funcName == MFuncNamePlayMusic {
			c.AddToolPlayMusic(ctx)
			log.Info(ctx, "RegisterTools: play_music tool registered")
		} else {
			log.Warnf(ctx, "RegisterTools: unknown function name %v", funcName)
		}
	}
}

// Start 启动本地MCP客户端
func (c *LocalClient) Start(ctx context.Context) error {
	c.RegisterTools(ctx)
	log.Info(ctx, "Local MCP client started")
	return nil
}

// Stop 停止本地MCP客户端
func (c *LocalClient) Stop() {
	// 本地客户端不需要停止任何服务，直接返回
}

// HasTool 检查本地客户端是否有指定名称的工具
func (c *LocalClient) HasTool(name string) bool {
	// 如果有local_前缀，则去掉前缀
	if len(name) > 6 && name[:6] == "local_" {
		name = name[6:]
	}
	if _, ok := FuncNamesMap.Load(name); ok {
		return true
	}
	return false
}

// GetAvailableTools 获取本地客户端的所有可用工具
func (c *LocalClient) GetAvailableTools() []openai.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]openai.Tool, 0, len(c.tools))
	for _, tool := range c.tools {
		openaiTool := openai.Tool{
			Type: "function",
			Function: &openai.FunctionDefinition{
				Name:        fmt.Sprintf("local_%s", tool.Name),
				Description: tool.Description,
				Parameters: map[string]interface{}{
					"type":       tool.InputSchema.Type,
					"properties": tool.InputSchema.Properties,
					"required":   tool.InputSchema.Required,
				},
			},
		}
		result = append(result, openaiTool)
	}
	return result
}

// CallTool 调用本地客户端的指定工具
func (c *LocalClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {

	// 检查工具是否存在
	if !c.HasTool(name) {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	// 如果有local_前缀，则去掉前缀
	if len(name) > 6 && name[:6] == "local_" {
		name = name[6:]
	}
	var handler HandlerFunc
	c.mu.RLock()
	if h, ok := c.handler[name]; ok {
		handler = h
		c.mu.RUnlock()
	} else {
		c.mu.RUnlock()
		return nil, fmt.Errorf("handler for tool %s not found", name)
	}

	return handler(ctx, args)
}

// IsReady 检查本地客户端是否已准备就绪
func (c *LocalClient) IsReady() bool {
	// 本地客户端始终就绪
	return true
}

// ResetConnection 重置本地客户端的连接状态
func (c *LocalClient) ResetConnection() error {
	// 本地客户端没有连接状态，直接返回nil
	return nil
}

func (c *LocalClient) AddTool(name string, description string, input ToolInputSchema, handler HandlerFunc) error {
	if c.HasTool(name) {
		return fmt.Errorf("tool %s already exists", name)
	}

	tool := Tool{
		Name:        name,
		Description: description,
		InputSchema: input,
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.tools = append(c.tools, tool)
	c.handler[name] = handler
	return nil
}
