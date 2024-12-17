package main

import (
	"context"
	"fmt"
	"mineNet/core"
	"mineNet/network"
	plogin "mineNet/proto/login"
)

func InitApp(server *network.TCPServer) {
	router := core.NewRouter()
	err := router.RegisterHandler(&plogin.LoginRequest{},
		func(ctx context.Context, msg *plogin.LoginRequest) (*plogin.LoginResponse, error) {
			fmt.Printf("收到的消息 用户名:%s mima:%s\n", msg.Username, msg.Password)
			return &plogin.LoginResponse{
				Success:      true,
				Token:        "123456",
				ErrorMessage: "",
			}, nil
		})
	if err != nil {
		panic(err)
	}
	server.SetHandler(network.CmdRequest, router)

}
