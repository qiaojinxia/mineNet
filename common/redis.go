package common

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	mconfig "mineNet/config"
	"sync"
)
import "github.com/redis/go-redis/v9"

var (
	redisClient *redis.Client
	once        sync.Once
)

type RedisServer struct {
	*redis.Client
}

// InitRedis 初始化 Redis 客户端
func InitRedis(cfg *mconfig.RedisConfig) (*RedisServer, error) {
	// 初始化 Redis 客户端
	once.Do(func() {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		})
	})
	return &RedisServer{redisClient}, nil
}

// GetRedisClient 获取 Redis 单例客户端
func GetRedisClient() *redis.Client {
	if redisClient == nil {
		GetLogger().Error("redis client is nil")
		return nil
	}
	return redisClient
}

// Start 关闭 Redis 客户端
func (rc *RedisServer) Start() error {
	if redisClient == nil {
		GetLogger().Error("Redis client is nil")
		return fmt.Errorf("redis client is nil")
	}

	// Ping 测试连接
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		GetLogger().Error("Redis connection failed",
			zap.Error(err),
			zap.String("addr", redisClient.Options().Addr))
		return err
	}

	GetLogger().Info("Redis connected successfully",
		zap.String("addr", redisClient.Options().Addr),
		zap.String("db", fmt.Sprintf("%d", redisClient.Options().DB)))

	return nil
}

// Stop 关闭 Redis 客户端
func (rc *RedisServer) Stop() error {
	if redisClient != nil {
		GetLogger().Info("Closing Redis connection",
			zap.String("addr", redisClient.Options().Addr))

		if err := redisClient.Close(); err != nil {
			GetLogger().Error("Failed to close Redis connection",
				zap.Error(err),
				zap.String("addr", redisClient.Options().Addr))
			return err
		}

		GetLogger().Info("Redis connection closed successfully")
	}
	return nil
}
