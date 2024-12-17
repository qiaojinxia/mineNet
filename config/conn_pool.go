package network

import "time"

type ConnPoolConfig struct {
	MaxConns      int           `mapstructure:"max_conns"`       // 最大连接数
	MinIdleConns  int           `mapstructure:"min_idle_conns"`  // 最小空闲连接数
	MaxErrorCount int           `mapstructure:"max_error_count"` // 最大错误计数
	IdleTimeout   time.Duration `mapstructure:"idle_timeout"`    // 空闲连接超时时间
	MaxLifetime   time.Duration `mapstructure:"max_lifetime"`    // 最大连接生命周期
	MaxIdleTime   time.Duration `mapstructure:"max_idle_time"`   // 最大空闲时间
}

// DefaultConnPoolConfig 返回默认的连接池配置
func DefaultConnPoolConfig() *ConnPoolConfig {
	return &ConnPoolConfig{
		MaxConns:      1000,              // 最大连接数
		MinIdleConns:  10,                // 最小空闲连接数
		MaxErrorCount: 3,                 // 最大错误次数
		IdleTimeout:   time.Second * 300, // 空闲超时时间(5分钟)
		MaxLifetime:   time.Hour * 6,     // 连接最大生命周期(6小时)
		MaxIdleTime:   time.Minute * 30,  // 最大空闲时间(30分钟)
	}
}
