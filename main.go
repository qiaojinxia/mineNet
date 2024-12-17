package main

import (
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
	cfg := mconfig.GetConfig()
	schedulerServer := common.NewScheduler()
	err = common.InitLogger(&cfg.Log)
	if err != nil {
		panic(err)
	}
	redisServer, err := common.InitRedis(&cfg.Redis)
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
	manager.Register(metricsServer)
	manager.Register(schedulerServer)
	manager.Register(redisServer)
	err = manager.Start()
	if err != nil {
		panic(err)
	}
	manager.Wait()

}
