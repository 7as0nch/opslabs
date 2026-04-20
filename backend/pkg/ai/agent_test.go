package ai

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
)

// TestAIAgentConfig 测试 AIAgent 配置的创建和检查
func TestAIAgentConfig(t *testing.T) {
	// 创建配置
	config := AIAgentConfig{
		AgentConfig: AgentConfig{
			Name:        "测试助手",
			Description: "测试用的 AI 助手",
			ModelConfig: ModelConfig{
				ModelType:   "deepseek",
				ModelName:   "deepseek-chat",
				MaxTokens:   4096,
				Temperature: 0.7,
				TopP:        0.5,
				Thinking:    true,
			},
			MaxIteration: 100,
		},
		PromptConfig: PromptConfig{
			SystemPrompt: "你是一个测试助手，负责回答测试问题。",
			UserPrompt:   "请回答：{{.question}}",
			Variables: map[string]string{
				"question": "什么是 AI？",
			},
		},
	}

	// 检查配置
	if config.AgentConfig.Name != "测试助手" {
		t.Errorf("期望 Agent 名称为 '测试助手'，但得到了 '%s'", config.AgentConfig.Name)
	}

	if config.AgentConfig.Description != "测试用的 AI 助手" {
		t.Errorf("期望 Agent 描述为 '测试用的 AI 助手'，但得到了 '%s'", config.AgentConfig.Description)
	}

	// 检查模型配置
	modelConfig := config.AgentConfig.ModelConfig
	if modelConfig.ModelType != "deepseek" {
		t.Errorf("期望模型类型为 'deepseek'，但得到了 '%s'", modelConfig.ModelType)
	}

	if modelConfig.ModelName != "deepseek-chat" {
		t.Errorf("期望模型名称为 'deepseek-chat'，但得到了 '%s'", modelConfig.ModelName)
	}

	if modelConfig.MaxTokens != 4096 {
		t.Errorf("期望最大令牌数为 4096，但得到了 %d", modelConfig.MaxTokens)
	}

	if modelConfig.Temperature != 0.7 {
		t.Errorf("期望温度为 0.7，但得到了 %f", modelConfig.Temperature)
	}

	if modelConfig.TopP != 0.5 {
		t.Errorf("期望 Top P 为 0.5，但得到了 %f", modelConfig.TopP)
	}

	if !modelConfig.Thinking {
		t.Errorf("期望思考模式为 true，但得到了 false")
	}

	// 检查提示词配置
	promptConfig := config.PromptConfig
	if promptConfig.SystemPrompt != "你是一个测试助手，负责回答测试问题。" {
		t.Errorf("期望系统提示词为 '你是一个测试助手，负责回答测试问题。'，但得到了 '%s'", promptConfig.SystemPrompt)
	}

	if promptConfig.UserPrompt != "请回答：{{.question}}" {
		t.Errorf("期望用户提示词为 '请回答：{{.question}}'，但得到了 '%s'", promptConfig.UserPrompt)
	}

	if promptConfig.Variables["question"] != "什么是 AI？" {
		t.Errorf("期望提示词变量 question 为 '什么是 AI？'，但得到了 '%s'", promptConfig.Variables["question"])
	}

	t.Log("测试通过！")
}

// TestPromptManager 测试 PromptManager 的功能
func TestPromptManager(t *testing.T) {
	// 创建配置
	config := PromptConfig{
		SystemPrompt: "你是一个测试助手。",
		UserPrompt:   "请回答：{{.question}}",
		Variables: map[string]string{
			"question": "什么是 AI？",
		},
	}

	// 创建 PromptManager
	pm := NewPromptManager(config)

	// 测试渲染系统提示词
	systemPrompt := pm.RenderSystemPrompt()
	if systemPrompt != "你是一个测试助手。" {
		t.Errorf("期望系统提示词为 '你是一个测试助手。'，但得到了 '%s'", systemPrompt)
	}

	// 测试渲染用户提示词
	userPrompt, err := pm.RenderUserPrompt()
	if err != nil {
		t.Fatalf("渲染用户提示词失败：%v", err)
	}
	if userPrompt != "请回答：什么是 AI？" {
		t.Errorf("期望用户提示词为 '请回答：什么是 AI？'，但得到了 '%s'", userPrompt)
	}

	// 测试更新变量
	pm.UpdateVariables(map[string]string{
		"question": "什么是机器学习？",
	})
	userPrompt, err = pm.RenderUserPrompt()
	if err != nil {
		t.Fatalf("渲染用户提示词失败：%v", err)
	}
	if userPrompt != "请回答：什么是机器学习？" {
		t.Errorf("期望用户提示词为 '请回答：什么是机器学习？'，但得到了 '%s'", userPrompt)
	}

	t.Log("测试通过！")
}

func TestDuckDuckGo(t *testing.T) {
    ctx := context.Background()

    // Create configuration
    config := &duckduckgo.Config{
       MaxResults: 3, // Limit to return 20 results
       Region:     duckduckgo.RegionWT,
       Timeout:    10 * time.Second,
    }

    // Create search client
    tool, err := duckduckgo.NewTextSearchTool(ctx, config)
    if err != nil {
       t.Fatalf("NewTextSearchTool of duckduckgo failed, err=%v", err)
    }

    results := make([]*duckduckgo.TextSearchResult, 0, config.MaxResults)

    searchReq := &duckduckgo.TextSearchRequest{
       Query: "eino",
    }
    jsonReq, err := json.Marshal(searchReq)
    if err != nil {
       t.Fatalf("Marshal of search request failed, err=%v", err)
    }

    resp, err := tool.InvokableRun(ctx, string(jsonReq))
    if err != nil {
       t.Fatalf("Search of duckduckgo failed, err=%v", err)
    }

    var searchResp duckduckgo.TextSearchResponse
    if err = json.Unmarshal([]byte(resp), &searchResp); err != nil {
       t.Fatalf("Unmarshal of search response failed, err=%v", err)
    }

    results = append(results, searchResp.Results...)

    // Print results
    t.Logf("Search Results:")
    t.Logf("==============")
    t.Logf("%s\n", searchResp.Message)
    for i, result := range results {
       t.Logf("\n%d. Title: %s\n", i+1, result.Title)
       t.Logf("   URL: %s\n", result.URL)
       t.Logf("   Summary: %s\n", result.Summary)
    }
    t.Logf("")
    t.Logf("==============")
}
