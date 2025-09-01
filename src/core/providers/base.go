package providers

import (
	"combot-server-go/src/core/types"
	"context"
)

// Provider 所有提供者的基础接口
type Provider interface {
	Initialize() error
	Cleanup() error
}

type AsrEventListener interface {
	OnAsrResult(result string) bool
}

// ASRProvider 语音识别提供者接口
type ASRProvider interface {
	Provider
	// Transcribe 直接识别音频数据
	Transcribe(ctx context.Context, audioData []byte) (string, error)
	// AddAudio 添加音频数据到缓冲区
	AddAudio(data []byte) error

	SetListener(listener AsrEventListener)
	// Reset 复位ASR状态
	Reset() error

	// GetSilenceCount 获取当前静音计数
	GetSilenceCount() int

	ResetStartListenTime()
}

// TTSProvider 语音合成提供者接口
type TTSProvider interface {
	Provider

	// 合成音频并返回文件路径
	ToTTS(text string) (string, error)

	SetVoice(voice string) error
}

// LLMProvider 大语言模型提供者接口
type LLMProvider interface {
	types.LLMProvider
}

// Message 对话消息
type Message = types.Message
