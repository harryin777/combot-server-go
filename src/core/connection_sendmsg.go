package core

import (
	"combot-server-go/src/log"
	"combot-server-go/src/utils"
	"context"
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"time"

	jsoniter "github.com/json-iterator/go"
)

// sendHelloMessage 发送欢迎消息
func (h *ConnectionHandler) sendHelloMessage(ctx context.Context) error {
	// 添加安全检查
	if h.conn == nil {
		return fmt.Errorf("连接对象未初始化，无法发送hello消息")
	}

	// 其他可能的 nil 检查
	if h.config == nil {
		return fmt.Errorf("配置对象未初始化")
	}

	hello := make(map[string]interface{})
	hello["type"] = "hello"
	hello["version"] = 1
	hello["transport"] = "websocket"
	hello["session_id"] = h.sessionID

	// 音频参数协商 - 根据客户端参数调整服务器参数
	audioParams := map[string]interface{}{
		"format":         h.serverAudioFormat,
		"sample_rate":    h.serverAudioSampleRate,
		"channels":       h.serverAudioChannels,
		"frame_duration": h.serverAudioFrameDuration,
	}

	// 如果客户端已发送音频参数，进行协商
	if h.clientAudioSampleRate > 0 {
		// 可以根据客户端能力调整服务器音频参数
		// 例如：如果客户端支持更高采样率，服务器可以使用更高质量
		if h.clientAudioSampleRate >= 24000 {
			// 客户端支持24kHz，服务器可以使用24kHz提供更好音质
			audioParams["sample_rate"] = 24000
			h.serverAudioSampleRate = 24000
		} else {
			// 客户端只支持16kHz，服务器降级到16kHz
			audioParams["sample_rate"] = 16000
			h.serverAudioSampleRate = 16000
		}
	}

	hello["audio_params"] = audioParams

	// 添加服务器能力信息
	capabilities := map[string]interface{}{
		"mcp":        true,                     // 服务器支持MCP协议，这里会告诉 combot 发送自己支持的工具到服务器，然后服务器会把设备支持的工具注册到内存
		"vlllm":      h.providers.vlllm != nil, // 是否支持视觉语言模型
		"multi_turn": true,                     // 支持多轮对话
		"functions":  true,                     // 支持函数调用
	}
	hello["capabilities"] = capabilities

	// 如果客户端支持特定功能，添加相关配置
	if h.clientSupportsMCP {
		hello["mcp_config"] = map[string]interface{}{
			"enabled": true,
			"tools":   []string{"system", "display", "camera"}, // 可用的MCP工具
		}
	}

	data, err := jsoniter.Marshal(hello)
	if err != nil {
		return fmt.Errorf("序列化欢迎消息失败: %v", err)
	}

	log.Infof(ctx, "开始发送hello响应消息 (长度=%d字节): %s", len(data), string(data))

	err = h.conn.WriteMessage(1, data)
	if err != nil {
		log.Errorf(ctx, "发送hello响应消息失败: %v", err)
		return fmt.Errorf("发送hello响应消息失败: %v", err)
	}

	log.Infof(ctx, "hello响应消息发送成功")
	return nil
}

func (h *ConnectionHandler) sendTTSMessage(ctx context.Context, state string, text string, textIndex int) error {
	// 发送TTS状态结束通知
	stateMsg := map[string]interface{}{
		"type":        "tts",
		"state":       state,
		"session_id":  h.sessionID,
		"text":        text,
		"index":       textIndex,
		"audio_codec": "opus", // 标识使用Opus编码
	}
	data, err := jsoniter.Marshal(stateMsg)
	if err != nil {
		log.Errorf(ctx, "序列化%s状态失败: %v", state, err)
		return fmt.Errorf("序列化%s状态失败: %v", state, err)
	}
	if err := h.conn.WriteMessage(1, data); err != nil {
		return fmt.Errorf("发送%s状态失败: %v", state, err)
	}
	return nil
}

func (h *ConnectionHandler) sendSTTMessage(text string) error {
	sttMsg := map[string]interface{}{
		"type":       "stt",
		"text":       text,
		"session_id": h.sessionID,
	}
	jsonData, err := jsoniter.Marshal(sttMsg)
	if err != nil {
		return fmt.Errorf("序列化 STT 消息失败: %v", err)
	}
	if err := h.conn.WriteMessage(1, jsonData); err != nil {
		return fmt.Errorf("发送 STT 消息失败: %v", err)
	}

	return nil
}

// sendEmotionMessage 发送情绪消息
func (h *ConnectionHandler) sendEmotionMessage(ctx context.Context, emotionType string, intensityLevel int) error {
	data := map[string]interface{}{
		"type":       "llm",
		"text":       utils.GetEmotionEmoji(emotionType),
		"emotion":    emotionType,
		"session_id": h.sessionID,
	}
	jsonData, err := jsoniter.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化情绪消息失败: %v", err)
	}
	return h.conn.WriteMessage(1, jsonData)
}

// sendListenState 将当前监听状态推送给设备端，例如在判定静音后通知停止聆听
func (h *ConnectionHandler) sendListenState(ctx context.Context, state string) error {
	msg := map[string]interface{}{
		"type":       "listen",
		"state":      state,
		"session_id": h.sessionID,
	}
	data, err := jsoniter.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化listen消息失败: %v", err)
	}
	if err := h.conn.WriteMessage(1, data); err != nil {
		return fmt.Errorf("发送listen消息失败: %v", err)
	}
	return nil
}

func (h *ConnectionHandler) sendAudioMessage(ctx context.Context, filepath string, text string, textIndex int, round int) {
	bFinishSuccess := false
	defer func() {
		// 音频发送完成后，根据配置决定是否删除文件
		h.deleteAudioFileIfNeeded(ctx, filepath, "音频发送完成")

		log.Infof(ctx, "TTS音频发送任务结束(%t): %s, 索引: %d/%d", bFinishSuccess, text, textIndex, h.ttsLastTextIndex)
		h.providers.asr.ResetStartListenTime()
		if textIndex == h.ttsLastTextIndex {
			h.sendTTSMessage(ctx, "stop", "", textIndex)
			if h.closeAfterChat {
				h.Close(ctx)
			} else {
				h.clearSpeakStatus(ctx)
			}
		}
	}()

	if len(filepath) == 0 {
		return
	}
	// 检查轮次
	if round != h.talkRound {
		log.Infof(ctx, "sendAudioMessage: 跳过过期轮次的音频: 任务轮次=%d, 当前轮次=%d, 文本=%s",
			round, h.talkRound, text)
		// 即使跳过，也要根据配置删除音频文件
		h.deleteAudioFileIfNeeded(ctx, filepath, "跳过过期轮次")
		return
	}

	if atomic.LoadInt32(&h.serverVoiceStop) == 1 { // 服务端语音停止
		log.Infof(ctx, "sendAudioMessage 服务端语音停止, 不再发送音频数据：%s", text)
		// 服务端语音停止时也要根据配置删除音频文件
		h.deleteAudioFileIfNeeded(ctx, filepath, "服务端语音停止")
		return
	}

	var audioData [][]byte
	var duration float64
	var err error

	// 使用TTS提供者的方法将音频转为Opus格式
	if h.serverAudioFormat == "pcm" {
		log.Info(ctx, "服务端音频格式为PCM，直接发送")
		audioData, duration, err = utils.AudioToPCMData(filepath)
		if err != nil {
			log.Errorf(ctx, "音频转PCM失败: %v", err)
			return
		}
	} else if h.serverAudioFormat == "opus" {
		audioData, duration, err = utils.AudioToOpusData(filepath)
		if err != nil {
			log.Errorf(ctx, "音频转Opus失败: %v", err)
			return
		}
	}

	// 发送TTS状态开始通知
	if err := h.sendTTSMessage(ctx, "sentence_start", text, textIndex); err != nil {
		log.Errorf(ctx, "发送TTS开始状态失败: %v", err)
		return
	}

	if textIndex == 1 {
		now := time.Now()
		spentTime := now.Sub(h.roundStartTime)
		log.Debugf(ctx, "回复首句耗时 %s 第一句话【%s】, round: %d", spentTime, text, round)
	}
	log.Debugf(ctx, "TTS发送(%s): \"%s\" (索引:%d/%d，时长:%f，帧数:%d)", h.serverAudioFormat, text, textIndex, h.ttsLastTextIndex, duration, len(audioData))

	// 分时发送音频数据
	if err := h.sendAudioFrames(ctx, audioData, text, round); err != nil {
		log.Errorf(ctx, "分时发送音频数据失败: %v", err)
		return
	}

	// 发送TTS状态结束通知
	if err := h.sendTTSMessage(ctx, "sentence_end", text, textIndex); err != nil {
		log.Errorf(ctx, "发送TTS结束状态失败: %v", err)
		return
	}

	bFinishSuccess = true
}

// sendAudioFrames 分时发送音频帧，避免撑爆客户端缓冲区
func (h *ConnectionHandler) sendAudioFrames(ctx context.Context, audioData [][]byte, text string, round int) error {
	if len(audioData) == 0 {
		return nil
	}

	startTime := time.Now()
	playPosition := 0 // 播放位置（毫秒）

	// 预缓冲：发送前几帧，提升播放流畅度
	preBufferFrames := 3
	if len(audioData) < preBufferFrames {
		preBufferFrames = len(audioData)
	}
	preBufferTime := time.Duration(h.serverAudioFrameDuration*preBufferFrames) * time.Millisecond // 预缓冲时间（毫秒）

	// 发送预缓冲帧
	for i := 0; i < preBufferFrames; i++ {
		// 检查是否被打断
		if atomic.LoadInt32(&h.serverVoiceStop) == 1 || round != h.talkRound {
			log.Infof(ctx, "音频发送被中断(预缓冲阶段): 帧=%d/%d, 文本=%s", i+1, preBufferFrames, text)
			return nil
		}

		if err := h.sendAudioFrame(audioData[i], 0); err != nil {
			return fmt.Errorf("发送预缓冲音频帧失败: %v", err)
		}
		playPosition += h.serverAudioFrameDuration
	}

	// 发送剩余音频帧
	remainingFrames := audioData[preBufferFrames:]
	for i, chunk := range remainingFrames {
		// 检查是否被打断或轮次变化
		if atomic.LoadInt32(&h.serverVoiceStop) == 1 || round != h.talkRound {
			log.Infof(ctx, "音频发送被中断: 帧=%d/%d, 文本=%s", i+preBufferFrames+1, len(audioData), text)
			return nil
		}

		// 检查连接是否关闭
		select {
		case <-h.stopChan:
			return nil
		default:
		}

		// 计算预期发送时间
		expectedTime := startTime.Add(time.Duration(playPosition)*time.Millisecond - preBufferTime)
		currentTime := time.Now()
		delay := expectedTime.Sub(currentTime)

		// 流控延迟处理
		if delay > 0 {
			// 使用简单的可中断睡眠
			ticker := time.NewTicker(10 * time.Millisecond) // 固定10ms检查间隔
			defer ticker.Stop()

			endTime := time.Now().Add(delay)
			for time.Now().Before(endTime) {
				select {
				case <-ticker.C:
					// 检查中断条件
					if atomic.LoadInt32(&h.serverVoiceStop) == 1 || round != h.talkRound {
						log.Infof(ctx, "音频发送在延迟中被中断: 帧=%d/%d, 文本=%s", i+preBufferFrames+1, len(audioData), text)
						return nil
					}
				case <-h.stopChan:
					return nil
				}
			}
		}

		// 发送音频帧
		if err := h.sendAudioFrame(chunk, uint32(playPosition)); err != nil {
			return fmt.Errorf("发送音频帧失败: %v", err)
		}

		playPosition += h.serverAudioFrameDuration
	}
	time.Sleep(preBufferTime) // 确保预缓冲时间已过，这里需要等待这么久的原因是缓冲帧是一起发送给客户端的
	// 就是缓冲帧之间没有间隔，其他的帧都是有流控处理的
	spentTime := time.Since(startTime).Milliseconds()
	log.Infof(ctx, "音频帧发送完成: 总帧数=%d, 总时长=%dms, 总耗时:%dms 文本=%s", len(audioData), playPosition, spentTime, text)
	return nil
}

// sendAudioFrame 根据客户端协议版本发送音频帧
func (h *ConnectionHandler) sendAudioFrame(audioData []byte, timestamp uint32) error {
	var frameData []byte

	switch h.clientProtocolVersion {
	case 2:
		// BinaryProtocol2: 16字节头部 + payload
		frameData = make([]byte, 16+len(audioData))

		// 填充头部 (大端序)
		binary.BigEndian.PutUint16(frameData[0:2], uint16(h.clientProtocolVersion)) // version
		binary.BigEndian.PutUint16(frameData[2:4], 0)                               // type (0=OPUS)
		binary.BigEndian.PutUint32(frameData[4:8], 0)                               // reserved
		binary.BigEndian.PutUint32(frameData[8:12], timestamp)                      // timestamp
		binary.BigEndian.PutUint32(frameData[12:16], uint32(len(audioData)))        // payload_size

		// 复制音频数据
		copy(frameData[16:], audioData)

	case 3:
		// BinaryProtocol3: 4字节头部 + payload
		frameData = make([]byte, 4+len(audioData))

		// 填充头部 (大端序)
		frameData[0] = 0                                                   // type (0=OPUS)
		frameData[1] = 0                                                   // reserved
		binary.BigEndian.PutUint16(frameData[2:4], uint16(len(audioData))) // payload_size

		// 复制音频数据
		copy(frameData[4:], audioData)

	default:
		// 协议版本1或默认：直接发送Opus数据
		frameData = audioData
	}

	return h.conn.WriteMessage(2, frameData)
}
