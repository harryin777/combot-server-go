package core

import (
	"combot-server-go/src/log"
	"combot-server-go/src/models"
	"combot-server-go/src/utils"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"combot-server-go/src/configs"
	"combot-server-go/src/core/chat"
	"combot-server-go/src/core/function"
	"combot-server-go/src/core/image"
	"combot-server-go/src/core/mcp"
	"combot-server-go/src/core/pool"
	"combot-server-go/src/core/providers"
	"combot-server-go/src/core/providers/tts"
	"combot-server-go/src/core/providers/vlllm"
	"combot-server-go/src/core/types"
	"combot-server-go/src/service"
	"combot-server-go/src/task"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	jsoniter "github.com/json-iterator/go"
)

// Connection 统一连接接口
type Connection interface {
	// 发送消息
	WriteMessage(messageType int, data []byte) error
	// 读取消息
	ReadMessage() (messageType int, data []byte, err error)
	// 关闭连接
	Close(ctx context.Context) error
	// 获取连接ID
	GetID() string
	// 获取连接类型
	GetType() string
	// 检查连接状态
	IsClosed() bool
	// 获取最后活跃时间
	GetLastActiveTime() time.Time
	// 检查是否过期
	IsStale(timeout time.Duration) bool
}

type configGetter interface {
	Config() *tts.Config
}

// ConnectionHandler 连接处理器结构
type ConnectionHandler struct {
	// 确保实现 AsrEventListener 接口
	_                providers.AsrEventListener
	config           *configs.Config
	conn             Connection
	closeOnce        sync.Once
	taskMgr          *task.TaskManager
	safeCallbackFunc func(func(*ConnectionHandler)) func()
	providers        struct {
		asr   providers.ASRProvider
		llm   providers.LLMProvider
		tts   providers.TTSProvider
		vlllm *vlllm.Provider // VLLLM提供者，可选
	}

	initialVoice string // 初始语音名称

	// 会话相关
	sessionID string
	deviceID  string            // 设备ID
	clientId  string            // 客户端ID
	headers   map[string]string // HTTP头部信息

	// 客户端音频相关
	clientAudioFormat        string
	clientAudioSampleRate    int
	clientAudioChannels      int
	clientAudioFrameDuration int

	// 客户端协议相关
	clientProtocolVersion int  // 客户端协议版本 (1,2,3)
	clientSupportsMCP     bool // 客户端是否支持MCP
	clientSupportsAEC     bool // 客户端是否支持AEC

	serverAudioFormat        string // 服务端音频格式
	serverAudioSampleRate    int
	serverAudioChannels      int
	serverAudioFrameDuration int

	clientListenMode string
	isDeviceVerified bool
	closeAfterChat   bool

	// 语音处理相关
	clientVoiceStop bool  // true客户端语音停止, 不再上传语音数据
	serverVoiceStop int32 // 1表示true服务端语音停止, 不再下发语音数据

	opusDecoder *utils.OpusDecoder // Opus解码器

	// 对话相关
	dialogueManager  *chat.DialogueManager
	ttsLastTextIndex int
	clientAsrText    string // 客户端ASR文本
	quickReplyCache  *utils.QuickReplyCache

	// 并发控制
	stopChan         chan struct{}
	clientAudioQueue chan []byte
	clientTextQueue  chan string

	// TTS任务队列
	ttsQueue chan struct {
		text      string
		round     int // 轮次
		textIndex int
	}

	audioMessagesQueue chan struct {
		filepath  string
		text      string
		round     int // 轮次
		textIndex int
	}

	talkRound      int       // 轮次计数
	roundStartTime time.Time // 轮次开始时间
	// functions
	functionRegister *function.FunctionRegistry
	mcpManager       *mcp.Manager

	mcpResultHandlers map[string]func(context.Context, interface{}) // MCP处理器映射

	// 对话历史服务
	conversationService service.ConversationService
	currentAIRole       string // 当前AI角色
}

// NewConnectionHandler 创建新的连接处理器
func NewConnectionHandler(
	config *configs.Config,
	providerSet *pool.ProviderSet,
	req *http.Request,
	ctx context.Context,
) *ConnectionHandler {
	handler := &ConnectionHandler{
		config:           config,
		clientListenMode: "auto",
		stopChan:         make(chan struct{}),
		clientAudioQueue: make(chan []byte, 100),
		clientTextQueue:  make(chan string, 100),
		ttsQueue: make(chan struct {
			text      string
			round     int // 轮次
			textIndex int
		}, 100),
		audioMessagesQueue: make(chan struct {
			filepath  string
			text      string
			round     int // 轮次
			textIndex int
		}, 100),

		ttsLastTextIndex: -1,

		talkRound: 0,

		serverAudioFormat:        "opus", // 默认使用Opus格式
		serverAudioSampleRate:    24000,
		serverAudioChannels:      1,
		serverAudioFrameDuration: 60,

		// 设置客户端协议默认值
		clientProtocolVersion: 1,      // 默认协议版本1
		clientSupportsMCP:     false,  // 默认不支持MCP
		clientSupportsAEC:     false,  // 默认不支持AEC
		clientAudioFormat:     "opus", // 默认音频格式

		headers: make(map[string]string),
	}

	for key, values := range req.Header {
		if len(values) > 0 {
			handler.headers[key] = values[0] // 取第一个值
		}
		if key == "Device-Id" {
			handler.deviceID = values[0] // 设备ID
		}
		if key == "Client-Id" {
			handler.clientId = values[0] // 客户端ID
		}
		if key == "Session-Id" {
			handler.sessionID = values[0] // 会话ID
		}
		log.Infof(ctx, "HTTP头部信息: %s: %s", key, values[0])
	}

	if handler.sessionID == "" {
		if handler.deviceID == "" {
			handler.sessionID = uuid.New().String() // 如果没有设备ID，则生成新的会话ID
		} else {
			handler.sessionID = "device-" + strings.Replace(handler.deviceID, ":", "_", -1)
		}
	}

	// 正确设置providers
	if providerSet != nil {
		handler.providers.asr = providerSet.ASR
		handler.providers.llm = providerSet.LLM
		handler.providers.tts = providerSet.TTS
		handler.providers.vlllm = providerSet.VLLLM
		handler.mcpManager = providerSet.MCP
	}

	ttsProvider := "default" // 默认TTS提供者名称
	voiceName := "default"
	if getter, ok := handler.providers.tts.(configGetter); ok {
		ttsProvider = getter.Config().Type
		voiceName = getter.Config().Voice
		handler.initialVoice = voiceName // 保存初始语音名称
	}
	log.Infof(ctx, "使用TTS提供者: %s, 语音名称: %s", ttsProvider, voiceName)
	handler.quickReplyCache = utils.NewQuickReplyCache(ttsProvider, voiceName)

	// 初始化对话管理器
	handler.dialogueManager = chat.NewDialogueManager(nil)
	handler.dialogueManager.SetSystemMessage(config.DefaultPrompt)
	handler.functionRegister = function.NewFunctionRegistry()
	handler.initMCPResultHandlers()

	// 初始化对话历史服务
	handler.conversationService = service.NewConversationService()
	handler.currentAIRole = "combot" // 默认角色

	return handler
}

func (h *ConnectionHandler) SetTaskCallback(callback func(func(*ConnectionHandler)) func()) {
	h.safeCallbackFunc = callback
}

func (h *ConnectionHandler) SubmitTask(ctx context.Context, taskType string, params map[string]interface{}) {
	_task, id := task.NewTask(ctx, "", params)
	log.Infof(ctx, "提交任务: %s, ID: %s, 参数: %v", _task.Type, id, params)
	// 创建安全回调用于任务完成时调用
	var taskCallback func(result interface{})
	if h.safeCallbackFunc != nil {
		taskCallback = func(result interface{}) {
			// 移除调试输出，使用logrus替代
			log.Debug(ctx, "任务完成回调")
			safeCallback := h.safeCallbackFunc(func(handler *ConnectionHandler) {
				// 处理任务完成逻辑
				handler.handleTaskComplete(ctx, _task, id, result)
			})
			// 执行安全回调
			if safeCallback != nil {
				safeCallback()
			}
		}
	}
	cb := task.NewCallBack(taskCallback)
	_task.Callback = cb
	h.taskMgr.SubmitTask(h.sessionID, _task)
}

func (h *ConnectionHandler) handleTaskComplete(ctx context.Context, task *task.Task, id string, result interface{}) {
	log.Infof(ctx, "任务 %s 完成，ID: %s, %v", task.Type, id, result)
}

// Handle 处理WebSocket连接
func (h *ConnectionHandler) Handle(ctx context.Context, conn Connection) {
	defer conn.Close(ctx)

	h.conn = conn

	// 设置ASR监听器，确保ASR结果能够回调到ConnectionHandler
	if h.providers.asr != nil {
		h.providers.asr.SetListener(h)
	}

	// 启动消息处理协程
	go h.processClientAudioMessagesGoroutine(ctx) // 添加客户端音频消息处理协程
	go h.processClientTextMessagesGoroutine(ctx)  // 添加客户端文本消息处理协程
	go h.processTTSQueueCoroutine(ctx)            // 添加TTS队列处理协程
	go h.sendAudioMessageCoroutine(ctx)           // 添加音频消息发送协程
	go h.keepAliveGoroutine(ctx)                  // 添加心跳保活协程

	// 检查MCP管理器是否可用（但不要在这里初始化连接）
	if h.mcpManager == nil {
		log.Error(ctx, "没有可用的MCP管理器")
		return
	}

	log.Info(ctx, "MCP管理器可用，等待客户端hello消息后再初始化MCP连接")
	log.Info(ctx, "进入主消息循环，准备接收客户端消息...")

	// 主消息循环
	for {
		select {
		case <-h.stopChan:
			log.Info(ctx, "收到停止信号，退出消息循环")
			return
		default:
			log.Info(ctx, "阻塞等待接收客户端消息...")
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				// 判断错误类型，提供更详细的诊断信息
				if errors.Is(err, ErrConnectionClosed) {
					log.Warnf(ctx, "连接已关闭 [设备ID: %s, 会话ID: %s]", h.deviceID, h.sessionID)
				} else if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Infof(ctx, "客户端正常断开连接 [设备ID: %s, 会话ID: %s]", h.deviceID, h.sessionID)
				} else if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Warnf(ctx, "客户端异常断开连接 [设备ID: %s, 会话ID: %s]: %v", h.deviceID, h.sessionID, err)
				} else {
					log.Errorf(ctx, "读取消息失败 [设备ID: %s, 会话ID: %s]: %v", h.deviceID, h.sessionID, err)
				}
				return
			}

			log.Infof(ctx, "收到消息 [类型=%d, 长度=%d字节]", messageType, len(message))

			if err := h.handleMessage(ctx, messageType, message); err != nil {
				log.Errorf(ctx, "处理消息失败: %v", err)
			}

			log.Info(ctx, "消息处理完成，继续等待下一条消息...")
		}
	}
}

// processClientTextMessagesGoroutine 处理文本消息队列
func (h *ConnectionHandler) processClientTextMessagesGoroutine(ctx context.Context) {
	for {
		select {
		case <-h.stopChan:
			return
		case text := <-h.clientTextQueue:
			if err := h.processClientTextMessage(ctx, text); err != nil {
				log.Error(ctx, fmt.Sprintf("处理文本数据失败: %v", err))
			}
		}
	}
}

// processClientAudioMessagesGoroutine 处理音频消息队列
func (h *ConnectionHandler) processClientAudioMessagesGoroutine(ctx context.Context) {
	for {
		select {
		case <-h.stopChan:
			return
		case audioData := <-h.clientAudioQueue:
			if h.closeAfterChat {
				continue
			}
			if err := h.providers.asr.AddAudio(ctx, audioData); err != nil {
				log.Errorf(ctx, "处理音频数据失败: %v", err)
			}
		}
	}
}

func (h *ConnectionHandler) sendAudioMessageCoroutine(ctx context.Context) {
	for {
		select {
		case <-h.stopChan:
			return
		case task := <-h.audioMessagesQueue:
			h.sendAudioMessage(ctx, task.filepath, task.text, task.textIndex, task.round)
		}
	}
}

// keepAliveGoroutine 定期发送 Ping 消息保持连接活跃
func (h *ConnectionHandler) keepAliveGoroutine(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // 每30秒发送一次 Ping
	defer ticker.Stop()

	for {
		select {
		case <-h.stopChan:
			return
		case <-ticker.C:
			// 发送 Ping 消息
			if h.conn != nil {
				// 使用 WriteMessage 发送 Ping（底层的 websocket 连接会处理）
				// 注意：这里我们通过发送一个空的心跳消息来保持连接
				// 实际的 Ping/Pong 由底层的 websocket_conn 处理
				log.Debug(ctx, "发送心跳保活消息")
			}
		}
	}
}

// OnAsrResult 实现 AsrEventListener 接口
// 返回true则停止语音识别，返回false会继续语音识别
func (h *ConnectionHandler) OnAsrResult(ctx context.Context, result string) bool {
	log.Infof(ctx, "[%s] ASR识别结果: %s", h.clientListenMode, result)

	if h.providers.asr.GetSilenceCount() >= 2 {
		log.Info(ctx, "检测到连续两次静音，结束对话")
		h.closeAfterChat = true // 如果连续两次静音，则结束对话
		result = "长时间未检测到用户说话，请礼貌的结束对话"
	}

	trimmed := strings.TrimSpace(result)
	if trimmed == "" && h.providers.asr.GetSilenceCount() > 0 {
		log.Info(ctx, "收到静音结束事件，通知设备端停止聆听")
		if err := h.sendListenState(ctx, "stop"); err != nil {
			log.Errorf(ctx, "发送静音停止消息失败: %v", err)
		}
		h.clientVoiceStop = true
		return true
	}

	if h.clientListenMode == "auto" {
		if trimmed == "" {
			return false
		}
		log.Infof(ctx, "[%s] ASR识别结果: %s", h.clientListenMode, result)
		h.handleChatMessage(ctx, result)
		return true
	} else if h.clientListenMode == "manual" {
		h.clientAsrText += result
		if trimmed != "" {
			log.Infof(ctx, "[%s] ASR识别结果: %s", h.clientListenMode, h.clientAsrText)
		}
		if h.clientVoiceStop {
			h.handleChatMessage(ctx, h.clientAsrText)
			return true
		}
		return false
	} else if h.clientListenMode == "realtime" {
		if result == "" {
			return false
		}
		h.stopServerSpeak(ctx)
		h.providers.asr.Reset(ctx) // 重置ASR状态，准备下一次识别
		log.Infof(ctx, "[%s] ASR识别结果: %s", h.clientListenMode, result)
		h.handleChatMessage(ctx, result)
		return true
	}
	return false
}

// clientAbortChat 处理中止消息
func (h *ConnectionHandler) clientAbortChat(ctx context.Context) error {
	log.Infof(ctx, "收到客户端中止消息，停止语音识别")
	h.stopServerSpeak(ctx)
	h.sendTTSMessage(ctx, "stop", "", 0)
	h.clearSpeakStatus(ctx)
	return nil
}

func (h *ConnectionHandler) QuitIntent(ctx context.Context, text string) bool {
	//CMD_exit 读取配置中的退出命令
	exitCommands := h.config.CMDExit
	if exitCommands == nil {
		return false
	}
	cleanText := utils.RemoveAllPunctuation(text) // 移除标点符号，确保匹配准确
	// 检查是否包含退出命令
	for _, cmd := range exitCommands {
		log.Debugf(ctx, "检查退出命令: %s,%s", cmd, cleanText)
		//判断相等
		if cleanText == cmd {
			log.Info(ctx, "收到客户端退出意图，准备结束对话")
			h.Close(ctx) // 直接关闭连接
			return true
		}
	}
	return false
}

// quickReplyWakeUpWords 处理快速回复和唤醒词逻辑
// 返回 true 表示应该跳过后续LLM处理（已处理完成），返回 false 表示需要继续LLM处理
func (h *ConnectionHandler) quickReplyWakeUpWords(ctx context.Context, text string) bool {
	// 如果未启用快速回复或不是第一轮对话，继续LLM处理
	if !h.config.QuickReply || h.talkRound != 1 {
		return false
	}

	// 如果包含唤醒词，继续LLM处理（不进行快速回复）
	if utils.IsWakeUpWord(text) {
		log.Infof(ctx, "检测到唤醒词，继续LLM处理: %s", text)
		return false
	}

	// 如果没有唤醒词，进行快速回复并跳过LLM处理
	replyText := utils.RandomSelectFromArray(h.config.QuickReplyWords)
	h.ttsLastTextIndex = 1 // 重置文本索引
	h.SpeakAndPlay(ctx, replyText, 1, h.talkRound)
	log.Infof(ctx, "使用快速回复（无唤醒词）: %s -> %s", text, replyText)

	return true
}

// handleChatMessage 处理聊天消息
func (h *ConnectionHandler) handleChatMessage(ctx context.Context, text string) error {
	if text == "" {
		log.Warn(ctx, "收到空聊天消息，忽略")
		_ = h.clientAbortChat(ctx)
		return fmt.Errorf("聊天消息为空")
	}

	if h.QuitIntent(ctx, text) {
		return fmt.Errorf("用户请求退出对话")
	}

	// 增加对话轮次
	h.talkRound++
	h.roundStartTime = time.Now()
	currentRound := h.talkRound
	log.Infof(ctx, "开始新的对话轮次: %d", currentRound)

	// 判断是否需要验证
	if h.isNeedAuth() {
		if err := h.checkAndBroadcastAuthCode(ctx); err != nil {
			log.Errorf(ctx, "检查认证码失败: %v", err)
			return err
		}
		log.Info(ctx, "设备未认证，等待管理员认证")
		return nil
	}

	// 普通文本消息处理流程
	// 立即发送 stt 消息，这里的消息是用户说的话经过 asr 识别后变成文字
	err := h.sendSTTMessage(text)
	if err != nil {
		log.Errorf(ctx, "发送STT消息失败: %v", err)
		return fmt.Errorf("发送STT消息失败: %v", err)
	}

	// 发送tts start状态，这里是告诉智能体准备切换到广播状态
	if err := h.sendTTSMessage(ctx, "start", "", 0); err != nil {
		log.Errorf(ctx, "发送TTS开始状态失败: %v", err)
		return fmt.Errorf("发送TTS开始状态失败: %v", err)
	}

	// 发送思考状态的情绪
	if err := h.sendEmotionMessage(ctx, "thinking", 1); err != nil {
		log.Errorf(ctx, "发送思考状态情绪消息失败: %v", err)
		return fmt.Errorf("发送情绪消息失败: %v", err)
	}

	log.Infof(ctx, "收到聊天消息: %v", text)

	if h.quickReplyWakeUpWords(ctx, text) {
		return nil
	}

	// 添加用户消息到对话历史
	userMessage := chat.Message{
		Role:    "user",
		Content: text,
	}
	h.dialogueManager.Put(userMessage)

	// 保存用户消息到数据库
	go func() {
		err := h.conversationService.SaveConversation(
			ctx,
			h.sessionID,
			h.deviceID,
			h.clientId,
			userMessage.Content,
			int(models.RoleUser),        // MessageRole - 用户消息
			int(models.ContentTypeText), // MessageType - 文本消息
			h.currentAIRole,
		)
		if err != nil {
			log.Errorf(ctx, "保存用户消息到数据库失败: %v", err)
		}
	}()

	return h.genResponseByLLM(ctx, h.dialogueManager.GetLLMDialogue(), currentRound)
}

// genResponseByLLM 使用LLM生成回复的核心方法
// 支持流式响应、实时播放、工具调用等完整的对话处理流程
func (h *ConnectionHandler) genResponseByLLM(ctx context.Context, messages []providers.Message, round int) error {
	// ========== 异常保护 ==========
	// 防止panic导致整个连接断开，确保系统稳定性
	defer func() {
		if r := recover(); r != nil {
			log.Errorf(ctx, "genResponseByLLM发生panic: %v", r)
			errorMsg := "抱歉，处理您的请求时发生了错误"
			h.ttsLastTextIndex = 1 // 重置文本索引
			h.SpeakAndPlay(ctx, errorMsg, 1, round)
		}
	}()

	// ========== 步骤1: 初始化和性能监控 ==========
	llmStartTime := time.Now() // 记录开始时间，用于计算首次响应延迟
	log.Infof(ctx, "开始生成LLM回复, round:%d ", round)

	// 获取所有可用的工具函数，支持MCP和本地函数调用
	tools := h.functionRegister.GetAllFunctions()

	// 调用LLM提供者的流式接口，获取响应通道
	responses, err := h.providers.llm.ResponseWithFunctions(ctx, h.sessionID, messages, tools)
	if err != nil {
		return fmt.Errorf("LLM生成回复失败: %v", err)
	}

	// ========== 步骤2: 初始化流式处理状态 ==========
	var responseMessage []string      // 累积的响应消息片段
	processedChars, textIndex := 0, 0 // 已处理字符数和文本片段索引

	// 重置服务端语音停止标志，准备新的语音输出
	atomic.StoreInt32(&h.serverVoiceStop, 0)

	// ========== 步骤3: 工具调用检测和状态管理 ==========
	toolCallFlag := false // 标记是否检测到工具调用
	var functionName, functionID, functionArguments, contentArguments string

	// ========== 步骤4: 流式响应处理主循环 ==========
	// 逐块处理LLM返回的数据，实现边生成边播放的用户体验
	for response := range responses {
		content := response.Content    // 本次接收到的文本内容
		toolCall := response.ToolCalls // 本次接收到的工具调用信息

		// ========== 错误处理 ==========
		// 检查LLM返回的错误信息，及时向用户反馈
		if response.Error != "" {
			log.Errorf(ctx, "LLM响应错误: %s", response.Error)
			errorMsg := "抱歉，服务暂时不可用，请稍后再试"
			h.ttsLastTextIndex = 1 // 重置文本索引
			h.SpeakAndPlay(ctx, errorMsg, 1, round)
			return fmt.Errorf("LLM响应错误: %s", response.Error)
		}

		// ========== 内容累积 ==========
		// 累积所有接收到的内容，用于工具调用检测和完整文本构建
		if content != "" {
			contentArguments += content
		}

		// ========== 工具调用检测 ==========
		// 方式1: 通过文本标签检测工具调用开始
		if !toolCallFlag && strings.HasPrefix(contentArguments, "<tool_call>") {
			toolCallFlag = true
		}

		// 方式2: 通过API返回的ToolCalls字段检测
		if len(toolCall) > 0 {
			toolCallFlag = true
			// 累积工具调用的相关信息
			if toolCall[0].ID != "" {
				functionID = toolCall[0].ID
			}
			if toolCall[0].Function.Name != "" {
				functionName = toolCall[0].Function.Name
			}
			if toolCall[0].Function.Arguments != "" {
				functionArguments += toolCall[0].Function.Arguments
			}
		}

		// ========== 文本内容处理 ==========
		if content != "" {
			// 服务异常检测：快速识别服务问题并反馈用户
			if strings.Contains(content, "服务响应异常") {
				log.Errorf(ctx, "检测到LLM服务异常: %s", content)
				errorMsg := "抱歉，服务暂时不可用，请稍后再试"
				h.ttsLastTextIndex = 1 // 重置文本索引
				h.SpeakAndPlay(ctx, errorMsg, 1, round)
				return fmt.Errorf("LLM服务异常")
			}

			// 工具调用时跳过文本处理：避免播放技术性内容
			if toolCallFlag {
				continue
			}

			// ========== 实时文本分段处理 ==========
			// 将新内容加入响应消息列表
			responseMessage = append(responseMessage, content)

			// 构建完整文本并提取未处理部分
			fullText := utils.JoinStrings(responseMessage)
			if len(fullText) <= processedChars {
				// 异常情况：文本长度不增长，记录警告并跳过
				log.Warnf(ctx, "文本处理异常: fullText长度=%d, processedChars=%d", len(fullText), processedChars)
				continue
			}
			currentText := fullText[processedChars:] // 提取未处理的文本部分

			// ========== 智能分段和实时播放 ==========
			// 按标点符号分割文本，实现自然的语音停顿
			if segment, dealCount := utils.SplitAtLastPunctuation(currentText); dealCount > 0 {
				textIndex++ // 分段序号递增
				h.ttsLastTextIndex = textIndex

				// ========== 性能监控和日志记录 ==========
				if textIndex == 1 {
					// 记录首次响应时间：关键性能指标
					llmSpentTime := time.Since(llmStartTime)
					log.Infof(ctx, "LLM首段回复耗时 %v，内容：【%s】(round: %d)", llmSpentTime, segment, round)
				} else {
					// 记录后续分段信息
					log.Infof(ctx, "LLM回复分段[%d]: %s (round: %d)", textIndex, segment, round)
				}

				// ========== 语音合成和播放 ==========
				// 异步处理TTS，不阻塞LLM响应接收
				err := h.SpeakAndPlay(ctx, segment, textIndex, round)
				if err != nil {
					log.Errorf(ctx, "播放LLM回复分段失败: %v", err)
				}

				// 更新已处理字符数，避免重复处理
				processedChars += dealCount
			}
		}
	}

	// ========== 步骤5: 工具调用处理 ==========
	// 当检测到工具调用时，解析参数并执行相应的工具函数
	if toolCallFlag {
		bHasError := false

		// ========== 工具调用参数解析 ==========
		// 如果没有从API直接获取到函数信息，尝试从累积的文本中解析
		if functionID == "" {
			// 从文本内容中提取JSON格式的工具调用信息
			a := utils.ExtractJsonFromString(ctx, contentArguments)
			if a != nil {
				// 安全的类型断言，避免panic
				if name, ok := a["name"].(string); ok {
					functionName = name
				} else {
					log.Error(ctx, "工具调用缺少name字段")
					bHasError = true
				}

				// 序列化参数为JSON字符串
				if !bHasError {
					argumentsJson, err := jsoniter.Marshal(a["arguments"])
					if err != nil {
						log.Errorf(ctx, "函数调用参数序列化失败: %v", err)
						bHasError = true
					} else {
						functionArguments = string(argumentsJson)
						functionID = uuid.New().String() // 生成唯一ID
					}
				}
			} else {
				bHasError = true
				log.Error(ctx, "无法从内容中解析工具调用信息")
			}
		}

		// ========== 工具调用执行 ==========
		if !bHasError {
			// 清空响应消息：工具调用时不保存中间文本内容
			responseMessage = []string{}

			// 解析函数参数
			arguments := make(map[string]interface{})
			if err := jsoniter.Unmarshal([]byte(functionArguments), &arguments); err != nil {
				log.Errorf(ctx, "函数调用参数解析失败: %v", err)
			}

			// 构造函数调用数据，用于后续的对话历史记录
			functionCallData := map[string]interface{}{
				"id":        functionID,
				"name":      functionName,
				"arguments": functionArguments,
			}
			log.Infof(ctx, "执行工具调用: %s, 参数: %v", functionName, arguments)

			// ========== MCP工具调用处理 ==========
			if h.mcpManager.IsMCPTool(functionName) {
				// 执行MCP (Model Context Protocol) 工具
				result, err := h.mcpManager.ExecuteTool(ctx, functionName, arguments)
				if err != nil {
					log.Errorf(ctx, "MCP函数调用失败: %v", err)
					if result == nil {
						result = "MCP工具调用失败"
					}
				}

				// ========== 处理工具调用结果 ==========
				if actionResult, ok := result.(types.ActionResponse); ok {
					// 结果已经是ActionResponse类型，直接处理
					h.handleFunctionResult(ctx, actionResult, functionCallData, textIndex)
				} else {
					// 包装为ActionResponse类型，请求LLM处理结果
					log.Infof(ctx, "MCP函数调用结果: %v", result)
					actionResult := types.ActionResponse{
						Action: types.ActionTypeReqLLM, // 请求LLM基于结果生成回复
						Result: result,                 // 工具执行的原始结果
					}
					h.handleFunctionResult(ctx, actionResult, functionCallData, textIndex)
				}
			} else {
				// ========== 普通函数调用处理 ==========
				// 预留接口：处理非MCP的本地函数调用
				// h.functionRegister.CallFunction(functionName, functionCallData)
				log.Infof(ctx, "普通函数调用暂未实现: %s", functionName)
			}
		}
	}

	// ========== 步骤6: 处理剩余文本 ==========
	// 处理流式响应结束后可能剩余的未播放文本
	fullResponse := utils.JoinStrings(responseMessage)
	if len(fullResponse) > processedChars {
		remainingText := fullResponse[processedChars:]
		if remainingText != "" {
			textIndex++
			log.Info(ctx, fmt.Sprintf("LLM回复分段[剩余文本]: %s, index: %d, round:%d", remainingText, textIndex, round))
			h.ttsLastTextIndex = textIndex
			h.SpeakAndPlay(ctx, remainingText, textIndex, round)
		}
	} else {
		log.Debugf(ctx, "无剩余文本需要处理: fullResponse长度=%d, processedChars=%d", len(fullResponse), processedChars)
	}

	// 获取完整响应内容，用于对话历史和情绪分析
	content := utils.JoinStrings(responseMessage)

	// ========== 步骤7: 对话历史管理 ==========
	// 只有非工具调用的响应才保存为assistant消息
	if !toolCallFlag {
		assistantMessage := chat.Message{
			Role:    "assistant",
			Content: content,
		}
		h.dialogueManager.Put(assistantMessage)

		// ========== 异步数据库保存 ==========
		// 使用goroutine异步保存，避免阻塞用户交互
		go func() {
			err := h.conversationService.SaveConversation(
				ctx,
				h.sessionID,
				h.deviceID,
				h.clientId,
				assistantMessage.Content,
				int(models.RoleAssistant),   // MessageRole - 助手消息
				int(models.ContentTypeText), // MessageType - 文本消息
				h.currentAIRole,
			)
			if err != nil {
				log.Errorf(ctx, "保存助手回复到数据库失败: %v", err)
			}
		}()
	}

	// ========== 流程完成 ==========
	// 返回nil表示处理成功，整个LLM响应流程完成
	return nil
}

func (h *ConnectionHandler) handleFunctionResult(ctx context.Context, result types.ActionResponse, functionCallData map[string]interface{}, textIndex int) {
	switch result.Action {
	case types.ActionTypeError:
		log.Errorf(ctx, "函数调用错误: %v", result.Result)
	case types.ActionTypeNotFound:
		log.Errorf(ctx, "函数未找到: %v", result.Result)
	case types.ActionTypeNone:
		log.Infof(ctx, "函数调用无操作: %v", result.Result)
	case types.ActionTypeResponse:
		log.Infof(ctx, "函数调用直接回复: %v", result.Response)
		h.SystemSpeak(ctx, result.Response.(string))
	case types.ActionTypeCallHandler:
		h.handleMCPResultCall(ctx, result)
	case types.ActionTypeReqLLM:
		log.Infof(ctx, "函数调用后请求LLM: %v", result.Result)
		text, ok := result.Result.(string)
		if ok && len(text) > 0 {
			functionID := functionCallData["id"].(string)
			functionName := functionCallData["name"].(string)
			functionArguments := functionCallData["arguments"].(string)
			log.Infof(ctx, "函数调用结果: %s", text)
			log.Infof(ctx, "函数调用参数: %s", functionArguments)
			log.Infof(ctx, "函数调用名称: %s", functionName)
			log.Infof(ctx, "函数调用ID: %s", functionID)

			// 添加 assistant 消息，包含 tool_calls
			h.dialogueManager.Put(chat.Message{
				Role: "assistant",
				ToolCalls: []types.ToolCall{{
					ID: functionID,
					Function: types.FunctionCall{
						Arguments: functionArguments,
						Name:      functionName,
					},
					Type:  "function",
					Index: 0,
				}},
			})

			// 添加 tool 消息
			toolCallID := functionID
			if toolCallID == "" {
				toolCallID = uuid.New().String()
			}
			h.dialogueManager.Put(chat.Message{
				Role:       "tool",
				ToolCallID: toolCallID,
				Content:    text,
			})
			h.genResponseByLLM(ctx, h.dialogueManager.GetLLMDialogue(), h.talkRound)

		} else {
			log.Errorf(ctx, "函数调用结果解析失败: %v", result.Result)
			// 发送错误消息
			errorMessage := fmt.Sprintf("函数调用结果解析失败 %v", result.Result)
			h.SystemSpeak(ctx, errorMessage)
		}
	}
}

func (h *ConnectionHandler) SystemSpeak(ctx context.Context, text string) error {
	if text == "" {
		log.Warn(ctx, "SystemSpeak 收到空文本，无法合成语音")
		return errors.New("收到空文本，无法合成语音")
	}
	texts := utils.SplitByPunctuation(text)
	index := 0
	for _, item := range texts {
		index++
		h.ttsLastTextIndex = index // 重置文本索引
		h.SpeakAndPlay(ctx, item, index, h.talkRound)
	}
	return nil
}

// isNeedAuth 判断是否需要验证
func (h *ConnectionHandler) isNeedAuth() bool {
	if !h.config.Server.Auth.Enabled {
		return false
	}
	return !h.isDeviceVerified
}

// checkAndBroadcastAuthCode 检查并广播认证码
func (h *ConnectionHandler) checkAndBroadcastAuthCode(ctx context.Context) error {
	// 这里简化了认证逻辑，实际需要根据具体需求实现
	text := "请联系管理员进行设备认证"
	return h.SpeakAndPlay(ctx, text, 0, h.talkRound)
}

// processTTSQueueCoroutine 处理TTS队列
func (h *ConnectionHandler) processTTSQueueCoroutine(ctx context.Context) {
	for {
		select {
		case <-h.stopChan:
			return
		case task := <-h.ttsQueue:
			h.processTTSTask(ctx, task.text, task.textIndex, task.round)
		}
	}
}

// 服务端打断说话
func (h *ConnectionHandler) stopServerSpeak(ctx context.Context) {
	log.Info(ctx, "服务端停止说话")
	atomic.StoreInt32(&h.serverVoiceStop, 1)
	h.cleanTTSAndAudioQueue(ctx, false)
}

func (h *ConnectionHandler) deleteAudioFileIfNeeded(ctx context.Context, filepath string, reason string) {
	if !h.config.DeleteAudio || filepath == "" {
		return
	}

	// 检查是否为快速回复缓存文件，如果是则不删除
	if h.quickReplyCache != nil && h.quickReplyCache.IsCachedFile(filepath) {
		log.Info(ctx, fmt.Sprintf(reason+" 跳过删除缓存音频文件: %s", filepath))
		return
	}

	// 检查是否是音乐文件，如果是则不删除
	if utils.IsMusicFile(filepath) {
		log.Info(ctx, fmt.Sprintf(reason+" 跳过删除音乐文件: %s", filepath))
		return
	}

	// 删除非缓存音频文件
	if err := os.Remove(filepath); err != nil {
		log.Error(ctx, fmt.Sprintf(reason+" 删除音频文件失败: %v", err))
	} else {
		log.Debug(ctx, fmt.Sprintf(reason+" 已删除音频文件: %s", filepath))
	}
}

// processTTSTask 处理单个TTS任务
func (h *ConnectionHandler) processTTSTask(ctx context.Context, text string, textIndex int, round int) {
	filepath := ""
	defer func() {
		h.audioMessagesQueue <- struct {
			filepath  string
			text      string
			round     int
			textIndex int
		}{filepath, text, round, textIndex}
	}()

	if utils.IsQuickReplyHit(text, h.config.QuickReplyWords) {
		// 尝试从缓存查找音频文件
		if cachedFile := h.quickReplyCache.FindCachedAudio(text); cachedFile != "" {
			log.Infof(ctx, "使用缓存的快速回复音频: %s", cachedFile)
			filepath = cachedFile
			return
		}
	}
	ttsStartTime := time.Now()
	// 过滤表情
	text = utils.RemoveAllEmoji(text)

	if text == "" {
		log.Warnf(ctx, "收到空文本，无法合成语音, 索引: %d", textIndex)
		return
	}

	// 生成语音文件
	filepath, err := h.providers.tts.ToTTS(text)
	if err != nil {
		log.Errorf(ctx, "TTS转换失败:text(%s) %v", text, err)
		return
	} else {
		log.Debugf(ctx, "TTS转换成功: text(%s), index(%d) %s", text, textIndex, filepath)
		// 如果是快速回复词，保存到缓存
		if utils.IsQuickReplyHit(text, h.config.QuickReplyWords) {
			if err := h.quickReplyCache.SaveCachedAudio(text, filepath); err != nil {
				log.Errorf(ctx, "保存快速回复音频失败: %v", err)
			} else {
				log.Infof(ctx, "成功缓存快速回复音频: %s", text)
			}
		}
	}
	if atomic.LoadInt32(&h.serverVoiceStop) == 1 { // 服务端语音停止
		log.Infof(ctx, "processTTSTask 服务端语音停止, 不再发送音频数据：%s", text)
		// 服务端语音停止时，根据配置删除已生成的音频文件
		h.deleteAudioFileIfNeeded(ctx, filepath, "服务端语音停止时")
		return
	}

	if textIndex == 1 {
		now := time.Now()
		ttsSpentTime := now.Sub(ttsStartTime)
		log.Debugf(ctx, "TTS转换耗时: %s, 文本: %s, 索引: %d", ttsSpentTime, text, textIndex)
	}

}

// SpeakAndPlay 合成并播放语音
func (h *ConnectionHandler) SpeakAndPlay(ctx context.Context, text string, textIndex int, round int) error {
	defer func() {
		// 将任务加入队列，不阻塞当前流程
		h.ttsQueue <- struct {
			text      string
			round     int
			textIndex int
		}{text, round, textIndex}
	}()

	originText := text // 保存原始文本用于日志
	text = utils.RemoveAllEmoji(text)
	text = utils.RemoveMarkdownSyntax(text) // 移除Markdown语法
	if text == "" {
		log.Warnf(ctx, "SpeakAndPlay 收到空文本，无法合成语音, %d, text:%s.", textIndex, originText)
		return errors.New("收到空文本，无法合成语音")
	}

	if atomic.LoadInt32(&h.serverVoiceStop) == 1 { // 服务端语音停止
		log.Infof(ctx, "speakAndPlay 服务端语音停止, 不再发送音频数据：%s", text)
		text = ""
		return errors.New("服务端语音已停止，无法合成语音")
	}

	if len(text) > 255 {
		log.Warnf(ctx, "文本过长，超过255字符限制，截断合成语音: %s", text)
		text = text[:255] // 截断文本
	}

	return nil
}

func (h *ConnectionHandler) clearSpeakStatus(ctx context.Context) {
	log.Info(ctx, "清除服务端讲话状态 ")
	h.ttsLastTextIndex = -1
	h.providers.asr.Reset(ctx) // 重置ASR状态
}

func (h *ConnectionHandler) closeOpusDecoder(ctx context.Context) {
	if h.opusDecoder != nil {
		if err := h.opusDecoder.Close(); err != nil {
			log.Errorf(ctx, "关闭Opus解码器失败: %v", err)
		}
		h.opusDecoder = nil
	}
}

func (h *ConnectionHandler) cleanTTSAndAudioQueue(ctx context.Context, bClose bool) error {
	msgPrefix := ""
	if bClose {
		msgPrefix = "关闭连接，"
	}
	// 终止tts任务，不再继续将文本加入到tts队列，清空ttsQueue队列
	for {
		select {
		case task := <-h.ttsQueue:
			log.Infof(ctx, "%v: 丢弃一个TTS任务: %s", msgPrefix, task.text)
		default:
			// 队列已清空，退出循环
			log.Infof(ctx, "%v: ttsQueue队列已清空，停止处理TTS任务,准备清空音频队列", msgPrefix)
			goto clearAudioQueue
		}
	}

clearAudioQueue:
	// 终止audioMessagesQueue发送，清空队列里的音频数据
	for {
		select {
		case task := <-h.audioMessagesQueue:
			log.Infof(ctx, msgPrefix+"丢弃一个音频任务: %s", task.text)
			// 根据配置删除被丢弃的音频文件
			h.deleteAudioFileIfNeeded(ctx, task.filepath, msgPrefix+"丢弃音频任务时")
		default:
			// 队列已清空，退出循环
			log.Infof(ctx, "%v: audioMessagesQueue队列已清空，停止处理音频任务", msgPrefix)
			return nil
		}
	}
}

// Close 清理资源
func (h *ConnectionHandler) Close(ctx context.Context) {
	h.closeOnce.Do(func() {
		close(h.stopChan)

		h.closeOpusDecoder(ctx)
		if h.providers.tts != nil {
			h.providers.tts.SetVoice(ctx, h.initialVoice) // 恢复初始语音
		}
		if h.providers.asr != nil {
			if err := h.providers.asr.Reset(ctx); err != nil {
				log.Errorf(ctx, "重置ASR状态失败: %v", err)
			}
		}
		h.cleanTTSAndAudioQueue(ctx, true)
	})
}

// genResponseByVLLM 使用VLLLM处理包含图片的消息
func (h *ConnectionHandler) genResponseByVLLM(ctx context.Context, messages []providers.Message, imageData image.ImageData, text string, round int) error {
	log.Infof(ctx, "开始生成VLLLM回复 text:%s, has_url:%v, has_data:%v, format:%s, message_count:%d",
		text, imageData.URL != "", imageData.Data != "", imageData.Format, len(messages))

	// 使用VLLLM处理图片和文本
	responses, err := h.providers.vlllm.ResponseWithImage(ctx, h.sessionID, messages, imageData, text)
	if err != nil {
		log.Errorf(ctx, "VLLLM生成回复失败，尝试降级到普通LLM: %v", err)
		// 降级策略：只使用文本部分调用普通LLM
		fallbackText := fmt.Sprintf("用户发送了一张图片并询问：%s（注：当前无法处理图片，只能根据文字回答）", text)
		fallbackMessages := append(messages, providers.Message{
			Role:    "user",
			Content: fallbackText,
		})
		return h.genResponseByLLM(ctx, fallbackMessages, round)
	}

	// 处理VLLLM流式回复
	var responseMessage []string
	processedChars := 0
	textIndex := 0

	atomic.StoreInt32(&h.serverVoiceStop, 0)

	for response := range responses {
		if response == "" {
			continue
		}

		responseMessage = append(responseMessage, response)
		// 处理分段
		fullText := utils.JoinStrings(responseMessage)
		currentText := fullText[processedChars:]

		// 按标点符号分割
		if segment, chars := utils.SplitAtLastPunctuation(currentText); chars > 0 {
			textIndex++
			h.ttsLastTextIndex = textIndex
			h.SpeakAndPlay(ctx, segment, textIndex, round)
			processedChars += chars
		}
	}

	// 处理剩余文本
	remainingText := utils.JoinStrings(responseMessage)[processedChars:]
	if remainingText != "" {
		textIndex++
		h.ttsLastTextIndex = textIndex
		h.SpeakAndPlay(ctx, remainingText, textIndex, round)
	}

	// 获取完整回复内容
	content := utils.JoinStrings(responseMessage)

	// 添加VLLLM回复到对话历史
	h.dialogueManager.Put(chat.Message{
		Role:    "assistant",
		Content: content,
	})

	log.Infof(ctx, "VLLLM回复处理完成 …%v", map[string]interface{}{
		"content_length": len(content),
		"text_segments":  textIndex,
	})

	return nil
}
