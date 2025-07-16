package pool

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ResourceFactory 资源工厂接口
type ResourceFactory interface {
	Create() (interface{}, error)
	Destroy(resource interface{}) error
}

// PoolConfig 资源池配置
type PoolConfig struct {
	MinSize       int           // 最小池大小
	MaxSize       int           // 最大池大小
	RefillSize    int           // 重新填充大小
	CheckInterval time.Duration // 检查间隔
}

// ResourcePool 资源池
type ResourcePool struct {
	factory    ResourceFactory
	config     PoolConfig
	resources  chan interface{}
	totalCount int
	inUseCount int
	mu         sync.RWMutex
	closeOnce  sync.Once
	closed     bool
	stopChan   chan struct{}
}

// NewResourcePool 创建新的资源池
func NewResourcePool(factory ResourceFactory, config PoolConfig) (*ResourcePool, error) {
	if factory == nil {
		return nil, fmt.Errorf("factory不能为空")
	}
	if config.MinSize < 0 || config.MaxSize < config.MinSize {
		return nil, fmt.Errorf("无效的池配置: MinSize=%d, MaxSize=%d", config.MinSize, config.MaxSize)
	}

	pool := &ResourcePool{
		factory:   factory,
		config:    config,
		resources: make(chan interface{}, config.MaxSize),
		stopChan:  make(chan struct{}),
	}

	// 初始化最小数量的资源
	for i := 0; i < config.MinSize; i++ {
		resource, err := factory.Create()
		if err != nil {
			// 清理已创建的资源
			pool.Close()
			return nil, fmt.Errorf("创建初始资源失败: %v", err)
		}
		pool.resources <- resource
		pool.totalCount++
	}

	// 启动定期维护
	go pool.maintain()

	return pool, nil
}

// Get 获取资源
func (p *ResourcePool) Get() (interface{}, error) {
	p.mu.RLock()
	if p.closed {
		p.mu.RUnlock()
		return nil, fmt.Errorf("资源池已关闭")
	}
	p.mu.RUnlock()

	select {
	case resource := <-p.resources:
		p.mu.Lock()
		p.inUseCount++
		p.mu.Unlock()
		return resource, nil
	default:
		// 没有可用资源，尝试创建新的
		p.mu.Lock()
		if p.totalCount < p.config.MaxSize {
			resource, err := p.factory.Create()
			if err != nil {
				p.mu.Unlock()
				return nil, fmt.Errorf("创建新资源失败: %v", err)
			}
			p.totalCount++
			p.inUseCount++
			p.mu.Unlock()
			return resource, nil
		}
		p.mu.Unlock()
		return nil, fmt.Errorf("资源池已满，无法获取资源")
	}
}

// Put 归还资源
func (p *ResourcePool) Put(resource interface{}) error {
	if resource == nil {
		return fmt.Errorf("资源不能为空")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		// 池已关闭，销毁资源
		return p.factory.Destroy(resource)
	}

	p.inUseCount--

	select {
	case p.resources <- resource:
		return nil
	default:
		// 池已满，销毁多余资源
		p.totalCount--
		return p.factory.Destroy(resource)
	}
}

// Reset 重置资源状态
func (p *ResourcePool) Reset(resource interface{}) error {
	if resetter, ok := resource.(interface{ Reset() error }); ok {
		return resetter.Reset()
	}
	return nil
}

// GetStats 获取统计信息
func (p *ResourcePool) GetStats() (available int, total int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.resources), p.totalCount
}

// GetDetailedStats 获取详细统计信息
func (p *ResourcePool) GetDetailedStats() map[string]int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return map[string]int{
		"available": len(p.resources),
		"total":     p.totalCount,
		"in_use":    p.inUseCount,
		"max":       p.config.MaxSize,
		"min":       p.config.MinSize,
	}
}

// Close 关闭资源池
func (p *ResourcePool) Close() {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		close(p.stopChan)
		p.mu.Unlock()

		// 清理所有资源
		close(p.resources)
		for resource := range p.resources {
			if err := p.factory.Destroy(resource); err != nil {
				logrus.WithError(err).Error("销毁资源失败")
			}
		}
	})
}

// maintain 维护资源池
func (p *ResourcePool) maintain() {
	ticker := time.NewTicker(p.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.refillPool()
		}
	}
}

// refillPool 重新填充资源池
func (p *ResourcePool) refillPool() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	currentAvailable := len(p.resources)
	if currentAvailable < p.config.MinSize {
		needed := p.config.MinSize - currentAvailable
		if needed > p.config.RefillSize {
			needed = p.config.RefillSize
		}

		for i := 0; i < needed && p.totalCount < p.config.MaxSize; i++ {
			resource, err := p.factory.Create()
			if err != nil {
				logrus.WithError(err).Error("重新填充资源失败")
				continue
			}

			select {
			case p.resources <- resource:
				p.totalCount++
			default:
				// 池已满，销毁资源
				p.factory.Destroy(resource)
			}
		}
	}
}
