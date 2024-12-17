package common

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type IComponent interface {
	Start() error // 启动服务
	Stop() error  // 关闭服务
}

type ComponentManager struct {
	components []IComponent
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex
}

func NewComponentManager() *ComponentManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ComponentManager{
		components: make([]IComponent, 0),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Register 注册组件
func (m *ComponentManager) Register(c IComponent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.components = append(m.components, c)
}

// Start 启动所有组件
func (m *ComponentManager) Start() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 启动所有组件
	for _, c := range m.components {
		m.wg.Add(1)
		go func(comp IComponent) {
			defer m.wg.Done()
			if err := comp.Start(); err != nil {
				GetLogger().Error("Component start failed",
					zap.Error(err))
			}
		}(c)
	}

	// 处理信号
	go m.handleSignals()

	return nil
}

// Stop 停止所有组件
func (m *ComponentManager) Stop() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 触发关闭信号
	m.cancel()

	// 创建用于优雅关闭的context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建错误通道
	errChan := make(chan error, len(m.components))

	// 反向停止所有组件
	for i := len(m.components) - 1; i >= 0; i-- {
		c := m.components[i]
		go func(comp IComponent) {
			errChan <- comp.Stop()
		}(c)
	}

	// 收集停止错误
	var errs []error
	for i := 0; i < len(m.components); i++ {
		select {
		case err := <-errChan:
			if err != nil {
				errs = append(errs, err)
			}
		case <-ctx.Done():
			errs = append(errs, fmt.Errorf("shutdown timeout"))
			return fmt.Errorf("shutdown errors: %v", errs)
		}
	}

	// 等待所有组件完全停止
	m.wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// handleSignals 处理系统信号
func (m *ComponentManager) handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case sig := <-sigChan:
		fmt.Printf("Received signal: %v\n", sig)
		if err := m.Stop(); err != nil {
			fmt.Printf("Stop components error: %v\n", err)
		}
	case <-m.ctx.Done():
		return
	}
}

// Wait 等待所有组件结束
func (m *ComponentManager) Wait() {
	m.wg.Wait()
}
