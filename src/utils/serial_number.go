package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
)

// GenerateSerialNumber 根据 machineID 和 deviceID 生成确定性的 UUID 格式序列号
// 使用 SHA256 哈希确保相同输入总是生成相同的 UUID
// 这样即使在多 Pod 环境下,同一设备在不同 Pod 上也会生成相同的序列号
func GenerateSerialNumber(machineID, deviceID string) string {
	// 组合 machineID 和 deviceID
	input := fmt.Sprintf("%s:%s", machineID, deviceID)

	// 使用 SHA256 生成哈希
	hash := sha256.Sum256([]byte(input))

	// 取前16字节转换为 UUID 格式
	// UUID v5 使用 SHA-1, 我们这里用 SHA256 自定义实现
	hashHex := hex.EncodeToString(hash[:16])

	// 格式化为标准 UUID 格式: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	serialNumber := fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		hashHex[0:8],
		hashHex[8:12],
		hashHex[12:16],
		hashHex[16:20],
		hashHex[20:32],
	)

	return serialNumber
}

// GenerateRandomSerialNumber 生成随机的 UUID 格式序列号
// 用于需要完全随机序列号的场景
func GenerateRandomSerialNumber() string {
	return uuid.New().String()
}
