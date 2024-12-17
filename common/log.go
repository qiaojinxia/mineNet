package common

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	defaultLogger atomic.Value
	initOnce      sync.Once
)

// InitLogger 初始化全局日志实例
func InitLogger(conf *Config) error {
	var err error
	initOnce.Do(func() {
		var logger *Logger
		logger, err = New(conf)
		if err != nil {
			return
		}
		defaultLogger.Store(logger)
	})
	return err
}

// GetLogger 获取全局日志实例
func GetLogger() *Logger {
	if v := defaultLogger.Load(); v != nil {
		return v.(*Logger)
	}
	// 如果未初始化，使用默认配置初始化
	_ = InitLogger(nil)
	return defaultLogger.Load().(*Logger)
}

// MustInitLogger 必须成功初始化日志实例，失败则panic
func MustInitLogger(conf *Config) {
	if err := InitLogger(conf); err != nil {
		panic(err)
	}
}

type Config struct {
	LogPath     string // 日志文件路径
	LogLevel    string // 日志级别 debug/info/warn/error
	MaxSize     int    // 单个日志文件最大尺寸，单位 MB
	MaxBackups  int    // 最大保留的日志文件数量
	MaxAge      int    // 日志文件保留天数
	Compress    bool   // 是否压缩历史日志
	ShowConsole bool   // 是否同时输出到控制台
}

type Logger struct {
	*zap.Logger
	config *Config
}

var defaultConfig = &Config{
	LogPath:     "logs/app.log",
	LogLevel:    "info",
	MaxSize:     100,
	MaxBackups:  30,
	MaxAge:      7,
	Compress:    true,
	ShowConsole: true,
}

// New 创建日志实例
func New(conf *Config) (*Logger, error) {
	if conf == nil {
		conf = defaultConfig
	}

	// 创建日志目录
	logDir := filepath.Dir(conf.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory failed: %v", err)
	}

	// 解析日志级别
	level, err := zapcore.ParseLevel(conf.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("parse log level failed: %v", err)
	}

	// 配置日志切割
	hook := &lumberjack.Logger{
		Filename:   conf.LogPath,
		MaxSize:    conf.MaxSize,
		MaxBackups: conf.MaxBackups,
		MaxAge:     conf.MaxAge,
		Compress:   conf.Compress,
	}

	// 设置日志编码器
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     timeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 创建核心
	var cores []zapcore.Core

	// 文件输出
	fileWriter := zapcore.AddSync(hook)
	cores = append(cores, zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		fileWriter,
		level,
	))

	// 控制台输出
	if conf.ShowConsole {
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		cores = append(cores, zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			level,
		))
	}

	// 创建Logger
	core := zapcore.NewTee(cores...)
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return &Logger{
		Logger: logger,
		config: conf,
	}, nil
}

// timeEncoder 自定义时间编码器
func timeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

// GetLevel 获取当前日志级别
func (l *Logger) GetLevel() string {
	return l.config.LogLevel
}

// SetLevel 动态设置日志级别
func (l *Logger) SetLevel(level string) error {
	l.config.LogLevel = level
	newLogger, err := New(l.config)
	if err != nil {
		return err
	}
	*l = *newLogger
	return nil
}

// Sync 同步缓存日志到磁盘
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}
