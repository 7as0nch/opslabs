/*
 * @Author: chengjiang
 * @Date: 2025-12-09
 * @Description: AI Agent 配置结构定义（支持数据库存储）
 */
package ai

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// AgentConfig 定义 Agent 的配置
type AgentConfig struct {
	Name               string          `json:"name" gorm:"uniqueIndex;size:100"`
	Description        string          `json:"description" gorm:"size:500"`              // agent prompt
	AdapterType        AdapterType     `json:"adapter_type" gorm:"size:50;default:eino"` // adk, deepadk 等
	ModelConfig        ModelConfig     `json:"model_config" gorm:"foreignKey:AgentID"`
	WorkflowConfig     *WorkflowConfig `json:"workflow_config"`
	MaxIteration       int             `json:"max_iteration" gorm:"default:10"`
	IsMaster           bool            `json:"is_master" gorm:"default:false"`
	Status             int             `json:"status" gorm:"default:1"` // 0: 禁用, 1: 启用
	ParentID           int64
	Order              int
	WithWriteTODOs     bool
	WithWebSearchAgent bool
}

type AdapterType string

const (
	AdapterTypeEino    AdapterType = "adk"
	AdapterTypeDeepAdk AdapterType = "deepadk"
	AdapterTypeHost    AdapterType = "host"
	AdapterTypeGraph   AdapterType = "graph"
)

// AppConfig 应用层面的角色配置
type AppConfig struct {
	ActiveType  ActiveType  `json:"active_type"`  // agent | workflow
	AdapterType AdapterType `json:"adapter_type"` // adk | deepadk | graph
	TargetCode  string      `json:"target_code"`  // AIAgent.Code or AIWorkflow.Code
	Config      interface{} `json:"config"`       // 额外配置覆盖
}

type ActiveType string

const (
	ActiveTypeAgent    ActiveType = "agent"
	ActiveTypeWorkflow ActiveType = "workflow"
)

// WorkflowConfig 定义工作流配置
type WorkflowConfig struct {
	Nodes []NodeConfig `json:"nodes"`
	Edges []EdgeConfig `json:"edges"`
}

// NodeConfig 定义工作流节点
type NodeConfig struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Type   NodeType    `json:"type"`
	Config interface{} `json:"config"` // 根据 Type 不同而不同
}

type NodeType string

const (
	NodeTypeStart    NodeType = "start"
	NodeTypeEnd      NodeType = "end"
	NodeTypeModel    NodeType = "model"
	NodeTypePrompt   NodeType = "prompt"
	NodeTypeTool     NodeType = "tool"
	NodeTypeAgent    NodeType = "agent"
	NodeTypeWorkflow NodeType = "workflow"
	NodeTypeLambda   NodeType = "lambda"
)

// EdgeConfig 定义工作流边
type EdgeConfig struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// ModelConfig 模型配置
type ModelConfig struct {
	AgentID     int64   `json:"agent_id" gorm:"index"`
	ModelType   string  `json:"model_type" gorm:"size:50"` // ark, openai, deepseek
	ModelName   string  `json:"model_name" gorm:"size:100"`
	APIKey      string  `json:"api_key" gorm:"size:255"` // 建议加密存储
	BaseURL     string  `json:"base_url" gorm:"size:255"`
	MaxTokens   int     `json:"max_tokens" gorm:"default:4096"`
	Temperature float32 `json:"temperature" gorm:"default:0.7"`
	TopP        float32 `json:"top_p" gorm:"default:0.9"`
	Thinking    bool    `json:"thinking" gorm:"default:false"`
	PriceType   string  `json:"price_type" gorm:"size:50"` // 计费方式
}

// PromptConfig 提示词配置
type PromptConfig struct {
	SystemPrompt string            `json:"system_prompt"`
	UserPrompt   string            `json:"user_prompt"`
	Variables    map[string]string `json:"variables"`
}

// ToolConfig 工具配置
type ToolConfig struct {
	ToolType    string                 `json:"tool_type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Params      map[string]interface{} `json:"params"`
}

// AIAgentConfig 完整 AI Agent 配置（用于内存中使用）
type AIAgentConfig struct {
	AgentConfig  AgentConfig     `json:"agent_config"`
	PromptConfig PromptConfig    `json:"prompt_config"`
	Tools        []ToolConfig    `json:"tools"`
	SubAgents    []AIAgentConfig `json:"sub_agents"`
}

// Scan implements the sql.Scanner interface for AppConfig
func (c *AppConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to unmarshal JSON value: %v", value)
	}
	return json.Unmarshal(bytes, c)
}

// Value implements the driver.Valuer interface for AppConfig
func (c AppConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface for WorkflowConfig
func (c *WorkflowConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("failed to unmarshal JSON value: %v", value)
	}
	return json.Unmarshal(bytes, c)
}

// Value implements the driver.Valuer interface for WorkflowConfig
func (c WorkflowConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}
