package network

import "time"

type ConnConfig struct {
	CompressEnabled bool   `mapstructure:"compress_enabled"` // 是否启用压缩
	EncryptEnabled  bool   `mapstructure:"encrypt_enabled"`  // 是否启用加密
	EncryptKey      string `mapstructure:"encrypt_key"`      // 加密密钥
	EncryptIV       string `mapstructure:"encrypt_iv"`       // 加密初始化向量

	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"` // 心跳间隔
	HeartbeatTimeout  time.Duration `mapstructure:"heartbeat_timeout"`  // 心跳超时时间
	RetryCount        int           `mapstructure:"retry_count"`        // 重试次数

	RPS       float64 `mapstructure:"rps"`        // 每秒请求数
	BurstSize int     `mapstructure:"burst_size"` // 突发请求大小

	ReadTimeout  time.Duration `mapstructure:"read_timeout"`  // 读取超时时间
	WriteTimeout time.Duration `mapstructure:"write_timeout"` // 写入超时时间

	MaxMessageSize int `mapstructure:"max_message_size"` // 最大消息大小
	SendQueueSize  int `mapstructure:"send_queue_size"`  // 发送队列大小
	RecvQueueSize  int `mapstructure:"recv_queue_size"`  // 接收队列大小
}

func DefaultConfig() *ConnConfig {
	return &ConnConfig{
		CompressEnabled:   true,
		EncryptEnabled:    true,
		EncryptKey:        "1234567890123456",
		EncryptIV:         "1234567890123456",
		HeartbeatInterval: time.Second * 30,
		HeartbeatTimeout:  time.Second * 90,
		RetryCount:        3,
		RPS:               1000,
		BurstSize:         100,
		ReadTimeout:       time.Second * 30,
		WriteTimeout:      time.Second * 30,
		MaxMessageSize:    1024 * 1024, // 1MB
		SendQueueSize:     1000,
		RecvQueueSize:     1000,
	}
}
