package core

import (
	"context"
	"fmt"
	"google.golang.org/protobuf/proto"
	msgtype "mineNet/proto"
	mproto "mineNet/proto/common"
	"reflect"
	"sync"
)

import "mineNet/network"

type Handler interface{}

type Router struct {
	handlers     map[reflect.Type]Handler // 请求处理函数
	typeRegistry map[string]reflect.Type  // 消息类型名到reflect.Type的映射
	mu           sync.RWMutex
}

func NewRouter() *Router {
	return &Router{
		handlers:     make(map[reflect.Type]Handler),
		typeRegistry: make(map[string]reflect.Type),
	}
}

// RegisterMessageType 注册消息类型
func (r *Router) RegisterMessageType(messages ...proto.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, msg := range messages {
		msgType := reflect.TypeOf(msg)
		if msgType.Kind() == reflect.Ptr {
			msgType = msgType.Elem()
		}
		typeName := string(msg.ProtoReflect().Descriptor().FullName())
		r.typeRegistry[typeName] = msgType
	}
}

// RegisterHandler 注册消息处理函数
func (r *Router) RegisterHandler(msg proto.Message, handler Handler) error {
	// 自动注册消息类型
	r.RegisterMessageType(msg)

	msgType := reflect.TypeOf(msg)
	if msgType.Kind() == reflect.Ptr {
		msgType = msgType.Elem()
	}

	if err := r.validateHandler(msgType, handler); err != nil {
		return fmt.Errorf("invalid handler: %v", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[msgType] = handler

	return nil
}

// Handle 处理收到的消息
func (r *Router) Handle(ctx context.Context, conn network.IConnection, packet network.IPacket) error {
	baseMsg := &mproto.BaseMessage{}
	if err := proto.Unmarshal(packet.Data(), baseMsg); err != nil {
		return fmt.Errorf("unmarshal base message failed: %v", err)
	}

	msgInfo := msgtype.GetMessageInfo(baseMsg.MsgType)

	if msgInfo == nil {
		return fmt.Errorf("unknown message type: %s", msgtype.GetMessageInfo(baseMsg.MsgType).Name)
	}

	concreteMsg := reflect.New(msgInfo.RequestType).Interface().(proto.Message)
	if err := proto.Unmarshal(baseMsg.Payload, concreteMsg); err != nil {
		return fmt.Errorf("unmarshal concrete message failed: %v", err)
	}

	r.mu.RLock()
	var handler Handler
	var ok bool
	handler, ok = r.handlers[msgInfo.RequestType]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler registered for message type: %v", msgInfo)
	}

	return r.callHandler(ctx, baseMsg.RequestId, handler, conn, concreteMsg, packet)
}

// callHandler 调用处理函数并处理返回值
func (r *Router) callHandler(ctx context.Context, requestId uint64, handler Handler,
	conn network.IConnection, msg proto.Message, packet network.IPacket) error {
	if handler == nil {
		return fmt.Errorf("handler is nil")
	}

	handlerValue := reflect.ValueOf(handler)
	handlerType := handlerValue.Type()

	// 预先检查参数类型匹配
	if handlerType.NumIn() != 2 {
		return fmt.Errorf("handler must have exactly 2 parameters")
	}

	// 创建参数切片并检查类型匹配
	args := make([]reflect.Value, 2)

	// 检查并设置 context 参数
	if !reflect.TypeOf(ctx).Implements(handlerType.In(0)) {
		return fmt.Errorf("context type mismatch: expected %v, got %v", handlerType.In(0), reflect.TypeOf(ctx))
	}
	args[0] = reflect.ValueOf(ctx)

	// 检查并设置消息参数
	msgType := reflect.TypeOf(msg)
	if !msgType.AssignableTo(handlerType.In(1)) {
		return fmt.Errorf("message type mismatch: expected %v, got %v", handlerType.In(1), msgType)
	}
	args[1] = reflect.ValueOf(msg)

	// 使用 recover 来处理可能的 panic
	var values []reflect.Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("handler panic: %v", r)
				values = []reflect.Value{reflect.Zero(handlerType.Out(0))}
				if handlerType.NumOut() == 2 {
					values = append(values, reflect.ValueOf(&err).Elem())
				}
			}
		}()
		values = handlerValue.Call(args)
	}()

	// 如果是由于 panic 导致的返回，直接处理错误
	if values == nil {
		return fmt.Errorf("handler execution failed")
	}

	// 根据返回值数量处理结果
	switch len(values) {
	case 1: // 只返回 error
		if values[0].IsNil() {
			return nil
		}
		return values[0].Interface().(error)

	case 2: // 返回 response 和 error
		// 先检查错误
		if !values[1].IsNil() {
			return values[1].Interface().(error)
		}

		// 处理响应消息
		if !values[0].IsNil() {
			resp := values[0].Interface().(proto.Message)
			// 只有请求消息才需要发送响应
			if packet.CmdId() == network.CmdRequest {
				if err := r.sendResponse(conn, requestId, resp); err != nil {
					return fmt.Errorf("failed to send response: %w", err)
				}
			}
		}
		return nil

	default:
		return fmt.Errorf("invalid number of return values: expected 1 or 2, got %d", len(values))
	}
}

// sendResponse 发送响应消息
func (r *Router) sendResponse(conn network.IConnection,
	requestId uint64, resp proto.Message) error {
	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	// 序列化响应消息
	respData, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// 封装基础消息
	baseMsg := &mproto.BaseMessage{
		MsgType:   msgtype.GetMessageType(resp),
		Payload:   respData,
		RequestId: requestId,
	}

	baseData, err := proto.Marshal(baseMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal base message: %w", err)
	}

	// 创建响应包
	respPacket, err := network.NewPacket(network.CmdReply, baseData)
	if err != nil {
		return fmt.Errorf("failed to create response packet: %w", err)
	}

	// 发送响应
	if err := conn.Send(respPacket); err != nil {
		return fmt.Errorf("failed to send packet: %w", err)
	}

	return nil
}

// getMsgType 根据消息类型名获取反射类型
func (r *Router) getMsgType(msgTypeName string) reflect.Type {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.typeRegistry[msgTypeName]
}

// GetRegisteredTypes 获取所有已注册的消息类型
func (r *Router) GetRegisteredTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.typeRegistry))
	for typeName := range r.typeRegistry {
		types = append(types, typeName)
	}
	return types
}

// validateHandler 验证处理函数签名
func (r *Router) validateHandler(msgType reflect.Type, handler Handler) error {
	handlerType := reflect.TypeOf(handler)
	if handlerType.Kind() != reflect.Func {
		return fmt.Errorf("handler must be a function")
	}

	// 检查输入参数
	if handlerType.NumIn() != 2 {
		return fmt.Errorf("handler must have exactly 2 parameters")
	}

	if handlerType.In(0) != reflect.TypeOf((*context.Context)(nil)).Elem() {
		return fmt.Errorf("first parameter must be context.Context")
	}

	if handlerType.In(1).Elem() != msgType {
		return fmt.Errorf("second parameter type doesn't match the message type")
	}

	// 检查返回值
	numOut := handlerType.NumOut()
	if numOut != 1 && numOut != 2 {
		return fmt.Errorf("handler must return (error) or (proto.Message, error)")
	}

	if numOut == 2 {
		if !handlerType.Out(0).Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
			return fmt.Errorf("first return value must implement proto.Message")
		}
	}

	if !handlerType.Out(numOut - 1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return fmt.Errorf("last return value must be error")
	}

	return nil
}

// HasHandler 检查是否存在指定消息类型的处理函数
func (r *Router) HasHandler(msg proto.Message) bool {
	msgType := reflect.TypeOf(msg)
	if msgType.Kind() == reflect.Ptr {
		msgType = msgType.Elem()
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.handlers[msgType]
	return ok
}
