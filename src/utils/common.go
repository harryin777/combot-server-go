package utils

import (
	"combot-server-go/src/log"
	"context"
	"os"
	"time"

	"github.com/google/uuid"
)

// GetProjectDir 获取项目根目录
func GetProjectDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

// MinDuration 辅助函数：返回两个时间间隔中较小的一个
func MinDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// GenerateCtx 生成一个新的上下文，并附加一个唯一的请求ID
func GenerateCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, log.RequestIDKey, uuid.New().String())
}
