package middleware

import (
	"combot-server-go/src/configs/database"
	"combot-server-go/src/core/codes"
	"combot-server-go/src/core/response"
	"combot-server-go/src/models"
	"combot-server-go/src/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTAuthMiddleware JWT身份验证中间件
func JWTAuthMiddleware(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 打印请求 dump 信息
		utils.DumpRequest(c.Request.Context(), c.Request)
		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Failed(c, codes.CodeTokenMissing, nil)
			c.Abort()
			return
		}

		// 检查Bearer前缀
		if !strings.HasPrefix(authHeader, "Bearer ") {
			response.Failed(c, codes.CodeInvalidToken, nil)
			c.Abort()
			return
		}

		// 提取token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// 解析token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// 验证签名方法
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secretKey), nil
		})

		if err != nil || !token.Valid {
			response.Failed(c, codes.CodeInvalidToken, nil)
			c.Abort()
			return
		}

		// 获取claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			response.Failed(c, codes.CodeJWTParseError, nil)
			c.Abort()
			return
		}

		// 获取用户ID
		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			response.Failed(c, codes.CodeJWTParseError, nil)
			c.Abort()
			return
		}

		userID := int64(userIDFloat)

		// 验证用户是否存在
		var user models.User
		if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
			response.Failed(c, codes.CodeUserNotFound, nil)
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", userID)
		c.Set("user", &user)

		c.Next()
	}
}
