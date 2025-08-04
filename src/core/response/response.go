package response

import (
	"net/http"
	"xiaozhi-server-go/src/core/codes"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    codes.CodeSuccess,
		Message: codes.GetMessage(codes.CodeSuccess),
		Data:    data,
	})
}

// SuccessWithMessage 成功响应（自定义消息）
func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    codes.CodeSuccess,
		Message: message,
		Data:    data,
	})
}

func Failed(c *gin.Context, code int, err error) {
	var errMsg string
	if code == http.StatusBadRequest {
		errMsg = err.Error()
	} else {
		errMsg = codes.GetMessage(code)
	}
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: errMsg,
	})
}

// getHTTPStatus 根据业务错误码映射到HTTP状态码
func getHTTPStatus(code int) int {
	switch {
	case code == codes.CodeSuccess:
		return http.StatusOK
	case code >= 1000 && code < 2000:
		// 通用错误
		switch code {
		case codes.CodeInvalidRequest:
			return http.StatusBadRequest
		case codes.CodeUnauthorized:
			return http.StatusUnauthorized
		case codes.CodeForbidden:
			return http.StatusForbidden
		case codes.CodeNotFound:
			return http.StatusNotFound
		default:
			return http.StatusInternalServerError
		}
	case code >= 2000 && code < 3000:
		// 用户相关错误
		switch code {
		case codes.CodeUserNotFound:
			return http.StatusNotFound
		case codes.CodeInvalidUsernamePassword, codes.CodeInvalidOldPassword:
			return http.StatusUnauthorized
		case codes.CodeUserAlreadyExists, codes.CodeUsernameExists, codes.CodePhoneExists:
			return http.StatusConflict
		default:
			return http.StatusBadRequest
		}
	case code >= 3000 && code < 4000:
		// 认证相关错误
		return http.StatusUnauthorized
	case code >= 4000 && code < 5000:
		// 数据库相关错误
		switch code {
		case codes.CodeDataNotFound:
			return http.StatusNotFound
		case codes.CodeDuplicateKey:
			return http.StatusConflict
		default:
			return http.StatusInternalServerError
		}
	case code >= 5000 && code < 6000:
		// 业务逻辑错误
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
