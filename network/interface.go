package network

import (
	"context"
	"mineNet/common"
	"time"
)

// IConnection 连接接口定义
type IConnection interface {
	IsAlive() bool
	ID() string                                     // 连接唯一标识
	Send(packet IPacket) error                      // 发送消息
	Close() error                                   // 关闭连接
	GetProperty(key string) any                     // 获取连接属性
	SetProperty(key string, value any)              // 设置连接属性
	LastActive() time.Time                          // 最后活跃时间
	CreatedAt() time.Time                           // 创建时间
	ErrorCount() int                                // 错误次数
	IsClosed() bool                                 // 连接是否已关闭
	RemoteAddr() string                             // 远程地址
	SetHandler(cmdId uint32, handler PacketHandler) // 设置消息处理器

}

// IPacket 消息包接口
type IPacket interface {
	SeqId() uint64     //消息序列号
	CmdId() uint32     // 消息ID
	Data() []byte      // 消息内容
	SetSeqId(i uint64) //设置消息序号
	SetData([]byte)    // 设置消息内容
}

// PacketHandler 消息处理器接口
type PacketHandler interface {
	Handle(ctx context.Context, conn IConnection, packet IPacket) error
}

// IServer 服务器接口
type IServer interface {
	common.IComponent
	Broadcast(packet IPacket)                       // 广播消息
	GetConnections() []IConnection                  // 获取所有连接
	OnConnect(fn func(IConnection))                 // 连接建立回调
	OnDisconnect(fn func(IConnection))              // 连接断开回调
	SetHandler(cmdId uint32, handler PacketHandler) // 设置消息处理器
}
