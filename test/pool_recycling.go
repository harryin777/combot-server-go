//go:build ignore
// +build ignore

package main

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/log"
	"combot-server-go/src/core/pool"
	"combot-server-go/src/log"
	"fmt"
	"log"
	"time"
)

// MockProvider 模拟提供者实现
type MockProvider struct {
	ID       string
	IsActive bool
}

func (m *MockProvider) Initialize() error {
	m.IsActive = true
	fmt.Printf("MockProvider %s initialized\n", m.ID)
	return nil
}

func (m *MockProvider) Cleanup() error {
	m.IsActive = false
	fmt.Printf("MockProvider %s cleaned up\n", m.ID)
	return nil
}

func (m *MockProvider) Reset() error {
	fmt.Printf("MockProvider %s reset\n", m.ID)
	return nil
}

// MockFactory 模拟工厂实现
type MockFactory struct {
	Counter int
}

func (f *MockFactory) Create() (interface{}, error) {
	f.Counter++
	provider := &MockProvider{
		ID:       fmt.Sprintf("mock-%d", f.Counter),
		IsActive: false,
	}
	provider.Initialize()
	return provider, nil
}

func (f *MockFactory) Destroy(resource interface{}) error {
	if provider, ok := resource.(*MockProvider); ok {
		return provider.Cleanup()
	}
	return nil
}

func main() {
	fmt.Println("🚀 开始测试资源池回收机制...")

	// 创建测试用的logger
	config := &configs.Config{}
	config.Log.LogLevel = "debug"
	config.Log.LogDir = "./logs"
	config.Log.LogFile = "test.log"
	logger, err := log.NewLogger(config)
	if err != nil {
		log.Fatal(err)
	}

	// 创建资源池
	factory := &MockFactory{}
	poolConfig := pool.PoolConfig{
		MinSize:       2,
		MaxSize:       5,
		RefillSize:    1,
		CheckInterval: 10 * time.Second,
	}

	// NewResourcePool 现在移除了 logger 参数
	resourcePool, err := pool.NewResourcePool(factory, poolConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer resourcePool.Close()

	fmt.Println("\n📊 初始池状态:")
	printPoolStats(resourcePool)

	// 测试1: 获取资源
	fmt.Println("\n🔧 测试1: 获取资源")
	resources := make([]interface{}, 0)

	for i := 0; i < 3; i++ {
		resource, err := resourcePool.Get()
		if err != nil {
			fmt.Printf("获取资源失败: %v\n", err)
			continue
		}
		resources = append(resources, resource)
		if provider, ok := resource.(*MockProvider); ok {
			fmt.Printf("✅ 获取到资源: %s\n", provider.ID)
		}
	}

	fmt.Println("\n📊 获取资源后的池状态:")
	printPoolStats(resourcePool)

	// 测试2: 归还资源
	fmt.Println("\n🔄 测试2: 归还资源")
	for i, resource := range resources {
		if provider, ok := resource.(*MockProvider); ok {
			fmt.Printf("🔄 归还资源: %s\n", provider.ID)
		}

		err := resourcePool.Put(resource)
		if err != nil {
			fmt.Printf("归还资源失败: %v\n", err)
		} else {
			fmt.Printf("✅ 资源 %d 已成功归还\n", i+1)
		}
	}

	fmt.Println("\n📊 归还资源后的池状态:")
	printPoolStats(resourcePool)

	// 测试3: 重复使用资源
	fmt.Println("\n🔄 测试3: 重复使用资源")
	resource1, err := resourcePool.Get()
	if err != nil {
		fmt.Printf("获取资源失败: %v\n", err)
	} else {
		if provider, ok := resource1.(*MockProvider); ok {
			fmt.Printf("✅ 重复获取到资源: %s (应该是复用的)\n", provider.ID)
		}
		resourcePool.Put(resource1)
	}

	// 测试4: 超出最大容量
	fmt.Println("\n⚠️  测试4: 超出最大容量")
	allResources := make([]interface{}, 0)

	for i := 0; i < 7; i++ { // 超过最大容量5
		resource, err := resourcePool.Get()
		if err != nil {
			fmt.Printf("❌ 获取资源 %d 失败: %v\n", i+1, err)
			break
		} else {
			if provider, ok := resource.(*MockProvider); ok {
				fmt.Printf("✅ 获取资源 %d: %s\n", i+1, provider.ID)
			}
			allResources = append(allResources, resource)
		}
	}

	fmt.Println("\n📊 达到最大容量时的池状态:")
	printPoolStats(resourcePool)

	// 归还所有资源
	fmt.Println("\n🔄 归还所有资源:")
	for _, resource := range allResources {
		resourcePool.Put(resource)
	}

	fmt.Println("\n📊 最终池状态:")
	printPoolStats(resourcePool)

	// 测试5: 池满时归还资源
	fmt.Println("\n🧪 测试5: 池满时归还多余资源")

	// 先填满池子
	for i := 0; i < 5; i++ {
		resource, _ := resourcePool.Get()
		resourcePool.Put(resource)
	}

	// 尝试归还额外的资源（应该被销毁）
	extraResource, _ := factory.Create()
	if provider, ok := extraResource.(*MockProvider); ok {
		fmt.Printf("🔄 尝试归还额外资源: %s (池已满，应该被销毁)\n", provider.ID)
	}
	err = resourcePool.Put(extraResource)
	if err != nil {
		fmt.Printf("✅ 预期行为: %v\n", err)
	}

	fmt.Println("\n📊 最终池状态:")
	printPoolStats(resourcePool)

	fmt.Println("\n🎉 资源池回收机制测试完成!")
}

func printPoolStats(pool *pool.ResourcePool) {
	stats := pool.GetDetailedStats()
	fmt.Printf("  Available: %d | Total: %d | Max: %d | Min: %d | In-use: %d\n",
		stats["available"], stats["total"], stats["max"], stats["min"], stats["in_use"])
}
