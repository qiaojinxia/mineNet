package common

import (
	"github.com/hashicorp/golang-lru"
	"sync"
)

type LruCacheManager struct {
	caches map[string]*lru.Cache
	mu     sync.RWMutex
}

// LruCacheManager 实例
var lruManager = &LruCacheManager{
	caches: make(map[string]*lru.Cache),
}

// RegisterLru 注册一个新的 LRU 缓存
func RegisterLru(moduleName string, size int) error {
	lruManager.mu.Lock()
	defer lruManager.mu.Unlock()

	if _, exists := lruManager.caches[moduleName]; exists {
		return nil // 如果模块已经注册，直接返回
	}

	cache, err := lru.New(size)
	if err != nil {
		return err
	}

	lruManager.caches[moduleName] = cache
	return nil
}

// GetLru 获取指定模块的 LRU 缓存
func GetLru(moduleName string) (*lru.Cache, bool) {
	lruManager.mu.RLock()
	defer lruManager.mu.RUnlock()

	cache, exists := lruManager.caches[moduleName]
	return cache, exists
}

// SetValue 向指定模块的 LRU 缓存中设置值
func SetValue(moduleName string, key interface{}, value interface{}) error {
	cache, exists := GetLru(moduleName)
	if !exists {
		return nil // 或者返回一个错误，表示模块未注册
	}

	cache.Add(key, value)
	return nil
}

// GetValue 从指定模块的 LRU 缓存中获取值
func GetValue(moduleName string, key interface{}) (interface{}, bool) {
	cache, exists := GetLru(moduleName)
	if !exists {
		return nil, false // 模块未注册
	}

	return cache.Get(key)
}
