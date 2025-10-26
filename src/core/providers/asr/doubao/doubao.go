package doubao

// 完全参考 sauc_go 项目的实现

import (
	"bytes"
	"combot-server-go/src/log"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"combot-server-go/src/core/providers/asr"

	"github.com/gorilla/websocket"
	"github.com/maxhawkins/go-webrtcvad"
	"github.com/sirupsen/logrus"
)

// ==================== 协议常量 - 完全按照 sauc_go/common/common.go ====================
const (
	// Serialization
	JSON_SERIALIZATION = SerializationType(0b0001)

	// Compression
	GZIP_COMPRESSION = CompressionType(0b0001)

	// 超时设置
	idleTimeout         = 2 * time.Second // 静音超时时间
	vadSilenceThreshold = 1               // 静音帧数阈值（连续检测到N帧静音后触发结束）
)

// ==================== Provider 结构 ====================

// Ensure Provider implements asr.Provider interface
var _ asr.Provider = (*Provider)(nil)

// Provider 豆包ASR提供者实现
type Provider struct {
	*asr.BaseProvider
	appID       string
	accessToken string
	outputDir   string
	wsURL       string
	connectID   string

	workflow string
	format   string
	codec    string

	// 流式识别相关字段
	conn        *websocket.Conn
	isStreaming bool
	reqID       string
	result      string
	err         error
	connMutex   sync.Mutex

	sendDataCnt int
	seq         int

	segmentDuration int

	// VAD 相关
	vad              *webrtcvad.VAD
	silentFrameCount int  // 连续静音帧计数
	vadEnabled       bool // 是否启用VAD
}

// NewProvider 创建豆包ASR提供者实例
func NewProvider(config *asr.Config, deleteFile bool) (*Provider, error) {
	base := asr.NewBaseProvider(config, deleteFile)

	appID, ok := config.Data["appid"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少appid配置")
	}

	accessToken, ok := config.Data["access_token"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少access_token配置")
	}

	wsurl, ok := config.Data["wsurl"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少wsurl配置")
	}

	outputDir, _ := config.Data["output_dir"].(string)
	if outputDir == "" {
		outputDir = "tmp/"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %v", err)
	}

	connectID := fmt.Sprintf("%d", time.Now().UnixNano())

	// 初始化 WebRTC VAD
	vad, err := webrtcvad.New()
	if err != nil {
		return nil, fmt.Errorf("创建VAD失败: %v", err)
	}

	// 设置 VAD 模式：0(质量模式) 1(低比特率) 2(激进) 3(非常激进)
	// 使用模式1，平衡质量和灵敏度
	if err := vad.SetMode(1); err != nil {
		return nil, fmt.Errorf("设置VAD模式失败: %v", err)
	}

	provider := &Provider{
		BaseProvider:     base,
		appID:            appID,
		accessToken:      accessToken,
		outputDir:        outputDir,
		wsURL:            wsurl,
		connectID:        connectID,
		workflow:         "audio_in,resample,partition,vad,fe,decode",
		format:           "wav",
		codec:            "raw",
		segmentDuration:  200,
		seq:              1,
		vad:              vad,
		vadEnabled:       true,
		silentFrameCount: 0,
	}

	provider.InitAudioProcessing()
	return provider, nil
}

// Transcribe 实现asr.Provider接口的转录方法
func (p *Provider) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	if p.isStreaming {
		return "", fmt.Errorf("正在进行流式识别, 请先调用Reset")
	}

	tempFile := filepath.Join(p.outputDir, fmt.Sprintf("temp_%d.wav", time.Now().UnixNano()))
	if err := os.WriteFile(tempFile, audioData, 0644); err != nil {
		return "", fmt.Errorf("保存临时文件失败: %v", err)
	}
	defer func() {
		if p.DeleteFile() {
			os.Remove(tempFile)
		}
	}()

	if err := p.Initialize(ctx); err != nil {
		return "", err
	}
	defer p.Cleanup(ctx)

	if err := p.AddAudioWithContext(ctx, audioData); err != nil {
		return "", err
	}
	return p.result, nil
}

// GetAudioBuffer 获取基类的audioBuffer
func (p *Provider) GetAudioBuffer() *bytes.Buffer {
	return p.BaseProvider.GetAudioBuffer()
}

// AddAudio 添加音频数据到缓冲区
func (p *Provider) AddAudio(ctx context.Context, data []byte) error {
	return p.AddAudioWithContext(ctx, data)
}

// AddAudioWithContext 带上下文的音频数据添加
func (p *Provider) AddAudioWithContext(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// 使用 WebRTC VAD 检测语音活动
	if p.vadEnabled && p.vad != nil {
		isSpeech, err := p.detectSpeechWithVAD(ctx, data)
		if err != nil {
			log.Warnf(ctx, "VAD检测失败，降级使用能量检测: %v", err)
			// 如果 VAD 失败，降级使用简单的能量检测
			p.vadEnabled = false
		} else {
			if isSpeech {
				// 检测到语音，重置静音帧计数和时间
				p.silentFrameCount = 0
				p.ResetStartListenTime()
				log.Infof(ctx, "VAD: 检测到语音活动")
			} else {
				// 检测到静音，增加静音帧计数
				p.silentFrameCount++
				log.Infof(ctx, "VAD: 检测到静音，连续静音帧数: %d/%d", p.silentFrameCount, vadSilenceThreshold)

				// 如果连续静音帧数超过阈值，发送结束信号
				if p.silentFrameCount >= vadSilenceThreshold && p.isStreaming {
					log.Infof(ctx, "VAD: 连续静音帧数达到阈值 (%d >= %d)，发送结束信号",
						p.silentFrameCount, vadSilenceThreshold)
					if err := p.EndStreaming(ctx); err != nil {
						log.Errorf(ctx, "发送结束信号失败: %v", err)
					}
					return nil
				}
			}
		}
	}

	// 如果连接未建立，先建立连接
	if !p.isStreaming || p.conn == nil {
		err := p.createConnection(ctx)
		if err != nil {
			log.Errorf(ctx, "创建ASR连接失败: %v", err)
			return err
		}
		// 连接建立后重置静音计数
		p.silentFrameCount = 0
	}

	// 使用 sauc_go 风格的音频发送
	audioMessage := NewAudioOnlyRequest(p.seq, data)
	if err := p.conn.WriteMessage(websocket.BinaryMessage, audioMessage); err != nil {
		log.Errorf(ctx, "发送音频数据失败: %v", err)
		// 连接出错，重置状态
		p.setErrorAndStop(err)
		return fmt.Errorf("发送音频数据失败: %v", err)
	}

	p.seq++
	p.sendDataCnt++
	if p.sendDataCnt%20 == 0 {
		log.Infof(ctx, "seq=%d, length=%v, 发送音频数据成功", p.seq-1, len(data))
	}

	return nil
}

// detectSpeechWithVAD 使用 WebRTC VAD 检测语音活动
// 返回 true 表示有语音，false 表示静音
func (p *Provider) detectSpeechWithVAD(ctx context.Context, data []byte) (bool, error) {
	if p.vad == nil {
		return false, fmt.Errorf("VAD未初始化")
	}

	// WebRTC VAD 要求：
	// 1. 采样率: 8000, 16000, 32000, 48000 Hz
	// 2. 帧长度: 10ms, 20ms, 30ms
	//
	// 我们的音频是 16kHz, 16-bit PCM
	// 960 samples = 60ms (不支持)
	// 我们需要使用 160 samples = 10ms 或 320 samples = 20ms

	sampleRate := 16000

	// 计算当前数据的样本数
	sampleCount := len(data) / 2 // 16位音频，每个样本2字节

	// 检查帧长度是否有效
	if !p.vad.ValidRateAndFrameLength(sampleRate, sampleCount) {
		// 如果帧长度不合法，使用前 320 个样本（20ms）或 160 个样本（10ms）
		validSampleCount := 320 // 20ms at 16kHz
		if sampleCount < validSampleCount {
			validSampleCount = 160 // 10ms at 16kHz
			if sampleCount < validSampleCount {
				return false, fmt.Errorf("音频帧太短: %d samples", sampleCount)
			}
		}
		// 只使用前 validSampleCount 个样本
		data = data[:validSampleCount*2]
		sampleCount = validSampleCount
	}

	// 调用 VAD 检测
	isActive, err := p.vad.Process(sampleRate, data)
	if err != nil {
		return false, fmt.Errorf("VAD处理失败: %v", err)
	}

	return isActive, nil
}

func (p *Provider) NewAuthHeader(ctx context.Context) http.Header {
	return NewAuthHeader(p.accessToken, p.appID)
}

// parseResponse 解析响应（使用response.go中的ParseResponse）
func (p *Provider) parseResponse(data []byte) (*AsrResponse, error) {
	result := ParseResponse(data)
	return result, nil
}

// StartStreaming 开始流式识别
func (p *Provider) StartStreaming(ctx context.Context) error {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if p.isStreaming {
		return nil
	}

	p.InitAudioProcessing()
	p.result = ""
	p.err = nil
	p.seq = 1

	if p.conn != nil {
		p.closeConnection()
	}

	log.Infof(ctx, "开始流式识别")

	return nil
}

func (p *Provider) ReadMessage(ctx context.Context) {
	log.Info(ctx, "doubao流式识别协程已启动")
	for {
		if p.conn == nil {
			log.Info(ctx, "连接已关闭，退出读取协程")
			return
		}

		_, data, err := p.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Info(ctx, "WebSocket连接正常关闭")
			} else {
				log.Errorf(ctx, "读取消息失败: %v", err)
			}
			p.setErrorAndStop(err)
			return
		}

		log.Infof(ctx, "收到ASR响应消息，长度=%d字节", len(data))

		result, err := p.parseResponse(data)
		if err != nil {
			log.Errorf(ctx, "解析响应失败: %v", err)
			p.setErrorAndStop(err)
			return
		}

		log.Infof(ctx, "解析响应: Code=%d, IsLastPackage=%v, Seq=%d",
			result.Code, result.IsLastPackage, result.PayloadSequence)

		if result.Code != 0 {
			errMsg := "未知错误"
			if result.PayloadMsg != nil {
				errMsg = result.PayloadMsg.Error
			}
			log.Errorf(ctx, "收到错误响应: Code=%d, Message=%s", result.Code, errMsg)
			p.setErrorAndStop(fmt.Errorf("ASR错误: Code=%d, Message=%s", result.Code, errMsg))
			return
		}

		if result.PayloadMsg != nil {
			text := result.PayloadMsg.Result.Text

			log.Infof(ctx, "识别文本内容: [%s]", text)

			if text != "" {
				log.Infof(ctx, "流式识别: 识别文本=%s", text)

				p.connMutex.Lock()
				p.result = text
				p.connMutex.Unlock()

				if listener := p.BaseProvider.GetListener(); listener != nil {
					log.Infof(ctx, "调用listener.OnAsrResult, text=%s", text)
					p.BaseProvider.SilenceCount = 0
					if finished := listener.OnAsrResult(ctx, text); finished {
						log.Infof(ctx, "listener.OnAsrResult返回finished=true，结束识别")
						// 主动发送结束信号
						if err := p.EndStreaming(ctx); err != nil {
							log.Errorf(ctx, "发送结束信号失败: %v", err)
						}
						return
					}
				} else {
					log.Warn(ctx, "listener为空，无法回调识别结果")
				}
			} else {
				log.Info(ctx, "识别文本为空，继续等待")
			}
		} else {
			log.Warn(ctx, "响应中PayloadMsg为空")
		}

		// 检查是否是最后一个包
		if result.IsLastPackage {
			log.Info(ctx, "收到最后一个包（IsLastPackage=true），结束识别")
			p.connMutex.Lock()
			p.isStreaming = false
			p.connMutex.Unlock()

			// 如果有最终文本且之前没有触发回调，现在触发
			if result.PayloadMsg != nil && result.PayloadMsg.Result.Text != "" {
				text := result.PayloadMsg.Result.Text
				if listener := p.BaseProvider.GetListener(); listener != nil {
					log.Infof(ctx, "最终结果回调: text=%s", text)
					listener.OnAsrResult(ctx, text)
				}
			}
			return
		}
	}
}

func (p *Provider) setErrorAndStop(err error) {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()
	p.err = err
	p.isStreaming = false
	errMsg := err.Error()
	if strings.Contains(errMsg, "use of closed network connection") {
		logrus.WithFields(logrus.Fields{
			"error":       err,
			"sendDataCnt": p.sendDataCnt,
		}).Debug("setErrorAndStop")
	} else {
		logrus.WithFields(logrus.Fields{
			"error":       err,
			"sendDataCnt": p.sendDataCnt,
		}).Error("setErrorAndStop")
	}

	if p.conn != nil {
		p.closeConnection()
	}
}

func (p *Provider) closeConnection() {
	defer func() {
		if r := recover(); r != nil {
			logrus.WithField("error", r).Error("关闭连接时发生错误")
		}
	}()

	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
}

// EndStreaming 结束流式识别，发送结束信号
func (p *Provider) EndStreaming(ctx context.Context) error {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if !p.isStreaming || p.conn == nil {
		log.Info(ctx, "连接未建立或已结束，无需发送结束信号")
		return nil
	}

	// 发送负序列号的空音频包作为结束信号
	negSeq := -p.seq
	log.Infof(ctx, "发送结束信号: seq=%d (负数表示结束)", negSeq)

	endMessage := NewAudioOnlyRequest(negSeq, []byte{})
	if err := p.conn.WriteMessage(websocket.BinaryMessage, endMessage); err != nil {
		log.Errorf(ctx, "发送结束信号失败: %v", err)
		return err
	}

	log.Info(ctx, "结束信号已发送，等待服务器最终响应")
	return nil
}

// Reset 重置ASR状态
func (p *Provider) Reset(ctx context.Context) error {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// 如果正在流式识别，先发送结束信号
	if p.isStreaming && p.conn != nil {
		log.Info(ctx, "Reset时发送结束信号")
		negSeq := -p.seq
		endMessage := NewAudioOnlyRequest(negSeq, []byte{})
		_ = p.conn.WriteMessage(websocket.BinaryMessage, endMessage)
		// 等待一小段时间让服务器处理
		time.Sleep(100 * time.Millisecond)
	}

	p.isStreaming = false
	p.closeConnection()

	p.reqID = ""
	p.result = ""
	p.err = nil
	p.seq = 1

	// 重置 VAD 相关状态
	p.silentFrameCount = 0

	p.InitAudioProcessing()

	log.Info(ctx, "ASR状态已重置")

	return nil
}

// Initialize 实现Provider接口的Initialize方法
func (p *Provider) Initialize(ctx context.Context) error {
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return fmt.Errorf("初始化输出目录失败: %v", err)
	}
	return nil
}

// Cleanup 实现Provider接口的Cleanup方法
func (p *Provider) Cleanup(ctx context.Context) error {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	p.closeConnection()

	logrus.Info("ASR资源已清理")

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	// 注册豆包ASR提供者
	asr.Register("doubao", func(config *asr.Config, deleteFile bool) (asr.Provider, error) {
		return NewProvider(config, deleteFile)
	})
}
