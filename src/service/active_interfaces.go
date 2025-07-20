package service

// ActiveService 定义active服务接口
type ActiveService interface {
	VerifyHMAC(challenge, hmacHex, hmacKey string) bool // 验证HMAC

	GenerateActivationCode() string // 生成激活码

	GenerateChallenge() string // 生成随机挑战码

	GenerateToken() string
}
