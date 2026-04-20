/*
 * @Author: chengjiang
 * @Date: 2025-12-09
 * @Description: AI Agent 工厂模式实现
 */
package ai

import (
	"context"
	"fmt"
	"sync"
)

// AdapterCreator 适配器创建函数类型
type AdapterCreator func(ctx context.Context, config *AgentConfig, subAgents []Agent) (Agent, error)

// AgentFactory Agent 工厂接口
type AgentFactory interface {
	// RegisterAdapter 注册适配器创建函数
	RegisterAdapter(adapterType string, creator AdapterCreator)

	// Create 根据配置创建 Agent
	Create(ctx context.Context, config AgentConfig) (Agent, error)

	// CreateWithSubAgents 创建带子 Agent 的主 Agent
	CreateWithSubAgents(ctx context.Context, masterConfig AgentConfig, subConfigs []AgentConfig) (Agent, error)

}

// DefaultFactory 默认工厂实现
type DefaultFactory struct {
	adapters map[AdapterType]AdapterCreator
	mu       sync.RWMutex
}

// NewFactory 创建工厂
func NewFactory() *DefaultFactory {
	return &DefaultFactory{	
		adapters: make(map[AdapterType]AdapterCreator),
	}
}

// RegisterAdapter 注册适配器创建函数
func (f *DefaultFactory) RegisterAdapter(adapterType AdapterType, creator AdapterCreator) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.adapters[adapterType] = creator
}

// Create 根据配置创建 Agent（不使用缓存）
func (f *DefaultFactory) Create(ctx context.Context, config *AgentConfig) (Agent, error) {
	return f.createWithSubAgents(ctx, config, nil)
}

// createWithSubAgents 内部方法：创建带子 Agent 的 Agent
func (f *DefaultFactory) createWithSubAgents(ctx context.Context, config *AgentConfig, subAgents []Agent) (Agent, error) {
	f.mu.RLock()
	creator, ok := f.adapters[config.AdapterType]
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown adapter type: %s", config.AdapterType)
	}

	return creator(ctx, config, subAgents)
}

// CreateWithSubAgents 创建带子 Agent 的主 Agent
func (f *DefaultFactory) CreateWithSubAgents(ctx context.Context, masterConfig *AgentConfig, subConfigs []*AgentConfig) (Agent, error) {
	var subAgents []Agent
	for _, sc := range subConfigs {
		sub, err := f.Create(ctx, sc)
		if err != nil {
			return nil, fmt.Errorf("failed to create sub-agent '%s': %w", sc.Name, err)
		}
		subAgents = append(subAgents, sub)
	}
	return f.createWithSubAgents(ctx, masterConfig, subAgents)
}