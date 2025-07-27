package user

import (
	"fmt"
	"net/http"

	"xiaozhi-server-go/src/core/utils"

	"github.com/gin-gonic/gin"
)

// 这是一个示例，展示如何在处理函数中使用新的日志系统
func (us *UserService) LoginWithNewLogger(c *gin.Context) {
	// 从gin的context中获取请求上下文
	ctx := c.Request.Context()

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用新的日志API，会自动包含request ID
		utils.Errorf(ctx, "请求参数错误: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 记录登录尝试
	utils.WithField(ctx, "username", req.Username).Info("用户尝试登录")

	// 业务逻辑...
	utils.Infof(ctx, "用户 %s 登录成功", req.Username)

	// 返回响应...
}

// 展示如何使用WithFields添加多个字段
func (us *UserService) ExampleWithMultipleFields(c *gin.Context) {
	ctx := c.Request.Context()

	// 添加多个字段
	utils.WithFields(ctx, map[string]interface{}{
		"user_id": 123,
		"action":  "example_action",
		"source":  "web_api",
	}).Info("执行示例操作")

	// 也可以链式调用
	utils.WithField(ctx, "step", "validation").
		WithField("result", "success").
		Info("验证步骤完成")
}

// 展示错误日志的使用
func (us *UserService) ExampleErrorHandling(c *gin.Context) {
	ctx := c.Request.Context()

	// 模拟一个错误
	err := fmt.Errorf("数据库连接失败")

	// 使用WithError记录错误详情
	utils.WithError(ctx, err).Error("处理用户请求时发生错误")

	// 或者使用Errorf格式化错误消息
	utils.Errorf(ctx, "处理用户ID %d 的请求失败: %v", 123, err)
}
