package network

import (
	"github.com/spf13/viper"
	"sync"
)

var (
	config *Config
	once   sync.Once
	mu     sync.RWMutex
)

type Config struct {
	Conn     ConnConfig     `mapstructure:"conn"`
	ConnPool ConnPoolConfig `mapstructure:"conn_pool"`
	Server   ServerConfig   `mapstructure:"server"`
	Redis    RedisConfig    `mapstructure:"redis"`
}

// GetConfig 获取配置单例
func GetConfig() *Config {
	mu.RLock()
	defer mu.RUnlock()
	return config
}

// LoadConfig 加载配置(线程安全)
func LoadConfig(path string) error {
	var err error
	once.Do(func() {
		config, err = loadConfig(path)
	})
	return err
}

// ReloadConfig 重新加载配置
func ReloadConfig(path string) error {
	mu.Lock()
	defer mu.Unlock()
	var err error
	config, err = loadConfig(path)
	return err
}

func loadConfig(path string) (*Config, error) {
	v := viper.New()

	// 设置配置文件路径
	v.SetConfigFile(path)
	// 设置配置文件类型
	v.SetConfigType("yaml")

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	// 解析到结构体
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
