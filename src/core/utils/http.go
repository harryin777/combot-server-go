package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DumpRequest 将 HTTP 请求格式化为人类可读的字符串并记录
func DumpRequest(ctx context.Context, req *http.Request) {
	var buf bytes.Buffer

	// 1. 请求基本信息
	buf.WriteString("=== HTTP Request ===\n")
	fmt.Fprintf(&buf, "Method: %s\n", req.Method)
	fmt.Fprintf(&buf, "URL: %s\n", req.URL.String())
	fmt.Fprintf(&buf, "Protocol: %s\n", req.Proto)
	buf.WriteString("\n")

	// 2. 主机和请求头
	buf.WriteString("--- Headers ---\n")
	fmt.Fprintf(&buf, "Host: %s\n", req.Host)
	for key, values := range req.Header {
		fmt.Fprintf(&buf, "%s: %s\n", key, strings.Join(values, ", "))
	}
	buf.WriteString("\n")

	// 3. 查询参数（可选，增加可读性）
	if len(req.URL.Query()) > 0 {
		buf.WriteString("--- Query Parameters ---\n")
		for key, values := range req.URL.Query() {
			fmt.Fprintf(&buf, "%s: %s\n", key, strings.Join(values, ", "))
		}
		buf.WriteString("\n")
	}

	// 4. 请求正文
	if req.Body != nil {
		buf.WriteString("--- Body ---\n")
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			fmt.Fprintf(&buf, "Error reading body: %v\n", err)
		} else if len(bodyBytes) > 0 {
			// 尝试格式化 JSON（如果适用）
			var formattedBody bytes.Buffer
			if jsonErr := json.Indent(&formattedBody, bodyBytes, "", "  "); jsonErr == nil {
				buf.WriteString(formattedBody.String())
			} else {
				buf.Write(bodyBytes)
			}
			buf.WriteString("\n")
		} else {
			buf.WriteString("(Empty)\n")
		}
		// 恢复请求正文
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	} else {
		buf.WriteString("--- Body ---\n(Empty)\n")
	}

	buf.WriteString("================\n")

	// 输出到日志
	Infof(ctx, buf.String())
}
