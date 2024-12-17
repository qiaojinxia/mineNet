package common

import (
	"context"
	mconfig "mineNet/config"
	"sync"
)
import "github.com/redis/go-redis/v9"

var (
	redisClient *redis.Client
	once        sync.Once
)

// InitRedis 初始化 Redis 客户端
func InitRedis(cfg *mconfig.RedisConfig) (*redis.Client, error) {
	// 初始化 Redis 客户端
	var err error
	once.Do(func() {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		})
	})

	// Ping 测试连接
	err = redisClient.Ping(context.Background()).Err()
	return redisClient, err
}

// GetRedisClient 获取 Redis 单例客户端
func GetRedisClient() *redis.Client {
	if redisClient == nil {
		GetLogger().Error("redis client is nil")
		return nil
	}
	return redisClient
}

// CloseRedisClient 关闭 Redis 客户端
func CloseRedisClient() error {
	if redisClient != nil {
		return redisClient.Close()
	}
	return nil
}
