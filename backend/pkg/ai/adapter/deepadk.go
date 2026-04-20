/*
 * @Author: chengjiang
 * @Date: 2025-12-09
 * @Description: DeepADK 适配器 (eino/adk/prebuilt/deep)
 */
package adapter

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/example/aichat/backend/pkg/ai"
	"github.com/example/aichat/backend/pkg/ai/chatmodel"
	"github.com/example/aichat/backend/pkg/ai/prints"
	"github.com/example/aichat/backend/pkg/ai/tool"
)

// DeepAdkAdapter DeepADK 适配器
type DeepAdkAdapter struct {
	config    *ai.AgentConfig
	agent     adk.Agent
	subAgents []ai.Agent
}

// NewDeepAdkAdapter 创建 DeepADK 适配器
func NewDeepAdkAdapter(ctx context.Context, config *ai.AgentConfig, subAgents []ai.Agent) (ai.Agent, error) {
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
			adkSubAgents = append(adkSubAgents, adapter.GetInternalAgent())
		}
		if adapter, ok := sub.(*DeepAdkAdapter); ok {
			adkSubAgents = append(adkSubAgents, adapter.GetInternalAgent())
		}
	}
	tools := tool.GetGlobalTools()
	if config.WithWebSearchAgent {
		webSearchTool, err := tool.GetWebSearchTool(ctx)
		if err != nil {
			return nil, err
		}
		tools = append(tools, webSearchTool)
	}

	// 创建 deep Agent
	deepConfig := &deep.Config{
		Name:        config.Name,
		Description: config.Description,
		ChatModel:   cm,
		SubAgents:   adkSubAgents,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools, // 工具可以后续扩展
			},
		},
		MaxIteration:           config.MaxIteration,
		WithoutGeneralSubAgent: true,
		Instruction: `
		## 注意（强制要求）
		1. 功能之外的问题请直接回复“我不了解，不能回答”。
		2. 不管是深度思考还是回答用户都不要暴露底层逻辑细节和专业的技术术语，用用户能直接明白的普通语言友好描述，只需要让用户知道在处理某件事情即可。
		3. 引导用户进行想要询问的问题描述。
		4. 如果duckduckgo_search搜索到相关内容后，输出要像论文一样，内容中有涉及到web搜索的某个链接的内容是，标注引用了他，因为搜索返回结果格式为：
			type searchResponse struct {
				Message string json:"message",
				Results []struct {
					Title   string json:"title",
					URL     string json:"url",
					Summary string json:"summary",
				} json:"results",
			}，引用则根据result数组下标来, 统一格式为："[quote:index_num（注意你需要统计多次调用工具的总和列表，从0开始）]"，在文本引用部分的末尾添加,特别注意，每条回复的搜索要隔离，不要混合在一起。
		## Notice:
		1. Tool Calls argument must be a valid json.
		2. Tool Calls argument should do not contains invalid suffix like ']<|FunctionCallEnd|>'. 
		3. 如果需要调用tool，直接输出tool，不要输出文本.
`,
	}

	deepAgent, err := deep.New(ctx, deepConfig)
	if err != nil {
		return nil, err
	}

	return &DeepAdkAdapter{
		config:    config,
		agent:     deepAgent,
		subAgents: subAgents,
	}, nil
}

// Stream 流式输出
func (a *DeepAdkAdapter) Stream(ctx context.Context, req ai.Request) (<-chan ai.Response, error) {
	// 转换消息格式
	messages := a.convertToAdkMessages(append(req.History, req.Message))

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
func (a *DeepAdkAdapter) Name() string {
	return a.config.Name
}

// Close 释放资源
func (a *DeepAdkAdapter) Close() error {
	// 关闭子 Agent
	for _, sub := range a.subAgents {
		_ = sub.Close()
	}
	return nil
}

// GetInternalAgent 获取内部 adk.Agent（供其他适配器使用）
func (a *DeepAdkAdapter) GetInternalAgent() adk.Agent {
	return a.agent
}

// convertToAdkMessages 将自定义消息转换为 adk 消息
func (a *DeepAdkAdapter) convertToAdkMessages(msgs []*ai.Message) []adk.Message {
	var result []adk.Message
	lenMsgs := len(msgs)
	for i, msg := range msgs {
		t := &schema.Message{
			Role:             schema.RoleType(msg.Role),
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
		}
		if i == lenMsgs-1 {
			if msg.QuoteContent != "" {
				t.Content = fmt.Sprintf(`
				## quote content:
				> this is the quote content by user selected.
				%s
				---
				## user input:
				%s`, 
				msg.QuoteContent, msg.Content)
			}
		}
		result = append(result, t)
	}
	return result
}
