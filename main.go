package main

import (
	"log"
	"mineNet/common"
	mconfig "mineNet/config"
	"mineNet/metrics"
	"mineNet/network"
)

func main() {
	err := mconfig.LoadConfig("config.yaml")
	if err != nil {
		panic(err)
	}
	// 启动 metrics 服务器
	metricsServer := metrics.NewMetricsServer(":9090")
	go func() {
		if err := metricsServer.Start(); err != nil {
			log.Fatalf("Failed to start metrics server: %v", err)
		}
	}()
	config := &common.Config{
		LogPath:     "logs/app.log",
		LogLevel:    "debug",
		MaxSize:     100,
		MaxBackups:  30,
		MaxAge:      7,
		Compress:    true,
		ShowConsole: true,
	}
	common.MustInitLogger(config)
	cfg := mconfig.GetConfig()
	_, err = common.InitRedis(&cfg.Redis)
	if err != nil {
		panic(err)
	}
	manager := common.NewComponentManager()
	server, err := network.InitTcpServer(cfg)
	if err != nil {
		panic(err)
	}
	InitApp(server)
	manager.Register(server)
	err = manager.Start()
	if err != nil {
		panic(err)
	}
	manager.Wait()

}
