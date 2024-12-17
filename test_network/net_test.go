package test_network

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"mineNet/network"
	msgtype "mineNet/proto"
	mproto "mineNet/proto/common"
	"mineNet/proto/login"
	"net"
	"testing"
)

func TestNetWork(t *testing.T) {
	data := []byte("hello world")
	d := &login.LoginRequest{
		Username: "zhangsan",
		Password: "zhangsan123",
	}
	data1, _ := proto.Marshal(d)
	xx := &mproto.BaseMessage{
		RequestId: 123123,
		MsgType:   msgtype.GetMessageType(d),
		Payload:   data1,
	}
	data1, _ = proto.Marshal(xx)
	// Create a heart packet
	hp, _ := network.NewPacket(network.CmdRequest, data1)
	// AES-128 密钥 (16字节)
	key := []byte("1234567890123456")
	// 初始化向量 (16字节)
	iv := []byte("1234567890123456")
	// Create packet codec
	codec := network.NewPacketCodec(true, true, key, iv) // No compression, no encryption

	// Encode packet
	data, err := codec.Encode(hp)
	if err != nil {
		panic(err)
	}

	// Connect to server
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Send packet
	_, err = conn.Write(data)
	if err != nil {
		panic(err)
	}

	fmt.Println("Heart packet sent successfully")
}
