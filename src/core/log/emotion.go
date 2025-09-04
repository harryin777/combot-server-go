package log

import "regexp"

// EmotionEmoji 定义情绪到表情的映射
var EmotionEmoji = map[string]string{
	"neutral":     "😐",
	"happy":       "😊",
	"laughing":    "😂",
	"funny":       "🤡",
	"sad":         "😢",
	"angry":       "😠",
	"crying":      "😭",
	"loving":      "🥰",
	"embarrassed": "😳",
	"surprised":   "😮",
	"shocked":     "😱",
	"thinking":    "🤔",
	"winking":     "😉",
	"cool":        "😎",
	"relaxed":     "😌",
	"delicious":   "😋",
	"kissy":       "😘",
	"confident":   "😏",
	"sleepy":      "😴",
	"silly":       "🤪",
	"confused":    "😕",
}

// GetEmotionEmoji 根据情绪返回对应的表情
func GetEmotionEmoji(emotion string) string {
	if emoji, ok := EmotionEmoji[emotion]; ok {
		return emoji
	}
	return EmotionEmoji["neutral"] // 默认返回中性表情
}

var ( // 简化版表情符号正则表达式
	SimpleEmojiRegex = regexp.MustCompile(`[\x{1F000}-\x{1FFFF}]|` +
		`[\x{2600}-\x{26FF}]|` + // 杂项符号
		`[\x{2700}-\x{27BF}]`) // 装饰符号
)

func RemoveAllEmoji(text string) string {
	return SimpleEmojiRegex.ReplaceAllString(text, "")
}
