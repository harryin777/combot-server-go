package core

import (
	"combot-server-go/src/core/pool"
	"combot-server-go/src/log"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// ConnectionState 连接状态枚举
type ConnectionState int32

const (
	// ConnectionStateActive 连接活跃状态
	ConnectionStateActive ConnectionState = 0
	// ConnectionStateClosed 连接已关闭状态
	ConnectionStateClosed ConnectionState = 1
)

// ConnectionContext 连接上下文，用于跟踪资源分配和连接生命周期管理
// 重构版本：遵循 Go 最佳实践，不在结构体中存储 context.Context
type ConnectionContext struct {
	// 连接相关资源
	handler     *ConnectionHandler
	providerSet *pool.ProviderSet
	poolManager *pool.PoolManager
	conn        Connection

	// 连接标识和状态
	clientID string
	state    int32 // 使用 ConnectionState，原子操作

	// 生命周期管理
	cancel context.CancelFunc // 用于取消连接级别的操作

	// 并发安全
	mu sync.RWMutex // 保护资源访问
}

// ConnectionConfig 连接配置参数
type ConnectionConfig struct {
	Handler     *ConnectionHandler
	ProviderSet *pool.ProviderSet
	PoolManager *pool.PoolManager
	ClientID    string
	Conn        Connection
	Cancel      context.CancelFunc
}

// NewConnectionContext 创建新的连接上下文
// 使用配置结构体，更易于扩展和维护
func NewConnectionContext(config ConnectionConfig) (*ConnectionContext, error) {
	if config.ClientID == "" {
		return nil, errors.New("clientID不能为空")
	}
	if config.Conn == nil {
		return nil, errors.New("connection不能为空")
	}

	return &ConnectionContext{
		handler:     config.Handler,
		providerSet: config.ProviderSet,
		poolManager: config.PoolManager,
		clientID:    config.ClientID,
		conn:        config.Conn,
		cancel:      config.Cancel,
		state:       int32(ConnectionStateActive),
	}, nil
}

// GetClientID 获取客户端ID
func (c *ConnectionContext) GetClientID() string {
	return c.clientID
}

// IsActive 检查连接是否仍然活跃
func (c *ConnectionContext) IsActive() bool {
	return ConnectionState(atomic.LoadInt32(&c.state)) == ConnectionStateActive
}

// GetState 获取当前连接状态
func (c *ConnectionContext) GetState() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&c.state))
}

// GetHandler 安全地获取连接处理器
func (c *ConnectionContext) GetHandler() *ConnectionHandler {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.handler
}

// GetConnection 安全地获取连接对象
func (c *ConnectionContext) GetConnection() Connection {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// CreateSafeCallback 创建安全的回调函数
// context.Context 作为参数传入，符合 Go 最佳实践
func (c *ConnectionContext) CreateSafeCallback(ctx context.Context) func(func(*ConnectionHandler)) func() {
	return func(callback func(*ConnectionHandler)) func() {
		return func() {
			// 检查连接是否仍然活跃
			if !c.IsActive() {
				log.Infof(ctx, "客户端 %s 连接已关闭，跳过回调", c.clientID)
				return
			}

			// 检查传入的上下文是否已取消
			select {
			case <-ctx.Done():
				log.Infof(ctx, "客户端 %s 上下文已取消，跳过回调", c.clientID)
				return
			default:
			}

			// 安全地获取处理器并执行回调
			handler := c.GetHandler()
			if handler != nil {
				callback(handler)
			}
		}
	}
}

// ExecuteWithContext 在给定的上下文中安全地执行操作
func (c *ConnectionContext) ExecuteWithContext(ctx context.Context, operation func(*ConnectionHandler) error) error {
	if !c.IsActive() {
		return errors.New("连接已关闭")
	}

	// 检查上下文
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 安全地获取处理器
	handler := c.GetHandler()
	if handler == nil {
		return errors.New("连接处理器不可用")
	}

	return operation(handler)
}

// Close 关闭连接并归还资源
// context.Context 作为参数传入，用于控制关闭操作的超时等
func (c *ConnectionContext) Close(ctx context.Context) error {
	// 使用原子操作标记为已关闭，确保只关闭一次
	if !atomic.CompareAndSwapInt32(&c.state, int32(ConnectionStateActive), int32(ConnectionStateClosed)) {
		return nil // 已经关闭过了
	}

	log.Infof(ctx, "开始关闭客户端 %s 的连接", c.clientID)

	// 取消连接级别的操作
	if c.cancel != nil {
		c.cancel()
	}

	// 使用锁保护关闭操作
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error

	// 关闭连接处理器
	if c.handler != nil {
		c.handler.Close()
		c.handler = nil // 清空引用
	}

	// 关闭WebSocket连接
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil // 清空引用
	}

	// 归还资源到池中
	if c.providerSet != nil && c.poolManager != nil {
		if err := c.poolManager.ReturnProviderSet(ctx, c.providerSet); err != nil {
			errs = append(errs, fmt.Errorf("归还资源失败: %w", err))
			log.Errorf(ctx, "客户端 %s 归还资源失败: %v", c.clientID, err)
		} else {
			log.Infof(ctx, "客户端 %s 资源已成功归还到池中", c.clientID)
		}
		c.providerSet = nil // 清空引用
		c.poolManager = nil // 清空引用
	}

	if len(errs) > 0 {
		return fmt.Errorf("关闭连接时发生 %d 个错误: %w", len(errs), errors.Join(errs...))
	}

	log.Infof(ctx, "客户端 %s 连接关闭完成", c.clientID)
	return nil
}

// String 实现 Stringer 接口，便于调试和日志
func (c *ConnectionContext) String() string {
	state := "active"
	if !c.IsActive() {
		state = "closed"
	}
	return fmt.Sprintf("ConnectionContext{clientID: %s, state: %s}", c.clientID, state)
}
