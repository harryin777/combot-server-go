package codes

// 错误码定义
const (
	// 成功
	CodeSuccess = 200

	// 通用错误 1000-1999
	CodeInvalidRequest = 1000 // 请求参数错误
	CodeInternalError  = 1001 // 内部服务器错误
	CodeUnauthorized   = 1002 // 未授权访问
	CodeForbidden      = 1003 // 禁止访问
	CodeNotFound       = 1004 // 资源不存在

	// 用户相关错误 2000-2999
	CodeUserNotFound            = 2000 // 用户不存在
	CodeUserAlreadyExists       = 2001 // 用户已存在
	CodeInvalidUsernamePassword = 2002 // 用户名或密码错误
	CodeUsernameExists          = 2003 // 用户名已存在
	CodePhoneExists             = 2004 // 手机号已存在
	CodeEmailExists             = 2005 // 邮箱已存在
	CodeInvalidOldPassword      = 2006 // 旧密码错误
	CodePasswordTooWeak         = 2007 // 密码强度不够
	CodeAccountDeleted          = 2008 // 账号已删除
	CodeSessionExpired          = 2009 // 会话已过期
	CodeDeviceNotFound          = 2010 // 设备不存在

	// 认证相关错误 3000-3999
	CodeInvalidToken     = 3000 // 无效的token
	CodeTokenExpired     = 3001 // token已过期
	CodeTokenMissing     = 3002 // 缺少token
	CodeInvalidSignature = 3003 // 无效的签名
	CodeJWTParseError    = 3004 // JWT解析错误

	// 数据库相关错误 4000-4999
	CodeDatabaseError    = 4000 // 数据库错误
	CodeDuplicateKey     = 4001 // 主键冲突
	CodeForeignKeyError  = 4002 // 外键约束错误
	CodeDataNotFound     = 4003 // 数据不存在
	CodeTransactionError = 4004 // 事务错误

	// 业务逻辑错误 5000-5999
	CodeBusinessLogicError = 5000 // 业务逻辑错误
	CodeInvalidOperation   = 5001 // 无效操作
	CodeResourceLocked     = 5002 // 资源被锁定
	CodeQuotaExceeded      = 5003 // 配额超限
	CodeDependencyError    = 5004 // 依赖错误
)

// 错误码对应的消息映射
var CodeMessages = map[int]string{
	// 成功
	CodeSuccess: "操作成功",

	// 通用错误
	CodeInvalidRequest: "请求参数错误",
	CodeInternalError:  "内部服务器错误",
	CodeUnauthorized:   "未授权访问",
	CodeForbidden:      "禁止访问",
	CodeNotFound:       "资源不存在",

	// 用户相关错误
	CodeUserNotFound:            "用户不存在",
	CodeUserAlreadyExists:       "用户已存在",
	CodeInvalidUsernamePassword: "用户名或密码错误",
	CodeUsernameExists:          "用户名已存在",
	CodePhoneExists:             "手机号已存在",
	CodeEmailExists:             "邮箱已存在",
	CodeInvalidOldPassword:      "旧密码错误",
	CodePasswordTooWeak:         "密码强度不够",
	CodeAccountDeleted:          "账号已删除",
	CodeSessionExpired:          "会话已过期",
	CodeDeviceNotFound:          "设备不存在",

	// 认证相关错误
	CodeInvalidToken:     "无效的token",
	CodeTokenExpired:     "token已过期",
	CodeTokenMissing:     "缺少token",
	CodeInvalidSignature: "无效的签名",
	CodeJWTParseError:    "JWT解析错误",

	// 数据库相关错误
	CodeDatabaseError:    "数据库错误",
	CodeDuplicateKey:     "主键冲突",
	CodeForeignKeyError:  "外键约束错误",
	CodeDataNotFound:     "数据不存在",
	CodeTransactionError: "事务错误",

	// 业务逻辑错误
	CodeBusinessLogicError: "业务逻辑错误",
	CodeInvalidOperation:   "无效操作",
	CodeResourceLocked:     "资源被锁定",
	CodeQuotaExceeded:      "配额超限",
	CodeDependencyError:    "依赖错误",
}

// GetMessage 根据错误码获取对应的消息
func GetMessage(code int) string {
	if message, exists := CodeMessages[code]; exists {
		return message
	}
	return "未知错误"
}

// IsSuccess 判断是否为成功状态码
func IsSuccess(code int) bool {
	return code == CodeSuccess
}
