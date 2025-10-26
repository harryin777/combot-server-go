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

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// ==================== 协议常量 - 完全按照 sauc_go/common/common.go ====================
const (
	// Serialization
	JSON_SERIALIZATION = SerializationType(0b0001)

	// Compression
	GZIP_COMPRESSION = CompressionType(0b0001)

	// 超时设置
	idleTimeout = 30 * time.Second
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

	sendDataCnt      int
	seq              int
	consecutiveFails int
	lastFailTime     time.Time
	inCooldown       bool

	segmentDuration int
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

	provider := &Provider{
		BaseProvider:    base,
		appID:           appID,
		accessToken:     accessToken,
		outputDir:       outputDir,
		wsURL:           wsurl,
		connectID:       connectID,
		workflow:        "audio_in,resample,partition,vad,fe,decode",
		format:          "wav",
		codec:           "raw",
		segmentDuration: 200,
		seq:             1,
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
	const maxConsecutiveFails = 3
	const cooldownDuration = 30 * time.Second

	//p.connMutex.Lock()
	//isStreaming := p.isStreaming
	//consecutiveFails := p.consecutiveFails
	//lastFailTime := p.lastFailTime
	//inCooldown := p.inCooldown
	//
	//if consecutiveFails >= maxConsecutiveFails {
	//	elapsed := time.Since(lastFailTime)
	//	if elapsed < cooldownDuration {
	//		if !inCooldown {
	//			p.inCooldown = true
	//			p.connMutex.Unlock()
	//			log.Warnf(ctx, "ASR连接失败%d次,进入%v冷却期,期间将丢弃音频数据",
	//				maxConsecutiveFails, cooldownDuration)
	//			return nil
	//		}
	//		p.connMutex.Unlock()
	//		return nil
	//	}
	//	if inCooldown {
	//		log.Info(ctx, "ASR冷却期结束,恢复音频处理")
	//		p.consecutiveFails = 0
	//		p.inCooldown = false
	//	}
	//}
	//p.connMutex.Unlock()

	//if !isStreaming {
	//	err := p.StartStreaming(ctx)
	//	if err != nil {
	//		p.connMutex.Lock()
	//		p.consecutiveFails++
	//		p.lastFailTime = time.Now()
	//		p.connMutex.Unlock()
	//		return err
	//	}
	//	p.connMutex.Lock()
	//	p.consecutiveFails = 0
	//	p.inCooldown = false
	//	p.connMutex.Unlock()
	//}

	if len(data) > 0 && p.isStreaming {
		if len(data) <= 2 {
			silenceTime := p.SilenceTime()
			if silenceTime > idleTimeout {
				log.Infof(ctx, "检测到静音超时 (%v > %v)，主动结束识别", silenceTime, idleTimeout)
				if listener := p.BaseProvider.GetListener(); listener != nil {
					p.BaseProvider.SilenceCount++
					text := "我没有听清你说话"
					listener.OnAsrResult(ctx, text)
				}
				p.Reset(ctx)
			}
			return nil
		}

		p.ResetStartListenTime()

		err := p.createConnection(ctx)
		if err != nil {
			return err
		}

		// 使用 sauc_go 风格的音频发送
		audioMessage := NewAudioOnlyRequest(p.seq, data)
		if err := p.conn.WriteMessage(websocket.BinaryMessage, audioMessage); err != nil {
			return fmt.Errorf("发送音频数据失败: %v", err)
		}

		p.seq++
		p.sendDataCnt++
		if p.sendDataCnt%20 == 0 {
			log.Infof(ctx, "length: %v, 发送音频数据成功", len(data))
		}
	}

	return nil
}

func (p *Provider) NewAuthHeader(ctx context.Context) http.Header {
	header := http.Header{}

	header.Add("X-Api-Resource-Id", "volc.bigasr.sauc.duration")
	header.Add("X-Api-Request-Id", ctx.Value(log.RequestIDKey).(string))
	header.Add("X-Api-Access-Key", p.accessToken)
	header.Add("X-Api-App-Key", p.appID)
	return header
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

	// 建立WebSocket连接
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	var conn *websocket.Conn
	var resp *http.Response
	var err error
	maxRetries := 2

	for i := 0; i <= maxRetries; i++ {
		dialCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		conn, resp, err = dialer.DialContext(dialCtx, p.wsURL, headers)
		cancel()

		if err == nil {
			log.Infof(ctx, "WebSocket连接成功")
			break
		}

		if i < maxRetries {
			backoffTime := time.Duration(500*(i+1)) * time.Millisecond
			log.Errorf(ctx, "WebSocket连接失败，将在 %v 后重试: %v, 重试次数: %d/%d",
				backoffTime, err, i+1, maxRetries+1)
			time.Sleep(backoffTime)
		} else {
			log.Errorf(ctx, "WebSocket连接失败，已达最大重试次数: %v", err)
		}
	}

	if err != nil {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return fmt.Errorf("WebSocket连接失败(状态码:%d): %v", statusCode, err)
	}

	p.conn = conn

	p.seq++

	// 读取响应
	_, response, err := p.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	log.Infof(ctx, "[DEBUG] 流式识别: 收到WebSocket消息, 长度=%d", len(response))

	initialResult, err := ParseResponse(response)
	if err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	log.Infof(ctx, "[DEBUG] 初始响应: %+v", initialResult)

	if code, hasCode := initialResult["code"]; hasCode {
		codeValue := code.(uint32)
		if codeValue != 0 {
			errMsg := "未知错误"
			if payloadMsg, ok := initialResult["payload_msg"].(map[string]interface{}); ok {
				if msg, ok := payloadMsg["message"].(string); ok {
					errMsg = msg
				}
			}
			return fmt.Errorf("ASR初始化错误: Code=%d, Message=%s", codeValue, errMsg)
		}
	}

	log.Infof(ctx, "流式识别初始化成功, seq=%d", p.seq)
	p.isStreaming = true

	go func() {
		p.ReadMessage(ctx)
	}()

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

		result, err := p.parseResponse(data)
		if err != nil {
			log.Errorf(ctx, "解析响应失败: %v", err)
			p.setErrorAndStop(err)
			return
		}

		if isLast, ok := result["is_last_package"].(bool); ok && isLast {
			log.Info(ctx, "收到最后一个包，结束识别")
			p.connMutex.Lock()
			p.isStreaming = false
			p.connMutex.Unlock()
			return
		}

		if code, hasCode := result["code"]; hasCode {
			codeValue := code.(uint32)
			if codeValue != 0 {
				errMsg := "未知错误"
				if payloadMsg, ok := result["payload_msg"].(map[string]interface{}); ok {
					if msg, ok := payloadMsg["message"].(string); ok {
						errMsg = msg
					}
				}
				log.Errorf(ctx, "收到错误响应: Code=%d, Message=%s", codeValue, errMsg)
				p.setErrorAndStop(fmt.Errorf("ASR错误: Code=%d, Message=%s", codeValue, errMsg))
				return
			}
		}

		if payloadMsg, ok := result["payload_msg"].(map[string]interface{}); ok {
			text := ""

			if resultField, hasResult := payloadMsg["result"].(map[string]interface{}); hasResult {
				if textData, hasText := resultField["text"].(string); hasText {
					text = textData
				}
			}

			if text != "" {
				log.Infof(ctx, "流式识别: 识别文本=%s", text)

				p.connMutex.Lock()
				p.result = text
				p.connMutex.Unlock()

				if listener := p.BaseProvider.GetListener(); listener != nil {
					if text == "" && p.SilenceTime() > idleTimeout {
						p.BaseProvider.SilenceCount += 1
						text = "我没有听清你说话"
					} else if text != "" {
						p.BaseProvider.SilenceCount = 0
					}
					if finished := listener.OnAsrResult(ctx, text); finished {
						return
					}
				}
			}
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

// Reset 重置ASR状态
func (p *Provider) Reset(ctx context.Context) error {
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	p.isStreaming = false
	p.closeConnection()

	p.reqID = ""
	p.result = ""
	p.err = nil
	p.seq = 1

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
