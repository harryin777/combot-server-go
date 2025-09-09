package core

import (
	"combot-server-go/src/core/types"
	"combot-server-go/src/log"
	"combot-server-go/src/utils"
	"combot-server-go/src/vision"
	"context"
	"encoding/json"
)

const (
	mcpPlayMusic   = "play_music"
	mcpChangeVoice = "change_voice"
	mcpChangeRole  = "change_role"
	mcpExit        = "exit"
	mcpTakePhoto   = "take_photo"
)

// initMCPResultHandlers 初始化MCP结果处理器映射
func (h *ConnectionHandler) initMCPResultHandlers() {
	// 初始化MCP结果处理器映射
	h.mcpResultHandlers = map[string]func(context.Context, interface{}){
		mcpPlayMusic:   h.mcpHandlerPlayMusic,
		mcpChangeVoice: h.mcpHandlerChangeVoice,
		mcpChangeRole:  h.mcpHandlerChangeRole,
		mcpExit:        h.mcpHandlerExit,
		mcpTakePhoto:   h.mcpHandlerTakePhoto,
	}
}

// handleMCPResultCall 处理MCP返回的函数调用结果
func (h *ConnectionHandler) handleMCPResultCall(ctx context.Context, result types.ActionResponse) {
	// 先取result
	if result.Action != types.ActionTypeCallHandler {
		log.Errorf(ctx, "handleMCPResultCall: result.Action is not ActionTypeCallHandler, but %d", result.Action)
		return
	}
	if result.Result == nil {
		log.Error(ctx, "handleMCPResultCall: result.Result is nil")
		return
	}

	// 取出result.Result结构体，包括函数名和参数
	if Caller, ok := result.Result.(types.ActionResponseCall); ok {
		if handler, exists := h.mcpResultHandlers[Caller.FuncName]; exists {
			// 调用对应的处理函数
			handler(ctx, Caller.Args)
		} else {
			log.Errorf(ctx, "handleMCPResultCall: no handler found for function %s", Caller.FuncName)
		}
	} else {
		log.Error(ctx, "handleMCPResultCall: result.Result is not a map[string]interface{}")
	}
}

// mcpHandlerPlayMusic 处理播放音乐的MCP函数调用
func (h *ConnectionHandler) mcpHandlerPlayMusic(ctx context.Context, args interface{}) {
	if songName, ok := args.(string); ok {
		log.Infof(ctx, "mcp_handler_play_music: %s", songName)
		if path, name, err := utils.GetMusicFilePathFuzzy(songName); err != nil {
			log.Errorf(ctx, "mcp_handler_play_music: Play failed: %v", err)
			h.SystemSpeak(ctx, "没有找到名为"+songName+"的歌曲")
		} else {
			//h.SystemSpeak("这就为您播放音乐: " + songName)
			h.sendAudioMessage(ctx, path, name, h.ttsLastTextIndex, h.talkRound)
		}
	} else {
		log.Error(ctx, "mcp_handler_play_music: args is not a string")
	}
}

// mcpHandlerChangeVoice 处理更换语音的MCP函数调用
func (h *ConnectionHandler) mcpHandlerChangeVoice(ctx context.Context, args interface{}) {
	if voice, ok := args.(string); ok {
		log.Infof(ctx, "mcp_handler_change_voice: %s", voice)
		if err := h.providers.tts.SetVoice(ctx, voice); err != nil {
			log.Errorf(ctx, "mcp_handler_change_voice: SetVoice failed: %v", err)
			h.SystemSpeak(ctx, "切换语音失败，没有叫"+voice+"的音色")
		} else {
			h.SystemSpeak(ctx, "已切换到音色"+voice)
		}
	} else {
		log.Error(ctx, "mcp_handler_change_voice: args is not a string")
	}
}

// mcpHandlerChangeRole 处理更换AI角色的MCP函数调用
func (h *ConnectionHandler) mcpHandlerChangeRole(ctx context.Context, args interface{}) {
	if params, ok := args.(map[string]string); ok {
		role := params["role"]
		prompt := params["prompt"]

		log.Infof(ctx, "mcp_handler_change_role: %s", role)
		h.dialogueManager.SetSystemMessage(prompt)
		h.dialogueManager.KeepRecentMessages(5) // 保留最近5条消息

		// 更新当前AI角色
		h.currentAIRole = role

		// 更新数据库中的会话角色
		//go func() {
		//	err := h.conversationService.UpdateSessionRole(ctx, h.sessionID, role)
		//	if err != nil {
		//		utils.Error("更新会话角色失败: %v", err)
		//	}
		//}()

		if getter, ok := h.providers.tts.(configGetter); ok {
			ttsProvider := getter.Config().Type
			if ttsProvider == "edge" {
				if role == "陕西女友" {
					h.providers.tts.SetVoice(ctx, "zh-CN-shaanxi-XiaoniNeural") // 陕西女友音色
				} else if role == "英语老师" {
					h.providers.tts.SetVoice(ctx, "zh-CN-XiaoyiNeural") // 英语老师音色
				} else if role == "好奇小男孩" {
					h.providers.tts.SetVoice(ctx, "zh-CN-YunxiNeural") // 好奇小男孩音色
				}
			}
		}
		h.SystemSpeak(ctx, "已切换到新角色 "+role)
	} else {
		log.Error(ctx, "mcp_handler_change_role: args is not a string")
	}
}

// mcpHandlerExit 处理退出对话的MCP函数调用
func (h *ConnectionHandler) mcpHandlerExit(ctx context.Context, args interface{}) {
	if text, ok := args.(string); ok {
		h.closeAfterChat = true
		h.SystemSpeak(ctx, text)
	} else {
		log.Error(ctx, "mcp_handler_exit: args is not a string")
	}
}

// mcpHandlerTakePhoto 处理拍照的MCP函数调用
func (h *ConnectionHandler) mcpHandlerTakePhoto(ctx context.Context, args interface{}) {
	// 特殊处理拍照函数，解析为VisionResponse
	resultStr, _ := args.(string)
	var visionResponse vision.VisionResponse
	if err := json.Unmarshal([]byte(resultStr), &visionResponse); err != nil {
		log.Errorf(ctx, "解析VisionResponse失败: %v", err)
	}

	if !visionResponse.Success {
		log.Errorf(ctx, "拍照失败: %s", visionResponse.Message)
		h.genResponseByLLM(context.Background(), h.dialogueManager.GetLLMDialogue(), h.talkRound)

	}

	h.SystemSpeak(ctx, visionResponse.Result)
}
