/* *
 * @Author: chengjiang
 * @Date: 2025-12-09 22:23:17
 * @Description:
**/
package agent

import (
	"context"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/example/aichat/backend/pkg/ai"
	"github.com/example/aichat/backend/pkg/ai/chatmodel"
	"github.com/example/aichat/backend/pkg/ai/tool"
)

func NewGlobalAgent(ctx context.Context, config *ai.AgentConfig) (adk.Agent, error) {
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

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "GlobalAgent",
		Description: "GlobalAgent 获取基础系统信息，如当前时间等。",
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tool.GetGlobalTools(),
			},
		},
		MaxIterations: 10,
	})
}