package tts

import (
	"combot-server-go/src/log"
	"combot-server-go/src/utils"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"combot-server-go/src/core/providers"
)

// Config TTS配置结构
type Config struct {
	Type            string   `yaml:"type"`
	OutputDir       string   `yaml:"output_dir"`
	Voice           string   `yaml:"voice,omitempty"`
	Format          string   `yaml:"format,omitempty"`
	SampleRate      int      `yaml:"sample_rate,omitempty"`
	AppID           string   `yaml:"appid"`
	Token           string   `yaml:"token"`
	Cluster         string   `yaml:"cluster"`
	SupportedVoices []string `yaml:"supported_voices"` // 支持的语音列表
}

// Provider TTS提供者接口
type Provider interface {
	providers.TTSProvider
}

// BaseProvider TTS基础实现
type BaseProvider struct {
	config     *Config
	deleteFile bool
}

// Config 获取配置
func (p *BaseProvider) Config() *Config {
	return p.config
}

// DeleteFile 获取是否删除文件标志
func (p *BaseProvider) DeleteFile() bool {
	return p.deleteFile
}

// NewBaseProvider 创建TTS基础提供者
func NewBaseProvider(config *Config, deleteFile bool) *BaseProvider {
	return &BaseProvider{
		config:     config,
		deleteFile: deleteFile,
	}
}

// Initialize 初始化提供者
func (p *BaseProvider) Initialize(ctx context.Context) error {
	if err := os.MkdirAll(p.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %v", err)
	}
	return nil
}

// SetVoice 设置声音
func (p *BaseProvider) SetVoice(ctx context.Context, voice string) error {
	// 设置声音配置
	if voice == "" {
		return fmt.Errorf("声音不能为空")
	}
	cnNames := map[string]string{}
	enNames := []string{}
	for _, v := range p.config.SupportedVoices {
		// "zh-CN-XiaoxiaoNeural|晓晓|女|商务知性风格，音色成熟清晰，适合新闻播报、专业内容朗读"
		parts := strings.Split(v, "|")
		if len(parts) >= 2 {
			cnNames[parts[1]] = parts[0] // 中文名
			enNames = append(enNames, parts[0])
		}
	}

	// 如果是中文名，则转换为英文名
	if enVoice, ok := cnNames[voice]; ok {
		voice = enVoice
	}

	// 检查声音是否在支持的列表中
	if !utils.IsInArray(voice, enNames) {
		return fmt.Errorf("不支持的声音: %s, 可用声音: %v", voice, enNames)
	}

	p.Config().Voice = voice
	log.Infof(ctx, "已设置声音: %s", voice)
	return nil
}

// Cleanup 清理资源
func (p *BaseProvider) Cleanup(ctx context.Context) error {
	if p.deleteFile {
		// 清理输出目录中的临时文件
		pattern := filepath.Join(p.config.OutputDir, "*.{wav,mp3,opus}")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("查找临时文件失败: %v", err)
		}
		for _, file := range matches {
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("删除临时文件失败: %v", err)
			}
		}
	}
	return nil
}

// Factory TTS工厂函数类型
type Factory func(config *Config, deleteFile bool) (Provider, error)

var (
	factories = make(map[string]Factory)
)

// Register 注册TTS提供者工厂
func Register(name string, factory Factory) {
	factories[name] = factory
}

// Create 创建TTS提供者实例
func Create(name string, config *Config, deleteFile bool) (Provider, error) {
	factory, ok := factories[name]
	if !ok {
		return nil, fmt.Errorf("未知的TTS提供者: %s", name)
	}

	provider, err := factory(config, deleteFile)
	if err != nil {
		return nil, fmt.Errorf("创建TTS提供者失败: %v", err)
	}

	if err := provider.Initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("初始化TTS提供者失败: %v", err)
	}

	return provider, nil
}
