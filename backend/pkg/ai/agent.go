/*
 * @Author: chengjiang
 * @Date: 2025-12-09
 * @Description: AI Agent 核心接口定义
 */
package ai

import (
	"context"
)

// Agent 定义了 AI Agent 的核心接口
// 所有具体实现（如 EinoAdapter, DeepAdkAdapter）必须实现此接口
type Agent interface {
	// Stream 执行流式输出，返回响应通道
	Stream(ctx context.Context, req Request) (<-chan Response, error)

	// Name 返回 Agent 名称
	Name() string

	// Close 释放资源
	Close() error
}

type TOOL_FUNC[T, D any] func(ctx context.Context, input T) (output D, err error)

// StreamRequest 流式请求
type Request struct {
	Message      *Message
	History      []*Message
	NeedTODOPlan NeedTODOPlan
}

// StreamResponse 流式响应
type Response struct {
	Message *Message
	Done    bool  `json:"done,omitempty"`
	Error   error `json:"-"`
}

type Message struct {
	ID               int64
	SessionID        int64
	Role             RoleType
	Content          string
	ReasoningContent string
	AIModel          *AIModel
	QuoteId          int64
	QuoteContent     string
	QuoteSearchLinks []*QuoteSearchLink
	TokenUsage       *TokenUsage
	CallingTools     []*CallingTool
	Attachments      []*Attachment
}

// 0. 智能，1. 需要，2. 不需要
type NeedTODOPlan uint8

const (
	NeedTODOPlanSmart  NeedTODOPlan = iota
	NeedTODOPlanNeed
	NeedTODOPlanUnNeed
)

// 'user' | 'assistant' | 'human'
type RoleType string

const (
	RoleUser      RoleType = "user"
	RoleAssistant RoleType = "assistant"
	RoleHuman     RoleType = "human"
)

// Custom Types

type AIModel struct {
	ID           string `json:"id"`
	ModelName    string `json:"modelName"`
	ThinkingMode string `json:"thinkingMode"` // 'smart' | 'deep' | 'quick'
	SearchByWeb  bool   `json:"searchByWeb"`
}

type QuoteSearchLink struct {
	Url       string   `json:"url"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Highlight []string `json:"highlight"`
}

type TokenUsage struct {
	CurrentTokens int64 `json:"currentTokens"`
	TotalTokens   int64 `json:"totalTokens"`
	InputTokens   int64 `json:"inputTokens"`
}

type CallingTool struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	FunctionName string                 `json:"functionName"`
	Args         map[string]interface{} `json:"args,omitempty"`
	Result       interface{}            `json:"result,omitempty"`
}

type Attachment struct {
	ID   string `json:"id"`
	Type string `json:"type"` // 'image' | 'file'
	Name string `json:"name"`
	Url  string `json:"url"`
}
