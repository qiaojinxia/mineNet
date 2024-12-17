package common

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"
)

// GenerateConnectionId 生成全局唯一的连接ID
func GenerateConnectionId() string {
	// 使用时间戳作为前缀
	timestamp := time.Now().UnixNano()

	// 生成8字节随机数
	random := make([]byte, 8)
	rand.Read(random)

	// 组合格式: timestamp-randomhex
	return fmt.Sprintf("%d-%s", timestamp, hex.EncodeToString(random))
}
