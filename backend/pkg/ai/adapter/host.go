/* *
 * @Author: chengjiang
 * @Date: 2025-12-16 17:55:04
 * @Description: host模式。
**/
package adapter

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/flow/agent/multiagent/host"
	"github.com/cloudwego/eino/schema"
	"github.com/example/aichat/backend/pkg/ai"
	"github.com/example/aichat/backend/pkg/ai/chatmodel"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent"
)

type HostAdapter struct {
	ma     *host.MultiAgent
	cm     model.ToolCallingChatModel
	config *ai.AgentConfig
}

func NewHost(ctx context.Context, config *ai.AgentConfig, subAgents []ai.Agent) (ai.Agent, error) {
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
	t := &HostAdapter{
		cm: cm,
		config: config,
	}
	ws, err := t.newWriteJournalSpecialist(ctx)
	if err != nil {
		return nil, err
	}
	hostMA, err := host.NewMultiAgent(ctx, &host.MultiAgentConfig{
		Host: host.Host{
			ToolCallingModel: cm,
			SystemPrompt:     "You can read and write journal on behalf of the user. When user asks a question, always answer with journal content.",
		},
		Specialists: []*host.Specialist{
			ws,
		},
	})
	if err != nil {
		panic(err)
	}
	t.ma = hostMA
	return t, nil
}

// Close implements ai.Agent.
func (h *HostAdapter) Close() error {
	panic("unimplemented")
}

// Name implements ai.Agent.
func (h *HostAdapter) Name() string {
	panic("unimplemented")
}

// Stream implements ai.Agent.
func (h *HostAdapter) Stream(ctx context.Context, req ai.Request) (<-chan ai.Response, error) {
	res, err := h.ma.Stream(ctx, h.convertToAdkMessages(append(req.History, req.Message)))
	if err != nil {
		panic(err)
	}
	out := make(chan ai.Response)

	go func() {
		defer close(out)
		defer res.Close()
		for {
			msg, err := res.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				out <- ai.Response{Error: err}
				return
			}
			out <- ai.Response{Message: &ai.Message{
				Role:             ai.RoleType(msg.Role),
				Content:          msg.Content,
				ReasoningContent: msg.ReasoningContent,
			}}
		}
	}()

	return out, nil
}

func (a *HostAdapter) convertToAdkMessages(msgs []*ai.Message) []*schema.Message {
	var result []*schema.Message
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

func appendJournal(text string) error {
	// open or create the journal file for today
	// get today's date
	now := time.Now()
	dateStr := now.Format("2006-01-02")

	filePath, err := getJournalFilePath(dateStr) // Assume this function returns the path to today's journal file
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// append text to file
	_, err = file.WriteString(text + "\n")
	if err != nil {
		return err
	}

	return nil
}

func getJournalFilePath(dateStr string) (string, error) {
	// generate the unique file path for today's journal file
	filePath := fmt.Sprintf("journal_%s.txt", dateStr)

	// find the file path for today's journal file
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// if not found, create the journal file with the file path
		file, err := os.Create(filePath)
		if err != nil {
			return "", err
		}
		file.Close()
	}

	// return the file path
	return filePath, nil
}

func (a *HostAdapter) newWriteJournalSpecialist(ctx context.Context) (*host.Specialist, error) {
	chatModel := a.cm

	// use a chat model to rewrite user query to journal entry
	// for example, the user query might be:
	//
	// write: I got up at 7:00 in the morning.
	//
	// should be rewritten to:
	//
	// I got up at 7:00 in the morning.
	chain := compose.NewChain[[]*schema.Message, *schema.Message]()
	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input []*schema.Message) ([]*schema.Message, error) {
		systemMsg := &schema.Message{
			Role:    schema.System,
			Content: "You are responsible for preparing the user query for insertion into journal. The user's query is expected to contain the actual text the user want to write to journal, as well as convey the intention that this query should be written to journal. You job is to remove that intention from the user query, while preserving as much as possible the user's original query, and output ONLY the text to be written into journal",
		}
		return append([]*schema.Message{systemMsg}, input...), nil
	})).
		AppendChatModel(chatModel).
		AppendLambda(compose.InvokableLambda(func(ctx context.Context, input *schema.Message) (*schema.Message, error) {
			err := appendJournal(input.Content)
			if err != nil {
				return nil, err
			}
			return &schema.Message{
				Role:    schema.Assistant,
				Content: "Journal written successfully: " + input.Content,
			}, nil
		}))

	r, err := chain.Compile(ctx)
	if err != nil {
		return nil, err
	}

	return &host.Specialist{
		AgentMeta: host.AgentMeta{
			Name:        "write_journal",
			IntendedUse: "treat the user query as a sentence of a journal entry, append it to the right journal file",
		},
		Invokable: func(ctx context.Context, input []*schema.Message, opts ...agent.AgentOption) (*schema.Message, error) {
			return r.Invoke(ctx, input, agent.GetComposeOptions(opts...)...)
		},
	}, nil
}
