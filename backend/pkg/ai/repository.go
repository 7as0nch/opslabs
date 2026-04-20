/*
 * @Author: chengjiang
 * @Date: 2025-12-09
 * @Description: AI Agent 配置仓库接口定义
 */
package ai

import "context"

// ConfigRepository Agent 配置仓库接口
// 由 data 层实现，注入到 factory
type ConfigRepository interface {
	// GetByID 根据 ID 获取配置
	GetByID(ctx context.Context, id int64) (*AgentConfig, error)

	// GetByName 根据名称获取配置
	GetByName(ctx context.Context, name string) (*AgentConfig, error)

	// ListEnabled 获取所有启用的配置
	ListEnabled(ctx context.Context) ([]AgentConfig, error)

	// GetSubAgents 获取子 Agent 配置
	GetSubAgents(ctx context.Context, parentID int64) ([]AgentConfig, error)

	// Save 保存配置
	Save(ctx context.Context, config *AgentConfig) error

	// Update 更新配置
	Update(ctx context.Context, config *AgentConfig) error

	// Delete 删除配置
	Delete(ctx context.Context, id int64) error
}
