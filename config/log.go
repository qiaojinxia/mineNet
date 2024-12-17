package network

type LogConfig struct {
	LogPath     string `mapstructure:"log_path"`     // 日志文件路径
	LogLevel    string `mapstructure:"log_level"`    // 日志级别 debug/info/warn/error
	MaxSize     int    `mapstructure:"max_size"`     // 单个日志文件最大尺寸，单位 MB
	MaxBackups  int    `mapstructure:"max_backups"`  // 最大保留的日志文件数量
	MaxAge      int    `mapstructure:"max_age"`      // 日志文件保留天数
	Compress    bool   `mapstructure:"compress"`     // 是否压缩历史日志
	ShowConsole bool   `mapstructure:"show_console"` // 是否同时输出到控制台
}
