package configs

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// TokenConfig Token配置
type TokenConfig struct {
	Token string `yaml:"token"`
}

// Config 主配置结构
type Config struct {
	Server struct {
		IP    string `yaml:"ip"`
		Port  int    `yaml:"port"`
		Token string
		Auth  struct {
			Enabled        bool          `yaml:"enabled"`
			AllowedDevices []string      `yaml:"allowed_devices"`
			Tokens         []TokenConfig `yaml:"tokens"`
		} `yaml:"auth"`
		Device struct {
			HmacKey string `yaml:"hmac_key"`
		} `yaml:"device"`
	} `yaml:"server"`

	Log struct {
		LogFormat string `yaml:"log_format"`
		LogLevel  string `yaml:"log_level"`
		LogDir    string `yaml:"log_dir"`
		LogFile   string `yaml:"log_file"`
	} `yaml:"log"`

	Web struct {
		Enabled   bool   `yaml:"enabled"`
		Port      int    `yaml:"port"`
		StaticDir string `yaml:"static_dir"`
		Websocket string `yaml:"websocket"`
		VisionURL string `yaml:"vision"`
	} `yaml:"web"`

	DefaultPrompt    string   `yaml:"prompt"`
	Roles            []string `yaml:"roles"` // 角色列表
	DeleteAudio      bool     `yaml:"delete_audio"`
	QuickReply       bool     `yaml:"quick_reply"`
	QuickReplyWords  []string `yaml:"quick_reply_words"`
	UsePrivateConfig bool     `yaml:"use_private_config"`
	LocalMCPFun      []string `yaml:"local_mcp_fun"` // 本地MCP函数映射

	SelectedModule map[string]string `yaml:"selected_module"`

	VAD   map[string]VADConfig  `yaml:"VAD"`
	ASR   map[string]ASRConfig  `yaml:"ASR"`
	TTS   map[string]TTSConfig  `yaml:"TTS"`
	LLM   map[string]LLMConfig  `yaml:"LLM"`
	VLLLM map[string]VLLMConfig `yaml:"VLLLM"`

	CMDExit []string `yaml:"CMD_exit"`

	// 连通性检查配置
	ConnectivityCheck ConnectivityCheckConfig `yaml:"connectivity_check"`

	// 数据库配置
	Database DatabaseConfig `yaml:"database"`
}

// VADConfig VAD配置结构
type VADConfig struct {
	Type               string                 `yaml:"type"`
	ModelDir           string                 `yaml:"model_dir"`
	Threshold          float64                `yaml:"threshold"`
	MinSilenceDuration int                    `yaml:"min_silence_duration_ms"`
	Extra              map[string]interface{} `yaml:",inline"`
}

// ASRConfig ASR配置结构
type ASRConfig map[string]interface{}

// TTSConfig TTS配置结构
type TTSConfig struct {
	Type            string   `yaml:"type"`
	Voice           string   `yaml:"voice"`
	Format          string   `yaml:"format"`
	OutputDir       string   `yaml:"output_dir"`
	AppID           string   `yaml:"appid"`
	Token           string   `yaml:"token"`
	Cluster         string   `yaml:"cluster"`
	SurportedVoices []string `yaml:"supported_voices"` // 支持的语音列表
}

// LLMConfig LLM配置结构
type LLMConfig struct {
	Type        string                 `yaml:"type"`
	ModelName   string                 `yaml:"model_name"`
	BaseURL     string                 `yaml:"url"`
	APIKey      string                 `yaml:"api_key"`
	Temperature float64                `yaml:"temperature"`
	MaxTokens   int                    `yaml:"max_tokens"`
	TopP        float64                `yaml:"top_p"`
	Extra       map[string]interface{} `yaml:",inline"`
}

// SecurityConfig 图片安全配置结构
type SecurityConfig struct {
	MaxFileSize       int64    `yaml:"max_file_size"`      // 最大文件大小（字节）
	MaxPixels         int64    `yaml:"max_pixels"`         // 最大像素数量
	MaxWidth          int      `yaml:"max_width"`          // 最大宽度
	MaxHeight         int      `yaml:"max_height"`         // 最大高度
	AllowedFormats    []string `yaml:"allowed_formats"`    // 允许的图片格式
	EnableDeepScan    bool     `yaml:"enable_deep_scan"`   // 启用深度安全扫描
	ValidationTimeout string   `yaml:"validation_timeout"` // 验证超时时间
}

// ConnectivityCheckConfig 连通性检查配置结构
type ConnectivityCheckConfig struct {
	Enabled       bool   `yaml:"enabled"`        // 是否启用连通性检查
	Timeout       string `yaml:"timeout"`        // 检查超时时间
	RetryAttempts int    `yaml:"retry_attempts"` // 重试次数
	RetryDelay    string `yaml:"retry_delay"`    // 重试延迟
	TestModes     struct {
		ASRTestAudio  string `yaml:"asr_test_audio"`  // ASR测试音频文件
		LLMTestPrompt string `yaml:"llm_test_prompt"` // LLM测试提示词
		TTSTestText   string `yaml:"tts_test_text"`   // TTS测试文本
	} `yaml:"test_modes"`
}

// VLLMConfig VLLLM配置结构（视觉语言大模型）
type VLLMConfig struct {
	Type        string                 `yaml:"type"`        // API类型，复用LLM的类型
	ModelName   string                 `yaml:"model_name"`  // 模型名称，使用支持视觉的模型
	BaseURL     string                 `yaml:"url"`         // API地址
	APIKey      string                 `yaml:"api_key"`     // API密钥
	Temperature float64                `yaml:"temperature"` // 温度参数
	MaxTokens   int                    `yaml:"max_tokens"`  // 最大令牌数
	TopP        float64                `yaml:"top_p"`       // TopP参数
	Security    SecurityConfig         `yaml:"security"`    // 图片安全配置
	Extra       map[string]interface{} `yaml:",inline"`     // 额外配置
}

// DatabaseConfig 数据库配置结构
type DatabaseConfig struct {
	Type     string `yaml:"type"`     // 数据库类型: mysql, postgres, sqlite
	Host     string `yaml:"host"`     // 数据库主机
	Port     int    `yaml:"port"`     // 数据库端口
	User     string `yaml:"user"`     // 数据库用户名
	Password string `yaml:"password"` // 数据库密码
	Database string `yaml:"database"` // 数据库名称
	Path     string `yaml:"path"`     // SQLite数据库文件路径
	SSLMode  string `yaml:"ssl_mode"` // SSL模式 (PostgreSQL)
	Charset  string `yaml:"charset"`  // 字符集 (MySQL)

	// 连接池配置
	MaxOpenConns    int    `yaml:"max_open_conns"`    // 最大打开连接数
	MaxIdleConns    int    `yaml:"max_idle_conns"`    // 最大空闲连接数
	ConnMaxLifetime string `yaml:"conn_max_lifetime"` // 连接最大生存时间
}

// LoadConfig 从文件加载配置
func LoadConfig() (*Config, string, error) {
	// 获取可执行文件所在目录
	ex, err := os.Executable()
	if err != nil {
		ex, _ = os.Getwd()
	} else {
		ex = filepath.Dir(ex)
	}

	// 尝试多个可能的配置文件路径
	configPaths := []string{
		".config.yaml",                          // 当前目录
		"config.yaml",                           // 当前目录
		filepath.Join(ex, ".config.yaml"),       // 可执行文件目录
		filepath.Join(ex, "config.yaml"),        // 可执行文件目录
		filepath.Join(ex, "..", ".config.yaml"), // 可执行文件上级目录
		filepath.Join(ex, "..", "config.yaml"),  // 可执行文件上级目录
	}

	var path string
	var data []byte
	var readErr error

	for _, p := range configPaths {
		if _, err := os.Stat(p); err == nil {
			data, readErr = os.ReadFile(p)
			if readErr == nil {
				path = p
				break
			}
		}
	}

	if path == "" {
		return nil, "", fmt.Errorf("未找到配置文件，尝试了以下路径: %v", configPaths)
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, path, err
	}

	return config, path, nil
}
