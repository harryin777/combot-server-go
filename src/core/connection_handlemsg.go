package core

import (
	"combot-server-go/src/core/chat"
	"combot-server-go/src/core/image"
	"combot-server-go/src/core/providers"
	"combot-server-go/src/core/utils"
	"context"
	"encoding/binary"
	"fmt"
	"strings"

	jsoniter "github.com/json-iterator/go"
)

// handleMessage 处理接收到的消息
func (h *ConnectionHandler) handleMessage(ctx context.Context, messageType int, message []byte) error {
	switch messageType {
	case 1: // 文本消息
		h.clientTextQueue <- string(message)
		return nil
	case 2: // 二进制消息（音频数据）
		// 解析二进制协议数据
		audioData, err := h.parseBinaryAudioMessage(ctx, message)
		if err != nil {
			utils.Errorf(ctx, "解析二进制音频消息失败: %v", err)
			return err
		}

		if h.clientAudioFormat == "pcm" {
			// 直接将PCM数据放入队列
			h.clientAudioQueue <- audioData
		} else if h.clientAudioFormat == "opus" {
			// 检查是否初始化了opus解码器
			if h.opusDecoder != nil {
				// 解码opus数据为PCM
				decodedData, err := h.opusDecoder.Decode(audioData)
				if err != nil {
					utils.Errorf(ctx, "解码Opus音频失败: %v", err)
					// 即使解码失败，也尝试将原始数据传递给ASR处理
					h.clientAudioQueue <- audioData
				} else {
					// 解码成功，将PCM数据放入队列
					utils.Debugf(ctx, "Opus解码成功: %d bytes -> %d bytes", len(audioData), len(decodedData))
					if len(decodedData) > 0 {
						h.clientAudioQueue <- decodedData
					}
				}
			} else {
				utils.Warnf(ctx, "没有初始化Opus解码器，无法解码音频")
				// 没有解码器，直接传递原始数据
				h.clientAudioQueue <- audioData
			}
		}
		return nil
	default:
		utils.Errorf(ctx, "未知的消息类型: %d", messageType)
		return fmt.Errorf("未知的消息类型: %d", messageType)
	}

}

func (h *ConnectionHandler) handleVisionMessage(msgMap map[string]interface{}) error {
	// 处理视觉消息
	cmd := msgMap["cmd"].(string)
	if cmd == "gen_pic" {
	} else if cmd == "gen_video" {
	} else if cmd == "read_img" {
	}
	return nil
}

// processClientTextMessage 处理文本数据
func (h *ConnectionHandler) processClientTextMessage(ctx context.Context, text string) error {
	// 解析JSON消息
	var msgJSON interface{}
	if err := jsoniter.Unmarshal([]byte(text), &msgJSON); err != nil {
		return h.conn.WriteMessage(1, []byte(text))
	}

	// 检查是否为整数类型
	if _, ok := msgJSON.(float64); ok {
		return h.conn.WriteMessage(1, []byte(text))
	}

	// 解析为map类型处理具体消息
	msgMap, ok := msgJSON.(map[string]interface{})
	if !ok {
		return fmt.Errorf("消息格式错误")
	}

	// 根据消息类型分发处理
	msgType, ok := msgMap["type"].(string)
	if !ok {
		return fmt.Errorf("消息类型错误")
	}

	switch msgType {
	case "hello":
		return h.handleHelloMessage(ctx, msgMap)
	case "abort":
		return h.clientAbortChat(ctx)
	case "listen":
		return h.handleListenMessage(ctx, msgMap)
	case "iot":
		return h.handleIotMessage(ctx, msgMap)
	case "chat":
		return h.handleChatMessage(ctx, text)
	case "vision":
		return h.handleVisionMessage(msgMap)
	case "image":
		return h.handleImageMessage(ctx, msgMap)
	case "mcp":
		return h.mcpManager.HandleXiaoZhiMCPMessage(ctx, msgMap)
	default:
		utils.Warnf(ctx, "=== 未知消息类型 ===: %s, full_message: %v", msgType, msgMap)
		return fmt.Errorf("未知的消息类型: %s", msgType)
	}
}

// handleHelloMessage 处理欢迎消息
func (h *ConnectionHandler) handleHelloMessage(ctx context.Context, msgMap map[string]interface{}) error {
	utils.Infof(ctx, "收到设备端hello消息: %v", msgMap)

	// 解析设备端协议版本
	if version, ok := msgMap["version"]; ok {
		if v, ok := version.(float64); ok {
			h.clientProtocolVersion = int(v)
			utils.Infof(ctx, "设备端协议版本: %d", h.clientProtocolVersion)
		}
	}

	// 解析设备端功能特性
	if features, ok := msgMap["features"].(map[string]interface{}); ok {
		if mcp, exists := features["mcp"]; exists {
			if mcpSupported, ok := mcp.(bool); ok {
				h.clientSupportsMCP = mcpSupported
				utils.Infof(ctx, "设备端MCP支持: %t", mcpSupported)
			}
		}
		if aec, exists := features["aec"]; exists {
			if aecSupported, ok := aec.(bool); ok {
				h.clientSupportsAEC = aecSupported
				utils.Infof(ctx, "设备端AEC支持: %t", aecSupported)
			}
		}
	}

	// 解析设备端音频参数
	if audioParams, ok := msgMap["audio_params"].(map[string]interface{}); ok {
		if format, exists := audioParams["format"]; exists {
			if formatStr, ok := format.(string); ok {
				h.clientAudioFormat = formatStr
				utils.Infof(ctx, "设备端音频格式: %s", formatStr)
			}
		}
		if sampleRate, exists := audioParams["sample_rate"]; exists {
			if sr, ok := sampleRate.(float64); ok {
				h.clientAudioSampleRate = int(sr)
				utils.Infof(ctx, "设备端采样率: %d", h.clientAudioSampleRate)
			}
		}
		if channels, exists := audioParams["channels"]; exists {
			if ch, ok := channels.(float64); ok {
				h.clientAudioChannels = int(ch)
				utils.Infof(ctx, "设备端声道数: %d", h.clientAudioChannels)
			}
		}
		if frameDuration, exists := audioParams["frame_duration"]; exists {
			if fd, ok := frameDuration.(float64); ok {
				h.clientAudioFrameDuration = int(fd)
				utils.Infof(ctx, "设备端帧时长: %dms", h.clientAudioFrameDuration)
			}
		}
	}

	// 初始化Opus解码器（如果需要）
	if h.clientAudioFormat == "opus" && h.opusDecoder == nil {
		config := &utils.OpusDecoderConfig{
			SampleRate:  h.clientAudioSampleRate,
			MaxChannels: h.clientAudioChannels,
		}
		decoder, err := utils.NewOpusDecoder(config)
		if err != nil {
			utils.Errorf(ctx, "创建Opus解码器失败: %v", err)
		} else {
			h.opusDecoder = decoder
			utils.Infof(ctx, "Opus解码器初始化成功")
		}
	}

	// 发送hello响应消息
	return h.sendHelloMessage(ctx)
}

// handleListenMessage 处理语音相关消息
func (h *ConnectionHandler) handleListenMessage(ctx context.Context, msgMap map[string]interface{}) error {
	utils.Infof(ctx, "收到语音控制消息: %v", msgMap)

	// 解析监听模式
	if mode, ok := msgMap["mode"].(string); ok {
		h.clientListenMode = mode
		utils.Infof(ctx, "设置客户端监听模式: %s", mode)

		// 根据不同模式处理
		switch mode {
		case "start", "auto":
			h.clientListenMode = "auto"
			h.clientVoiceStop = false
			utils.Info(ctx, "开始自动语音识别")
		case "manual":
			h.clientListenMode = "manual"
			h.clientVoiceStop = false
			h.clientAsrText = "" // 重置ASR文本
			utils.Info(ctx, "开始手动语音识别")
		case "realtime":
			if h.clientSupportsAEC {
				h.clientListenMode = "realtime"
				h.clientVoiceStop = false
				utils.Info(ctx, "开始实时语音识别（需要AEC支持）")
			} else {
				utils.Warn(ctx, "设备不支持AEC，无法使用实时模式，降级为自动模式")
				h.clientListenMode = "auto"
			}
		case "stop":
			h.clientVoiceStop = true
			utils.Info(ctx, "停止语音识别")
		default:
			utils.Warnf(ctx, "未知的监听模式: %s", mode)
		}
	}

	// 处理其他语音相关参数
	if params, ok := msgMap["params"].(map[string]interface{}); ok {
		if timeout, exists := params["timeout"]; exists {
			if timeoutMs, ok := timeout.(float64); ok {
				utils.Infof(ctx, "设置语音超时时间: %.0fms", timeoutMs)
				// 可以在这里设置ASR超时时间
			}
		}

		if sensitivity, exists := params["sensitivity"]; exists {
			if sens, ok := sensitivity.(float64); ok {
				utils.Infof(ctx, "设置语音敏感度: %.2f", sens)
				// 可以在这里设置VAD敏感度
			}
		}
	}

	return nil
}

// handleIotMessage 处理IOT设备消息
func (h *ConnectionHandler) handleIotMessage(ctx context.Context, msgMap map[string]interface{}) error {
	utils.Infof(ctx, "收到IOT设备消息: %v", msgMap)

	// 处理设备描述符
	if descriptors, ok := msgMap["descriptors"].([]interface{}); ok {
		utils.Infof(ctx, "收到IOT设备描述符数量: %d", len(descriptors))
		for i, descriptor := range descriptors {
			if desc, ok := descriptor.(map[string]interface{}); ok {
				deviceId := ""
				deviceType := ""
				deviceName := ""

				if id, exists := desc["id"]; exists {
					if idStr, ok := id.(string); ok {
						deviceId = idStr
					}
				}
				if dtype, exists := desc["type"]; exists {
					if typeStr, ok := dtype.(string); ok {
						deviceType = typeStr
					}
				}
				if name, exists := desc["name"]; exists {
					if nameStr, ok := name.(string); ok {
						deviceName = nameStr
					}
				}

				utils.Infof(ctx, "设备描述符[%d]: ID=%s, Type=%s, Name=%s", i, deviceId, deviceType, deviceName)

				// 这里可以注册设备到MCP系统或其他设备管理系统
				if h.clientSupportsMCP && h.mcpManager != nil {
					// 可以通过MCP系统注册设备功能
					utils.Debugf(ctx, "通过MCP注册设备: %s", deviceId)
				}
			}
		}
	}

	// 处理设备状态
	if states, ok := msgMap["states"].([]interface{}); ok {
		utils.Infof(ctx, "收到IOT设备状态数量: %d", len(states))
		for i, state := range states {
			if st, ok := state.(map[string]interface{}); ok {
				deviceId := ""
				status := ""
				value := interface{}(nil)

				if id, exists := st["device_id"]; exists {
					if idStr, ok := id.(string); ok {
						deviceId = idStr
					}
				}
				if stat, exists := st["status"]; exists {
					if statStr, ok := stat.(string); ok {
						status = statStr
					}
				}
				if val, exists := st["value"]; exists {
					value = val
				}

				utils.Infof(ctx, "设备状态[%d]: DeviceID=%s, Status=%s, Value=%v", i, deviceId, status, value)

				// 这里可以更新设备状态到系统中
				// 例如存储到数据库或通知其他服务
			}
		}
	}

	// 处理设备控制命令
	if command, ok := msgMap["command"].(map[string]interface{}); ok {
		deviceId := ""
		action := ""
		params := map[string]interface{}{}

		if id, exists := command["device_id"]; exists {
			if idStr, ok := id.(string); ok {
				deviceId = idStr
			}
		}
		if act, exists := command["action"]; exists {
			if actStr, ok := act.(string); ok {
				action = actStr
			}
		}
		if par, exists := command["params"]; exists {
			if parMap, ok := par.(map[string]interface{}); ok {
				params = parMap
			}
		}

		utils.Infof(ctx, "设备控制命令: DeviceID=%s, Action=%s, Params=%v", deviceId, action, params)

		// 这里可以执行实际的设备控制逻辑
		// 例如通过MCP或其他协议发送控制命令
		if h.clientSupportsMCP && h.mcpManager != nil {
			// 可以通过MCP执行设备控制
			utils.Debugf(ctx, "通过MCP控制设备: %s -> %s", deviceId, action)
		}
	}

	return nil
}

// handleImageMessage 处理图片消息
func (h *ConnectionHandler) handleImageMessage(ctx context.Context, msgMap map[string]interface{}) error {
	// 增加对话轮次
	h.talkRound++
	currentRound := h.talkRound
	utils.Infof(ctx, "开始新的图片对话轮次: %d", currentRound)

	// 判断是否需要验证
	if h.isNeedAuth() {
		if err := h.checkAndBroadcastAuthCode(ctx); err != nil {
			utils.Errorf(ctx, "检查认证码失败: %v", err)
			return err
		}
		utils.Info(ctx, "设备未认证，等待管理员认证")
		return nil
	}

	// 检查是否有VLLLM Provider
	if h.providers.vlllm == nil {
		utils.Warnf(ctx, "未配置VLLLM服务，图片消息将被忽略")
		return h.conn.WriteMessage(1, []byte("系统暂不支持图片处理功能"))
	}

	// 解析文本内容
	text, ok := msgMap["text"].(string)
	if !ok {
		text = "请描述这张图片" // 默认提示
	}

	// 解析图片数据
	imageDataMap, ok := msgMap["image_data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("缺少图片数据")
	}

	imageData := image.ImageData{}
	if url, ok := imageDataMap["url"].(string); ok {
		imageData.URL = url
	}
	if data, ok := imageDataMap["data"].(string); ok {
		imageData.Data = data
	}
	if format, ok := imageDataMap["format"].(string); ok {
		imageData.Format = format
	}

	// 验证图片数据
	if imageData.URL == "" && imageData.Data == "" {
		return fmt.Errorf("图片数据为空")
	}

	utils.Infof(ctx, "收到图片消息 %v", map[string]interface{}{
		"text":        text,
		"has_url":     imageData.URL != "",
		"has_data":    imageData.Data != "",
		"format":      imageData.Format,
		"data_length": len(imageData.Data),
	})

	// 立即发送STT消息
	err := h.sendSTTMessage(text)
	if err != nil {
		utils.Errorf(ctx, "发送STT消息失败: %v", err)
		return fmt.Errorf("发送STT消息失败: %v", err)
	}

	// 发送TTS开始状态
	if err := h.sendTTSMessage(ctx, "start", "", 0); err != nil {
		utils.Errorf(ctx, "发送TTS开始状态失败: %v", err)
		return fmt.Errorf("发送TTS开始状态失败: %v", err)
	}

	// 发送思考状态的情绪
	if err := h.sendEmotionMessage(ctx, "thinking", 1); err != nil {
		utils.Errorf(ctx, "发送思考状态情绪消息失败: %v", err)
		return fmt.Errorf("发送情绪消息失败: %v", err)
	}

	// 添加用户消息到对话历史（包含图片信息的描述）
	userMessage := fmt.Sprintf("%s [用户发送了一张%s格式的图片]", text, imageData.Format)
	h.dialogueManager.Put(chat.Message{
		Role:    "user",
		Content: userMessage,
	})

	// 获取对话历史
	messages := make([]providers.Message, 0)
	for _, msg := range h.dialogueManager.GetLLMDialogue() {
		// 排除包含图片信息的最后一条消息，因为我们要用VLLLM处理
		if msg.Role == "user" && strings.Contains(msg.Content, "[用户发送了一张") {
			continue
		}
		messages = append(messages, providers.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return h.genResponseByVLLM(ctx, messages, imageData, text, currentRound)
}

// parseBinaryAudioMessage 解析二进制音频消息，支持不同协议版本
func (h *ConnectionHandler) parseBinaryAudioMessage(ctx context.Context, message []byte) ([]byte, error) {
	// 根据客户端协议版本解析消息
	switch h.clientProtocolVersion {
	case 2:
		// BinaryProtocol2: 16字节头部 + payload
		if len(message) < 16 {
			return nil, fmt.Errorf("BinaryProtocol2消息长度不足，需要至少16字节，实际%d字节", len(message))
		}

		// 解析头部字段 (大端序)
		// version := binary.BigEndian.Uint16(message[0:2])
		// msgType := binary.BigEndian.Uint16(message[2:4])
		// reserved := binary.BigEndian.Uint32(message[4:8])
		timestamp := binary.BigEndian.Uint32(message[8:12])
		payloadSize := binary.BigEndian.Uint32(message[12:16])

		utils.Debugf(ctx, "BinaryProtocol2: timestamp=%d, payloadSize=%d", timestamp, payloadSize)

		// 验证负载大小
		if len(message) < 16+int(payloadSize) {
			return nil, fmt.Errorf("BinaryProtocol2负载大小不匹配，期望%d字节，实际%d字节", 16+payloadSize, len(message))
		}

		// 提取音频数据
		return message[16 : 16+payloadSize], nil

	case 3:
		// BinaryProtocol3: 4字节头部 + payload
		if len(message) < 4 {
			return nil, fmt.Errorf("BinaryProtocol3消息长度不足，需要至少4字节，实际%d字节", len(message))
		}

		// 解析头部字段 (大端序)
		// msgType := message[0]
		// reserved := message[1]
		payloadSize := binary.BigEndian.Uint16(message[2:4])

		utils.Debugf(ctx, "BinaryProtocol3: payloadSize=%d", payloadSize)

		// 验证负载大小
		if len(message) < 4+int(payloadSize) {
			return nil, fmt.Errorf("BinaryProtocol3负载大小不匹配，期望%d字节，实际%d字节", 4+payloadSize, len(message))
		}

		// 提取音频数据
		return message[4 : 4+payloadSize], nil

	default:
		// 版本1或未知版本：直接返回原始数据（纯Opus）
		utils.Debugf(ctx, "使用协议版本1或默认处理，直接返回原始音频数据")
		return message, nil
	}
}
