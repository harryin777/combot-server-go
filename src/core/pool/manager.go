package pool

import (
	"combot-server-go/src/configs"
	"combot-server-go/src/core/mcp"
	"combot-server-go/src/core/providers"
	"combot-server-go/src/core/providers/vlllm"
	"combot-server-go/src/core/utils"
	"context"
	"fmt"
	"time"
)

// PoolManager 资源池管理器
type PoolManager struct {
	asrPool   *ResourcePool
	llmPool   *ResourcePool
	ttsPool   *ResourcePool
	vlllmPool *ResourcePool
	mcpPool   *ResourcePool
}

// ProviderSet 提供者集合
type ProviderSet struct {
	ASR   providers.ASRProvider
	LLM   providers.LLMProvider
	TTS   providers.TTSProvider
	VLLLM *vlllm.Provider
	MCP   *mcp.Manager
}

// NewPoolManager 创建资源池管理器
func NewPoolManager(ctx context.Context, config *configs.Config) (*PoolManager, error) {
	pm := &PoolManager{}

	// 暂时跳过连通性检查
	// if err := pm.performConnectivityCheck(config, logrus.New()); err != nil {
	// 	return nil, fmt.Errorf("资源连通性检查失败: %v", err)
	// }

	poolConfig := PoolConfig{
		MinSize:       5,
		MaxSize:       20,
		RefillSize:    3,
		CheckInterval: 30 * time.Second,
	}

	// 检查配置是否包含所需的模块

	// 初始化ASR池
	if asrType, ok := config.SelectedModule["ASR"]; ok && asrType != "" {
		asrFactory := NewASRFactory(asrType, config)
		if asrFactory == nil {
			return nil, fmt.Errorf("创建ASR工厂失败: 找不到配置 %s", asrType)
		}
		asrPool, err := NewResourcePool(ctx, asrFactory, poolConfig)
		if err != nil {
			return nil, fmt.Errorf("初始化ASR资源池失败: %v", err)
		}
		pm.asrPool = asrPool
		_, cnt := asrPool.GetStats()
		utils.Infof(ctx, "ASR资源池初始化成功，类型: %s, 数量：%d", asrType, cnt)
	}

	// 初始化LLM池
	if llmType, ok := config.SelectedModule["LLM"]; ok && llmType != "" {
		llmFactory := NewLLMFactory(llmType, config)
		if llmFactory == nil {
			return nil, fmt.Errorf("创建LLM工厂失败: 找不到配置 %s", llmType)
		}
		llmPool, err := NewResourcePool(ctx, llmFactory, poolConfig)
		if err != nil {
			return nil, fmt.Errorf("初始化LLM资源池失败: %v", err)
		}
		pm.llmPool = llmPool
		_, cnt := llmPool.GetStats()
		utils.Infof(ctx, "LLM资源池初始化成功，类型: %v, 数量：%v", llmType, cnt)
	}

	// 初始化TTS池
	if ttsType, ok := config.SelectedModule["TTS"]; ok && ttsType != "" {
		ttsFactory := NewTTSFactory(ttsType, config)
		if ttsFactory == nil {
			return nil, fmt.Errorf("创建TTS工厂失败: 找不到配置 %s", ttsType)
		}
		ttsPool, err := NewResourcePool(ctx, ttsFactory, poolConfig)
		if err != nil {
			return nil, fmt.Errorf("初始化TTS资源池失败: %v", err)
		}
		pm.ttsPool = ttsPool
		_, cnt := ttsPool.GetStats()
		utils.Infof(ctx, "TTS资源池初始化成功，类型: %v, 数量：%v", ttsType, cnt)
	}

	// 初始化VLLLM池（可选）
	if vlllmType, ok := config.SelectedModule["VLLLM"]; ok && vlllmType != "" {
		vlllmFactory := NewVLLLMFactory(vlllmType, config)
		if vlllmFactory == nil {
			utils.WithField(ctx, "type", vlllmType).Warn("创建VLLLM工厂失败: 找不到配置")
		} else {
			vlllmPool, err := NewResourcePool(ctx, vlllmFactory, poolConfig)
			if err != nil {
				utils.WithError(ctx, err).Warn("初始化VLLLM资源池失败（将继续使用普通LLM）")
			} else {
				pm.vlllmPool = vlllmPool
			}
		}
		if pm.vlllmPool != nil {
			_, cnt := pm.vlllmPool.GetStats()
			utils.Infof(ctx, "VLLLM资源池初始化成功，类型: %v, 数量：%v", vlllmType, cnt)
		} else {
			utils.Warn(ctx, "VLLLM资源池未初始化，将使用普通LLM")
		}
	}

	poolConfig = PoolConfig{
		MinSize:       2,
		MaxSize:       20,
		RefillSize:    1,
		CheckInterval: 30 * time.Second,
	}

	// 初始化MCP池（总是初始化，因为MCP是核心功能）
	utils.Info(ctx, "开始初始化MCP资源池，请等待...")
	mcpFactory := NewMCPFactory(config)
	if mcpFactory != nil {
		mcpPool, err := NewResourcePool(ctx, mcpFactory, poolConfig)
		if err != nil {
			return nil, fmt.Errorf("初始化MCP资源池失败: %v", err)
		}
		pm.mcpPool = mcpPool
		_, cnt := mcpPool.GetStats()
		utils.Infof(ctx, "MCP资源池初始化成功 count:%v", cnt)
	} else {
		utils.Warn(ctx, "创建MCP工厂失败，MCP功能将不可用")
	}

	return pm, nil
}

// GetProviderSet 获取一套提供者
func (pm *PoolManager) GetProviderSet(ctx context.Context) (*ProviderSet, error) {
	set := &ProviderSet{}

	if pm.asrPool != nil {
		asr, err := pm.asrPool.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("获取ASR提供者失败: %v", err)
		}
		set.ASR = asr.(providers.ASRProvider)
	}

	if pm.llmPool != nil {
		llm, err := pm.llmPool.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("获取LLM提供者失败: %v", err)
		}
		set.LLM = llm.(providers.LLMProvider)
	}

	if pm.ttsPool != nil {
		tts, err := pm.ttsPool.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("获取TTS提供者失败: %v", err)
		}
		set.TTS = tts.(providers.TTSProvider)
	}

	if pm.vlllmPool != nil {
		vlllmProvider, err := pm.vlllmPool.Get(ctx)
		if err == nil {
			// 直接转换，因为我们知道这是从 vlllm 工厂创建的
			set.VLLLM = vlllmProvider.(*vlllm.Provider)
		}
	}

	if pm.mcpPool != nil {
		mcpManager, err := pm.mcpPool.Get(ctx)
		if err == nil {
			// 直接转换，因为我们知道这是从 mcp 工厂创建的
			set.MCP = mcpManager.(*mcp.Manager)
		}
	}

	return set, nil
}

// Close 关闭所有资源池
func (pm *PoolManager) Close(ctx context.Context) error {
	if pm.asrPool != nil {
		pm.asrPool.Close(ctx)
	}
	if pm.llmPool != nil {
		pm.llmPool.Close(ctx)
	}
	if pm.ttsPool != nil {
		pm.ttsPool.Close(ctx)
	}
	if pm.vlllmPool != nil {
		pm.vlllmPool.Close(ctx)
	}
	if pm.mcpPool != nil {
		pm.mcpPool.Close(ctx)
	}

	return nil
}

// ReturnProviderSet 归还提供者集合到池中
func (pm *PoolManager) ReturnProviderSet(ctx context.Context, set *ProviderSet) error {
	if set == nil {
		return fmt.Errorf("提供者集合为空，无法归还")
	}

	var errs []error

	// 归还ASR提供者
	if set.ASR != nil && pm.asrPool != nil {
		// 重置资源状态
		if err := pm.asrPool.Reset(set.ASR); err != nil {
			utils.Error(ctx, "重置ASR资源状态失败")
		}
		// 归还到池中
		if err := pm.asrPool.Put(ctx, set.ASR); err != nil {
			errs = append(errs, fmt.Errorf("归还ASR提供者失败: %v", err))
			utils.Error(ctx, "归还ASR提供者失败")
		} else {
			utils.Debug(ctx, "ASR提供者已成功归还到池中")
		}
	}

	// 归还LLM提供者
	if set.LLM != nil && pm.llmPool != nil {
		if err := pm.llmPool.Reset(set.LLM); err != nil {
			utils.WithError(ctx, err).Warn("重置LLM资源状态失败")
		}
		if err := pm.llmPool.Put(ctx, set.LLM); err != nil {
			errs = append(errs, fmt.Errorf("归还LLM提供者失败: %v", err))
			utils.WithError(ctx, err).Error("归还LLM提供者失败")
		} else {
			utils.Debug(ctx, "LLM提供者已成功归还到池中")
		}
	}

	// 归还TTS提供者
	if set.TTS != nil && pm.ttsPool != nil {
		if err := pm.ttsPool.Reset(set.TTS); err != nil {
			utils.WithError(ctx, err).Warn("重置TTS资源状态失败")
		}
		if err := pm.ttsPool.Put(ctx, set.TTS); err != nil {
			errs = append(errs, fmt.Errorf("归还TTS提供者失败: %v", err))
			utils.WithError(ctx, err).Error("归还TTS提供者失败")
		} else {
			utils.Debug(ctx, "TTS提供者已成功归还到池中")
		}
	}

	// 归还VLLLM提供者
	if set.VLLLM != nil && pm.vlllmPool != nil {
		if err := pm.vlllmPool.Reset(set.VLLLM); err != nil {
			utils.WithError(ctx, err).Warn("重置VLLLM资源状态失败")
		}
		if err := pm.vlllmPool.Put(ctx, set.VLLLM); err != nil {
			errs = append(errs, fmt.Errorf("归还VLLLM提供者失败: %v", err))
			utils.WithError(ctx, err).Error("归还VLLLM提供者失败")
		} else {
			utils.Debug(ctx, "VLLLM提供者已成功归还到池中")
		}
	}

	// 归还MCP提供者
	if set.MCP != nil && pm.mcpPool != nil {
		if err := pm.mcpPool.Reset(set.MCP); err != nil {
			utils.WithError(ctx, err).Warn("重置MCP资源状态失败")
		}
		if err := pm.mcpPool.Put(ctx, set.MCP); err != nil {
			errs = append(errs, fmt.Errorf("归还MCP提供者失败: %v", err))
			utils.WithError(ctx, err).Error("归还MCP提供者失败")
		} else {
			utils.Debug(ctx, "MCP提供者已成功归还到池中")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("归还过程中发生多个错误: %v", errs)
	}

	utils.Debug(ctx, "所有提供者已成功归还到池中")
	return nil
}

// GetStats 获取所有池的统计信息
func (pm *PoolManager) GetStats() map[string]map[string]int {
	stats := make(map[string]map[string]int)

	if pm.asrPool != nil {
		available, total := pm.asrPool.GetStats()
		stats["asr"] = map[string]int{"available": available, "total": total}
	}

	if pm.llmPool != nil {
		available, total := pm.llmPool.GetStats()
		stats["llm"] = map[string]int{"available": available, "total": total}
	}

	if pm.ttsPool != nil {
		available, total := pm.ttsPool.GetStats()
		stats["tts"] = map[string]int{"available": available, "total": total}
	}

	if pm.vlllmPool != nil {
		available, total := pm.vlllmPool.GetStats()
		stats["vlllm"] = map[string]int{"available": available, "total": total}
	}

	if pm.mcpPool != nil {
		available, total := pm.mcpPool.GetStats()
		stats["mcp"] = map[string]int{"available": available, "total": total}
	}

	return stats
}

// performConnectivityCheck 执行连通性检查
func (pm *PoolManager) performConnectivityCheck(ctx context.Context, config *configs.Config) error {

	// 从配置创建连通性检查配置
	connConfig, err := ConfigFromYAML(&config.ConnectivityCheck)
	if err != nil {
		utils.Warnf(ctx, "解析连通性检查配置失败，使用默认配置: %v", err)
		connConfig = DefaultConnectivityConfig()
	}

	// 创建健康检查器
	healthChecker := NewHealthChecker(config, connConfig)

	// 执行功能性连通性检查
	checkCtx, cancel := context.WithTimeout(ctx, connConfig.Timeout*3) // 给功能性检查更多时间
	defer cancel()

	err = healthChecker.CheckAllProviders(checkCtx, FunctionalCheck)

	// 打印检查报告
	healthChecker.PrintReport(checkCtx)

	return err
}

// GetDetailedStats 获取所有池的详细统计信息
func (pm *PoolManager) GetDetailedStats() map[string]map[string]int {
	stats := make(map[string]map[string]int)

	if pm.asrPool != nil {
		stats["asr"] = pm.asrPool.GetDetailedStats()
	}

	if pm.llmPool != nil {
		stats["llm"] = pm.llmPool.GetDetailedStats()
	}

	if pm.ttsPool != nil {
		stats["tts"] = pm.ttsPool.GetDetailedStats()
	}

	if pm.vlllmPool != nil {
		stats["vlllm"] = pm.vlllmPool.GetDetailedStats()
	}

	if pm.mcpPool != nil {
		stats["mcp"] = pm.mcpPool.GetDetailedStats()
	}

	return stats
}
