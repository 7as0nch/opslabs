/*
 * @Author: chengjiang
 * @Date: 2025-12-09
 * @Description: Eino ChatModelAgent 适配器
 */
package adapter

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/example/aichat/backend/pkg/agenttools"
	"github.com/example/aichat/backend/pkg/ai"
	"github.com/example/aichat/backend/pkg/ai/chatmodel"
	"github.com/example/aichat/backend/pkg/ai/prints"
	"github.com/example/aichat/backend/pkg/ai/tool"
)

// EinoAdapter eino ChatModelAgent 适配器
type EinoAdapter struct {
	config *ai.AgentConfig
	agent  adk.Agent
}

// NewEinoAdapter 创建 Eino 适配器
func NewEinoAdapter(ctx context.Context, config *ai.AgentConfig, subAgents []ai.Agent) (ai.Agent, error) {
	// 创建 ChatModel
	cm, err := chatmodel.NewModel(ctx, chatmodel.ModelConfig{
		ModelType: chatmodel.ModelType(config.ModelConfig.ModelType),
		ModelName: config.ModelConfig.ModelName,
		ApiKey:    config.ModelConfig.APIKey,
		BaseURL:   config.ModelConfig.BaseURL,
		Thinking:  config.ModelConfig.Thinking,
	},
		chatmodel.WithMaxTokens(config.ModelConfig.MaxTokens),
		chatmodel.WithTemperature(config.ModelConfig.Temperature),
		chatmodel.WithTopP(config.ModelConfig.TopP),
		chatmodel.WithDisableThinking(!config.ModelConfig.Thinking),
	)
	if err != nil {
		return nil, err
	}

	// 转换子 Agent 为 adk.Agent
	var adkSubAgents []adk.Agent
	for _, sub := range subAgents {
		if adapter, ok := sub.(*EinoAdapter); ok {
			adkSubAgents = append(adkSubAgents, adapter.agent)
		}
		if adapter, ok := sub.(*DeepAdkAdapter); ok {
			adkSubAgents = append(adkSubAgents, adapter.agent)
		}
	}
	tools := agenttools.GetBussinessTools()
	if config.WithWebSearchAgent {
		webSearchTool, err := tool.GetWebSearchTool(ctx)
		if err != nil {
			return nil, err
		}
		tools = append(tools, webSearchTool)
	}

	// 创建 adk Agent
	agentConfig := &adk.ChatModelAgentConfig{
		Name:        config.Name,
		Description: config.Description,
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools, // 工具可以后续扩展
			},
		},
// 		Instruction: `
// Notice:
// 1. Tool Calls argument must be a valid json.
// 2. Tool Calls argument should do not contains invalid suffix like ']<|FunctionCallEnd|>'.`,
	}
	agent, err := adk.NewChatModelAgent(ctx, agentConfig)
	if err != nil {
		return nil, err
	}

	// 如果有子 Agent，设置子 Agent
	if len(adkSubAgents) > 0 {
		host, err := adk.SetSubAgents(ctx, agent, adkSubAgents)
		if err != nil {
			return nil, err
		}
		return &EinoAdapter{
			config: config,
			agent:  host,
		}, nil
	}

	return &EinoAdapter{
		config: config,
		agent:  agent,
	}, nil
}

// Stream 流式输出
func (a *EinoAdapter) Stream(ctx context.Context, req ai.Request) (<-chan ai.Response, error) {
	// 转换消息格式
	messages := convertToAdkMessages(append(req.History, req.Message))

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           a.agent,
		EnableStreaming: true,
	})

	iter := runner.Run(ctx, messages)
	out := make(chan ai.Response)

	go func() {
		defer close(out)
		for {
			event, ok := iter.Next()
			if !ok {
				break
			}
			if event.Err != nil {
				// if errors.Is(event.Err, context.Canceled) || strings.Contains(event.Err.Error(), "context canceled") {
				// 	log.Infof("Stream canceled: %v", event.Err)
				// } else {
				// 	log.Errorf("Stream error: %v", event.Err)
				// }
				out <- ai.Response{Error: event.Err}
				break
			}
			prints.EventHandler(event, func(msg *ai.Message, err error) {
				if err != nil {
					out <- ai.Response{Error: err}
					return
				}
				out <- ai.Response{Message: msg}
			})
		}
	}()

	return out, nil
}

// Name 返回 Agent 名称
func (a *EinoAdapter) Name() string {
	return a.config.Name
}

// Close 释放资源
func (a *EinoAdapter) Close() error {
	return nil
}

// GetInternalAgent 获取内部 adk.Agent（供其他适配器使用）
func (a *EinoAdapter) GetInternalAgent() adk.Agent {
	return a.agent
}

// convertToAdkMessages 将自定义消息转换为 adk 消息
func convertToAdkMessages(msgs []*ai.Message) []adk.Message {
	var result []adk.Message
	for _, msg := range msgs {
		result = append(result, &schema.Message{
			Role:    schema.RoleType(msg.Role),
			Content: msg.Content,
		})
	}
	return result
}