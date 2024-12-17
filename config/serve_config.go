package network

type ServerConfig struct {
	AppName        string `mapstructure:"app_name"`
	Addr           string `mapstructure:"addr"`           // 服务器地址
	MaxConnections int    `mapstructure:"maxConnections"` // 最大连接数
}
