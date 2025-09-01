package utils

import (
	"combot-server-go/src/core/utils"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// GetCtxWithReq 为传入的ctx生成一个唯一的reqId，并返回包含该reqId的新context
func GetCtxWithReq(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	} else {
		if v := ctx.Value(utils.RequestIDKey); v != nil {
			if _, ok := v.(string); ok {
				// 已经存在reqId，直接返回原ctx
				return ctx
			}
		}
	}
	// 生成唯一的请求ID
	uuid, _ := uuid.NewUUID()
	reqId := fmt.Sprintf("req-generate-%v", strings.Replace(uuid.String(), "-", "", -1))
	return context.WithValue(ctx, "reqId", reqId)
}
