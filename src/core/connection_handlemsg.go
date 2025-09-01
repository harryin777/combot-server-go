package core

import (
	"combot-server-go/src/core/chat"
	"combot-server-go/src/core/image"
	"combot-server-go/src/core/providers"
	"combot-server-go/src/core/utils"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// handleMessage 处理接收到的消息
func (h *ConnectionHandler) handleMessage(ctx context.Context, messageType int, message []byte) error {
	switch messageType {
	case 1: // 文本消息
		h.clientTextQueue <- string(message)
		return nil
	case 2: // 二进制消息（音频数据）
		if h.clientAudioFormat == "pcm" {
			// 直接将PCM数据放入队列
			h.clientAudioQueue <- message
		} else if h.clientAudioFormat == "opus" {
			// 检查是否初始化了opus解码器
			if h.opusDecoder != nil {
				// 解码opus数据为PCM
				decodedData, err := h.opusDecoder.Decode(message)
				if err != nil {
					utils.Errorf(ctx, "解码Opus音频失败: %v", err)
					// 即使解码失败，也尝试将原始数据传递给ASR处理
					h.clientAudioQueue <- message
				} else {
					// 解码成功，将PCM数据放入队列
					utils.Debugf(ctx, "Opus解码成功: %d bytes -> %d bytes", len(message), len(decodedData))
					if len(decodedData) > 0 {
						h.clientAudioQueue <- decodedData
					}
				}
			} else {
				utils.Warnf(ctx, "没有初始化Opus解码器，无法解码音频")
				// 没有解码器，直接传递原始数据
				h.clientAudioQueue <- message
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
	if err := json.Unmarshal([]byte(text), &msgJSON); err != nil {
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
		return h.handleIotMessage(msgMap)
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
	// TODO: 实现具体逻辑
	return nil
}

// handleListenMessage 处理语音相关消息
func (h *ConnectionHandler) handleListenMessage(ctx context.Context, msgMap map[string]interface{}) error {
	// TODO: 实现具体逻辑
	return nil
}

// handleIotMessage 处理IOT设备消息
func (h *ConnectionHandler) handleIotMessage(msgMap map[string]interface{}) error {
	if descriptors, ok := msgMap["descriptors"].([]interface{}); ok {
		// 处理设备描述符
		// 这里需要实现具体的IOT设备描述符处理逻辑
		utils.Infof(h.ctx, "收到IOT设备描述符：%v", descriptors)
	}
	if states, ok := msgMap["states"].([]interface{}); ok {
		// 处理设备状态
		// 这里需要实现具体的IOT设备状态处理逻辑
		utils.Infof(h.ctx, "收到IOT设备状态：%v", states)
	}
	return nil
}

// handleImageMessage 处理图片消息
func (h *ConnectionHandler) handleImageMessage(ctx context.Context, msgMap map[string]interface{}) error {
	// 增加对话轮次
	h.talkRound++
	currentRound := h.talkRound
	utils.Infof(h.ctx, "开始新的图片对话轮次: %d", currentRound)

	// 判断是否需要验证
	if h.isNeedAuth() {
		if err := h.checkAndBroadcastAuthCode(); err != nil {
			utils.Errorf(ctx, "检查认证码失败: %v", err)
			return err
		}
		utils.Info(h.ctx, "设备未认证，等待管理员认证")
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

	utils.Infof(h.ctx, "收到图片消息 %v", map[string]interface{}{
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
	if err := h.sendTTSMessage("start", "", 0); err != nil {
		utils.Errorf(ctx, "发送TTS开始状态失败: %v", err)
		return fmt.Errorf("发送TTS开始状态失败: %v", err)
	}

	// 发送思考状态的情绪
	if err := h.sendEmotionMessage("thinking"); err != nil {
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
