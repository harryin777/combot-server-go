package doubao

// 本实现参考了 sauc_go 项目的 doubao ASR 流式识别代码
// 主要改进：
// 1. 使用更清晰的协议常量定义（protocolVersion, posSequence, negWithSequence等）
// 2. 添加序列号（seq）支持，用于追踪音频数据包
// 3. 改进协议头生成方式，使用 bytes.Buffer 构造消息
// 4. 改进响应解析，支持 messageTypeSpecificFlags 的位标志解析
// 5. 音频数据发送时包含序列号，最后一帧使用负序列号标志

import (
	"bytes"
	"combot-server-go/src/log"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
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
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
)

// Protocol Version
const (
	protocolVersion = 0x1
)

// Message Type
const (
	clientFullRequest      = 0x1
	clientAudioOnlyRequest = 0x2
	serverFullResponse     = 0x9
	serverAck              = 0xB
	serverErrorResponse    = 0xF
)

// Message Type Specific Flags
const (
	noSequence      = 0x0
	posSequence     = 0x1
	negSequence     = 0x2
	negWithSequence = 0x3
)

// Serialization methods
const (
	noSerialization   = 0x0
	jsonFormat        = 0x1
	thriftFormat      = 0x3
	gzipCompression   = 0x1
	customCompression = 0xF

	// 超时设置
	idleTimeout = 30 * time.Second // 没有新数据就结束识别
)

// Ensure Provider implements asr.Provider interface
var _ asr.Provider = (*Provider)(nil)

// Provider 豆包ASR提供者实现 - v2 API
type Provider struct {
	*asr.BaseProvider
	appID       string
	accessToken string
	//cluster       string // ASR集群名称
	outputDir     string
	wsURL         string
	chunkDuration int
	connectID     string

	// Demo 中 AsrClient 的参数
	workflow string // 音频处理流程
	format   string // 音频格式
	codec    string // 音频编码

	// 流式识别相关字段
	conn        *websocket.Conn
	isStreaming bool
	reqID       string
	result      string
	err         error
	connMutex   sync.Mutex // 添加互斥锁保护连接状态

	sendDataCnt      int       // 计数器,用于跟踪发送的音频数据包数量
	seq              int       // 音频序列号，参考 sauc_go
	consecutiveFails int       // 连续失败次数
	lastFailTime     time.Time // 上次失败时间
	inCooldown       bool      // 是否在冷却期(避免重复日志)
}

// NewProvider 创建豆包ASR提供者实例
func NewProvider(config *asr.Config, deleteFile bool) (*Provider, error) {
	base := asr.NewBaseProvider(config, deleteFile)

	// 从config.Data中获取配置
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

	// 确保输出目录存在
	outputDir, _ := config.Data["output_dir"].(string)
	if outputDir == "" {
		outputDir = "tmp/"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %v", err)
	}

	// 创建连接ID
	connectID := fmt.Sprintf("%d", time.Now().UnixNano())

	provider := &Provider{
		BaseProvider:  base,
		appID:         appID,
		accessToken:   accessToken,
		outputDir:     outputDir,
		wsURL:         wsurl,
		chunkDuration: 200, // 固定使用200ms分片
		connectID:     connectID,
		workflow:      "audio_in,resample,partition,vad,fe,decode",
		format:        "wav", // default wav audio
		codec:         "raw", // default raw codec
	}

	// 初始化音频处理
	provider.InitAudioProcessing()

	return provider, nil
}

// Transcribe 实现asr.Provider接口的转录方法
func (p *Provider) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	if p.isStreaming {
		return "", fmt.Errorf("正在进行流式识别, 请先调用Reset")
	}

	// 创建临时文件
	tempFile := filepath.Join(p.outputDir, fmt.Sprintf("temp_%d.wav", time.Now().UnixNano()))
	if err := os.WriteFile(tempFile, audioData, 0644); err != nil {
		return "", fmt.Errorf("保存临时文件失败: %v", err)
	}
	defer func() {
		if p.DeleteFile() {
			os.Remove(tempFile)
		}
	}()

	// 初始化连接
	if err := p.Initialize(ctx); err != nil {
		return "", err
	}
	defer p.Cleanup(ctx)

	// 添加音频数据
	if err := p.AddAudioWithContext(ctx, audioData); err != nil {
		return "", err
	}
	// 等待结果,无法立即返回正确的结果，通过回调函数返回
	return p.result, nil
}

// generateHeader 生成协议头
func (p *Provider) generateHeader(messageType uint8, flags uint8, serializationMethod uint8) []byte {
	header := make([]byte, 4)
	header[0] = (protocolVersion << 4) | 1                   // 协议版本(4位) + 头大小(4位)
	header[1] = (messageType << 4) | flags                   // 消息类型(4位) + 消息标志(4位)
	header[2] = (serializationMethod << 4) | gzipCompression // 序列化方法(4位) + 压缩方法(4位)
	header[3] = 0                                            // 保留字段
	return header
}

// constructRequest 构造请求数据
func (p *Provider) constructRequest(ctx context.Context) map[string]interface{} {
	// 参考 sauc_go 的 AsrRequestPayload 结构
	req := make(map[string]interface{})

	// User 部分
	req["user"] = map[string]interface{}{
		"uid": "demo_uid",
	}

	// Audio 部分
	req["audio"] = map[string]interface{}{
		"format":  "wav",
		"codec":   "raw",
		"rate":    16000,
		"bits":    16,
		"channel": 1,
	}

	// Request 部分
	req["request"] = map[string]interface{}{
		"model_name":       "bigmodel",
		"enable_itn":       true,
		"enable_punc":      true,
		"enable_ddc":       true,
		"show_utterances":  true,
		"enable_nonstream": false,
	}

	return req
}

// GetAudioBuffer 获取基类的audioBuffer
func (p *Provider) GetAudioBuffer() *bytes.Buffer {
	return p.BaseProvider.GetAudioBuffer()
}

// parseResponse 解析响应数据 - 参考 sauc_go 的 ParseResponse 实现
func (p *Provider) parseResponse(data []byte) (map[string]interface{}, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("响应数据太短")
	}

	// 解析头部
	headerSize := data[0] & 0x0f
	messageType := data[1] >> 4
	messageTypeSpecificFlags := data[1] & 0x0f
	serializationMethod := data[2] >> 4
	compressionMethod := data[2] & 0x0f

	// 跳过头部获取payload
	payload := data[headerSize*4:]
	result := make(map[string]interface{})

	// 解析 messageTypeSpecificFlags - 参考 sauc_go
	if messageTypeSpecificFlags&0x01 != 0 {
		// 有序列号
		if len(payload) < 4 {
			return nil, fmt.Errorf("payload太短，无法读取序列号")
		}
		payloadSequence := int32(binary.BigEndian.Uint32(payload[:4]))
		result["payload_sequence"] = payloadSequence
		payload = payload[4:]
	}

	if messageTypeSpecificFlags&0x02 != 0 {
		// 最后一个包
		result["is_last_package"] = true
	}

	if messageTypeSpecificFlags&0x04 != 0 {
		// 有事件
		if len(payload) < 4 {
			return nil, fmt.Errorf("payload太短，无法读取事件")
		}
		event := int(binary.BigEndian.Uint32(payload[:4]))
		result["event"] = event
		payload = payload[4:]
	}

	var payloadMsg []byte
	var payloadSize int32

	// 解析 messageType
	switch messageType {
	case serverFullResponse:
		if len(payload) < 4 {
			return nil, fmt.Errorf("serverFullResponse payload太短")
		}
		payloadSize = int32(binary.BigEndian.Uint32(payload[:4]))
		payload = payload[4:]

	case serverErrorResponse:
		if len(payload) < 8 {
			return nil, fmt.Errorf("serverErrorResponse payload太短")
		}
		code := uint32(binary.BigEndian.Uint32(payload[:4]))
		result["code"] = code
		payloadSize = int32(binary.BigEndian.Uint32(payload[4:8]))
		payload = payload[8:]

	case serverAck:
		// serverAck 可能没有 payload，或者有 payload size
		if len(payload) >= 4 {
			payloadSize = int32(binary.BigEndian.Uint32(payload[:4]))
			payload = payload[4:]
		} else {
			payloadSize = 0
		}
	}

	// 提取 payload 数据
	if len(payload) == 0 {
		return result, nil
	}

	if int(payloadSize) > len(payload) {
		return nil, fmt.Errorf("payload大小不匹配: 声明=%d, 实际=%d", payloadSize, len(payload))
	}

	payloadMsg = payload[:payloadSize]

	// 解压缩 - 参考 sauc_go
	if compressionMethod == gzipCompression {
		reader, err := gzip.NewReader(bytes.NewReader(payloadMsg))
		if err != nil {
			return nil, fmt.Errorf("解压响应数据失败: %v", err)
		}
		defer reader.Close()

		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(reader); err != nil {
			return nil, fmt.Errorf("读取解压数据失败: %v", err)
		}
		payloadMsg = buf.Bytes()
	}

	// 反序列化
	if serializationMethod == jsonFormat {
		var jsonData map[string]interface{}
		if err := json.Unmarshal(payloadMsg, &jsonData); err != nil {
			return nil, fmt.Errorf("解析JSON响应失败: %v", err)
		}
		result["payload_msg"] = jsonData
	} else if serializationMethod != noSerialization {
		result["payload_msg"] = string(payloadMsg)
	}

	result["payload_size"] = payloadSize
	return result, nil
}

// AddAudio 添加音频数据到缓冲区
func (p *Provider) AddAudio(ctx context.Context, data []byte) error {
	return p.AddAudioWithContext(ctx, data)
}

// AddAudioWithContext 带上下文的音频数据添加
func (p *Provider) AddAudioWithContext(ctx context.Context, data []byte) error {
	const maxConsecutiveFails = 3             // 最大连续失败次数
	const cooldownDuration = 30 * time.Second // 冷却时间30秒

	p.connMutex.Lock()
	isStreaming := p.isStreaming
	consecutiveFails := p.consecutiveFails
	lastFailTime := p.lastFailTime
	inCooldown := p.inCooldown

	// 检查是否需要进入或退出冷却期
	if consecutiveFails >= maxConsecutiveFails {
		elapsed := time.Since(lastFailTime)
		if elapsed < cooldownDuration {
			// 仍在冷却期,静默丢弃音频包
			if !inCooldown {
				// 第一次进入冷却期,记录日志
				p.inCooldown = true
				p.connMutex.Unlock()
				log.Warnf(ctx, "ASR连接失败%d次,进入%v冷却期,期间将丢弃音频数据",
					maxConsecutiveFails, cooldownDuration)
				return nil
			}
			p.connMutex.Unlock()
			return nil // 静默丢弃,不记录日志
		}
		// 冷却期结束,重置状态
		if inCooldown {
			log.Info(ctx, "ASR冷却期结束,恢复音频处理")
			p.consecutiveFails = 0
			p.inCooldown = false
		}
	}
	p.connMutex.Unlock()

	if !isStreaming {
		err := p.StartStreaming(ctx)
		if err != nil {
			// 记录失败
			p.connMutex.Lock()
			p.consecutiveFails++
			p.lastFailTime = time.Now()
			p.connMutex.Unlock()
			return err
		}
		// 连接成功,重置失败计数
		p.connMutex.Lock()
		p.consecutiveFails = 0
		p.inCooldown = false
		p.connMutex.Unlock()
	}

	// 检查是否有实际数据需要发送
	if len(data) > 0 && p.isStreaming {
		// 过滤掉1字节的静音包 - 避免无意义的发送
		if len(data) <= 2 {
			// 1-2字节的数据包通常是静音，检查是否超时
			silenceTime := p.SilenceTime()
			if silenceTime > idleTimeout {
				log.Infof(ctx, "检测到静音超时 (%v > %v)，主动结束识别", silenceTime, idleTimeout)
				// 通知listener静音超时
				if listener := p.BaseProvider.GetListener(); listener != nil {
					p.BaseProvider.SilenceCount++
					text := "我没有听清你说话"
					listener.OnAsrResult(ctx, text)
				}
				// 重置ASR状态
				p.Reset(ctx)
			}
			return nil
		}

		// 有音频数据输入时重置静音时间
		p.ResetStartListenTime()

		// 直接发送音频数据
		if err := p.sendAudioData(ctx, data, false); err != nil {
			return err
		} else {
			p.sendDataCnt += 1
			if p.sendDataCnt%20 == 0 {
				log.Infof(ctx, "length: %v, 发送音频数据成功", len(data))
			}
		}
	}

	return nil
}

func (p *Provider) StartStreaming(ctx context.Context) error {
	log.Info(ctx, "----开始流式识别----")
	p.ResetStartListenTime()
	// 加锁保护连接初始化
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// 双重检查，避免并发初始化
	if p.isStreaming {
		return nil
	}

	// 初始化流式识别
	p.InitAudioProcessing()
	p.result = ""
	p.err = nil
	p.seq = 1 // 初始化序列号，参考 sauc_go 从1开始

	// 确保旧连接已关闭
	if p.conn != nil {
		p.closeConnection()
	}

	// 建立WebSocket连接 - 参考 sauc_go 构造完整的认证头
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second, // 设置握手超时
	}

	// 生成请求ID
	reqID := uuid.New().String()

	// 构造认证头 - 参考 sauc_go 的 NewAuthHeader
	headers := http.Header{}
	headers.Add("X-Api-Resource-Id", "volc.bigasr.sauc.duration")
	headers.Add("X-Api-Request-Id", reqID)
	headers.Add("X-Api-Access-Key", p.accessToken)
	headers.Add("X-Api-App-Key", p.appID)

	// 重试机制 - 使用带超时的 context
	var conn *websocket.Conn
	var resp *http.Response
	var err error
	maxRetries := 2

	for i := 0; i <= maxRetries; i++ {
		// 每次重试使用独立的带超时的 context，避免无限阻塞
		dialCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		conn, resp, err = dialer.DialContext(dialCtx, p.wsURL, headers)
		cancel() // 立即释放 context 资源

		if err == nil {
			log.Infof(ctx, "WebSocket连接成功")
			break
		}

		// 只在还有重试次数时才等待
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

	// 发送初始请求 - 参考 sauc_go 的 sendFullClientRequest
	request := p.constructRequest(ctx)
	requestBytes, err := jsoniter.Marshal(request)
	if err != nil {
		return fmt.Errorf("构造请求数据失败: %v", err)
	}

	// 压缩请求数据
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	if _, err := gzipWriter.Write(requestBytes); err != nil {
		return fmt.Errorf("压缩请求数据失败: %v", err)
	}
	gzipWriter.Close()

	compressedRequest := buf.Bytes()

	// 构造完整请求 - 参考 sauc_go 的 NewFullClientRequest
	var fullRequest bytes.Buffer
	header := p.generateHeader(clientFullRequest, posSequence, jsonFormat)
	fullRequest.Write(header)

	// 写入序列号
	_ = binary.Write(&fullRequest, binary.BigEndian, int32(1))

	// 写入payload大小
	payloadSize := make([]byte, 4)
	binary.BigEndian.PutUint32(payloadSize, uint32(len(compressedRequest)))
	fullRequest.Write(payloadSize)

	// 写入payload
	fullRequest.Write(compressedRequest)

	// 发送请求
	if err := p.conn.WriteMessage(websocket.BinaryMessage, fullRequest.Bytes()); err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}

	// 发送完初始请求后，seq++ (参考 sauc_go 的 sendFullClientRequest)
	p.seq++

	// 读取响应
	_, response, err := p.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取响应失败: %v", err)
	}

	log.Infof(ctx, "[DEBUG] 流式识别: 收到WebSocket消息, 长度=%d", len(response))

	initialResult, err := p.parseResponse(response)
	if err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查初始响应 - 打印详细信息用于调试
	log.Infof(ctx, "[DEBUG] 初始响应: %+v", initialResult)

	// 检查错误码 - 注意 Code 字段在 header 中，不在 payload_msg 中
	if code, hasCode := initialResult["code"]; hasCode {
		codeValue := code.(uint32)
		if codeValue != 0 {
			// 尝试获取错误信息
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
	// 开启一个协程来处理响应，读取最后的结果，读取完成后关闭协程
	go func() {
		p.ReadMessage(ctx)
	}()

	return nil
}

func (p *Provider) ReadMessage(ctx context.Context) {
	log.Info(ctx, "doubao流式识别协程已启动")
	defer func() {
		if r := recover(); r != nil {
			log.Errorf(ctx, "流式识别协程发生错误: %v", r)
		}
		p.connMutex.Lock()
		p.isStreaming = false // 标记流式识别结束
		if p.conn != nil {
			p.closeConnection()
		}
		p.connMutex.Unlock()
		log.Info(ctx, "----流式识别协程已结束----")
	}()

	for {
		// 检查连接状态，避免在连接关闭后继续读取
		p.connMutex.Lock()
		if !p.isStreaming || p.conn == nil {
			p.connMutex.Unlock()
			log.Info(ctx, "流式识别已结束或连接已关闭，退出读取循环")
			return
		}
		conn := p.conn
		p.connMutex.Unlock()

		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		_, response, err := conn.ReadMessage()
		if err != nil {
			p.setErrorAndStop(err)
			return
		}

		result, err := p.parseResponse(response)
		if err != nil {
			p.setErrorAndStop(fmt.Errorf("解析响应失败: %v", err))
			return
		}

		// 检查是否是最后一个包 - 参考 sauc_go 的 recvMessages
		if isLast, ok := result["is_last_package"].(bool); ok && isLast {
			log.Info(ctx, "收到最后一个包，流式识别结束")
			return
		}

		// 检查错误码 - 参考 sauc_go: if resp.Code != 0
		if code, hasCode := result["code"]; hasCode {
			codeValue := code.(uint32)
			if codeValue != 0 {
				p.setErrorAndStop(fmt.Errorf("ASR服务端错误: Code=%d", codeValue))
				return
			}
		}

		// 处理正常响应
		if payloadMsg, ok := result["payload_msg"].(map[string]interface{}); ok {
			// 提取识别结果
			text := ""

			// 尝试从 result 字段提取文本
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
						p.BaseProvider.SilenceCount = 0 // 重置静音计数
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
			// 静默处理panic，避免程序崩溃
			logrus.WithField("error", r).Error("关闭连接时发生错误")
		}
	}()

	if p.conn != nil {
		// 不发送关闭消息，直接关闭连接
		_ = p.conn.Close()
		p.conn = nil
	}
}

// sendAudioData 直接发送音频数据，参考 sauc_go 的 NewAudioOnlyRequest
func (p *Provider) sendAudioData(ctx context.Context, data []byte, isLast bool) error {
	log.Infof(ctx, "sendAudioData: 发送音频数据, 长度=%d, isLast=%t, 序列号=%d",
		len(data), isLast, p.seq)

	// 使用锁保护连接状态
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	if p.err != nil {
		return p.err
	}
	if !p.isStreaming {
		return fmt.Errorf("流式识别未初始化")
	}
	// 如果没有数据且不是最后一帧，不发送
	if len(data) == 0 && !isLast {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			// 捕获WebSocket写入时的panic，避免程序崩溃
			log.Errorf(ctx, "发送音频数据时发生panic: %v", r)
		}
	}()

	// 检查连接是否存在
	if p.conn == nil {
		return fmt.Errorf("WebSocket连接不存在")
	}

	// 压缩音频数据
	var compressBuffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressBuffer)
	if _, err := gzipWriter.Write(data); err != nil {
		return fmt.Errorf("压缩音频数据失败: %v", err)
	}
	gzipWriter.Close()
	compressedAudio := compressBuffer.Bytes()

	// 构造音频消息 - 参考 sauc_go 的实现
	var audioMessage bytes.Buffer

	// 确定flags：如果是最后一帧使用负序列号标志
	seq := p.seq
	flags := uint8(posSequence)
	if isLast {
		seq = -seq
		flags = negWithSequence
	}

	header := p.generateHeader(clientAudioOnlyRequest, flags, noSerialization)
	audioMessage.Write(header)

	// 写入序列号
	_ = binary.Write(&audioMessage, binary.BigEndian, int32(seq))

	// 写入payload大小
	_ = binary.Write(&audioMessage, binary.BigEndian, int32(len(compressedAudio)))

	// 写入payload
	audioMessage.Write(compressedAudio)

	if err := p.conn.WriteMessage(websocket.BinaryMessage, audioMessage.Bytes()); err != nil {
		return fmt.Errorf("发送音频数据失败: %v", err)
	}

	// 增加序列号（仅在非最后一帧时）
	if !isLast {
		p.seq++
	}

	return nil
}

// Reset 重置ASR状态
func (p *Provider) Reset(ctx context.Context) error {
	// 使用锁保护状态变更
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	p.isStreaming = false
	p.closeConnection()

	p.reqID = ""
	p.result = ""
	p.err = nil
	p.seq = 1 // 重置序列号

	// 重置音频处理
	p.InitAudioProcessing()

	log.Info(ctx, "ASR状态已重置")

	return nil
}

// Initialize 实现Provider接口的Initialize方法
func (p *Provider) Initialize(ctx context.Context) error {
	// 确保输出目录存在
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return fmt.Errorf("初始化输出目录失败: %v", err)
	}
	return nil
}

// Cleanup 实现Provider接口的Cleanup方法
func (p *Provider) Cleanup(ctx context.Context) error {
	// 使用锁保护状态变更
	p.connMutex.Lock()
	defer p.connMutex.Unlock()

	// 确保WebSocket连接关闭
	p.closeConnection()

	logrus.Info("ASR资源已清理")

	return nil
}

func init() {
	// 注册豆包ASR提供者
	asr.Register("doubao", func(config *asr.Config, deleteFile bool) (asr.Provider, error) {
		return NewProvider(config, deleteFile)
	})
}
