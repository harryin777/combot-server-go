package middleware

import (
	"context"

	"combot-server-go/src/core/log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDMiddleware 为每个请求生成唯一的request ID并添加到context中
func RequestIDMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// 生成唯一的request ID
		reqID := uuid.New().String()

		// 添加到gin的context中
		c.Set(log.RequestIDKey, reqID)

		// 将request ID添加到标准context中，供日志使用
		ctx := context.WithValue(c.Request.Context(), log.RequestIDKey, reqID)
		c.Request = c.Request.WithContext(ctx)

		// 添加到响应头中，便于调试
		c.Header("X-Request-ID", reqID)

		c.Next()
	})
}
