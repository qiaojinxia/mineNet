package network

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`     // Redis 地址
	Password string `mapstructure:"password"` // Redis 密码
	DB       int    `mapstructure:"db"`       // Redis 数据库
}
