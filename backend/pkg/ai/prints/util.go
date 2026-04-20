package prints

import (
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/example/aichat/backend/pkg/ai"
	// "github.com/cloudwego/eino-examples/internal/logs"
)

func EventHandler(event *adk.AgentEvent, handlerFn func(msg *ai.Message, err error)) {
	log.Printf("name: %s\npath: %s", event.AgentName, event.RunPath)
	toolTracker := newToolTracker()
	if event.Output != nil && event.Output.MessageOutput != nil {
		if m := event.Output.MessageOutput.Message; m != nil {
			msg := &ai.Message{
				Role: ai.RoleType(schema.Assistant),
			}
			if len(m.Content) > 0 {
				if m.Role == schema.Tool {
					log.Printf("\ntool response: %s", m.Content)
				} else {
					log.Printf("\nanswer: %s", m.Content)
					msg.Content = m.Content
				}
			}
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					log.Printf("\ntool name: %s", tc.Function.Name)
					log.Printf("\narguments: %s", tc.Function.Arguments)
					toolTracker.UpsertToolCall(tc)
				}
			}
			if m.Role == schema.Tool && m.Content != "" {
				toolTracker.UpsertToolResult(m.ToolCallID, m.ToolName, m.Content)
				msg.QuoteSearchLinks = parseSearchLinks(m.ToolName, m.Content)
			}
			msg.CallingTools = toolTracker.Snapshot()
			handlerFn(msg, nil)
		} else if s := event.Output.MessageOutput.MessageStream; s != nil {
			toolMap := map[int][]*schema.Message{}
			var contentStart, thinkingStart bool
			for {
				chunk, err := s.Recv()
				if err != nil {
					if err == io.EOF {
						break
					}
					log.Printf("error: %v", err)
					handlerFn(nil, err)
					return
				}
				msg := &ai.Message{
					Role: ai.RoleType(chunk.Role),
				}
				if chunk.Role != schema.Tool {
					msg.ReasoningContent = chunk.ReasoningContent
					msg.Content = chunk.Content
				}
				if chunk.ReasoningContent != "" {
					if !thinkingStart {
						thinkingStart = true
						if chunk.Role == schema.Tool {
							log.Printf("\ntool response: ")
						} else {
							log.Printf("\nThinking: ")
						}
					}
				}
				if chunk.Content != "" {
					if !contentStart {
						contentStart = true
						if chunk.Role == schema.Tool {
							log.Printf("\ntool response: ")
						} else {
							log.Printf("\nanswer: ")
						}
					}
				}

				if len(chunk.ToolCalls) > 0 {
					for _, tc := range chunk.ToolCalls {
						index := tc.Index
						if index == nil {
							log.Fatalf("index is nil")
						}
						toolMap[*index] = append(toolMap[*index], &schema.Message{
							Role: chunk.Role,
							ToolCalls: []schema.ToolCall{
								{
									ID:    tc.ID,
									Type:  tc.Type,
									Index: tc.Index,
									Function: schema.FunctionCall{
										Name:      tc.Function.Name,
										Arguments: tc.Function.Arguments,
									},
								},
							},
						})
						if tc.Function.Name != "" {
							toolTracker.UpsertToolCall(tc)
						}
					}
				}
				if chunk.Role == schema.Tool && chunk.Content != "" {
					toolTracker.UpsertToolResult(chunk.ToolCallID, chunk.ToolName, chunk.Content)
					msg.QuoteSearchLinks = append(msg.QuoteSearchLinks, parseSearchLinks(chunk.ToolName, chunk.Content)...)
				}
				if chunk.ResponseMeta.FinishReason == "stop" {
					if chunk.ResponseMeta.Usage != nil {
						log.Printf("\nusage: %v", chunk.ResponseMeta.Usage)
						msg.TokenUsage = &ai.TokenUsage{
							InputTokens:   int64(chunk.ResponseMeta.Usage.PromptTokens),
							CurrentTokens: int64(chunk.ResponseMeta.Usage.CompletionTokens),
							TotalTokens:   int64(chunk.ResponseMeta.Usage.TotalTokens),
						}
					}
				}
				msg.CallingTools = toolTracker.Snapshot()
				handlerFn(msg, nil)
			}

			for _, msgs := range toolMap {
				m, err := schema.ConcatMessages(msgs)
				if err != nil {
					log.Fatalf("ConcatMessage failed: %v", err)
					return
				}
				log.Printf("\ntool name: %s", m.ToolCalls[0].Function.Name)
				log.Printf("\narguments: %s", m.ToolCalls[0].Function.Arguments)
			}
		}
	}
	if event.Action != nil {
		if event.Action.TransferToAgent != nil {
			fmt.Printf("\naction: transfer to %v", event.Action.TransferToAgent.DestAgentName)
		}
		if event.Action.Interrupted != nil {
			ii, _ := json.MarshalIndent(event.Action.Interrupted.Data, "  ", "  ")
			log.Println("action: interrupted")
			log.Printf("interrupt snapshot: %v\n", string(ii))
		}
		// if event.Action.Exit {
		// 	fmt.Printf("\naction: exit")
		// }
	}
	if event.Err != nil {
		handlerFn(nil, event.Err)
	}
}

type toolTracker struct {
	index map[string]*ai.CallingTool
	order []string
}

func newToolTracker() *toolTracker {
	return &toolTracker{
		index: map[string]*ai.CallingTool{},
		order: make([]string, 0, 4),
	}
}

func (t *toolTracker) UpsertToolCall(tc schema.ToolCall) {
	key := toolKey(tc.ID, tc.Function.Name, tc.Function.Arguments)
	if key == "" {
		return
	}
	if existing, ok := t.index[key]; ok {
		if existing.Name == "" {
			existing.Name = tc.Function.Name
		}
		if existing.FunctionName == "" {
			existing.FunctionName = tc.Function.Name
		}
		if existing.Args == nil && tc.Function.Arguments != "" {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				log.Printf("Unmarshal arguments failed: %v", err)
			} else {
				existing.Args = args
			}
		}
		return
	}

	tool := &ai.CallingTool{
		Name:         tc.Function.Name,
		FunctionName: tc.Function.Name,
	}
	if tc.Function.Arguments != "" {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			log.Printf("Unmarshal arguments failed: %v", err)
		} else {
			tool.Args = args
		}
	}
	t.index[key] = tool
	t.order = append(t.order, key)
}

func (t *toolTracker) UpsertToolResult(toolCallID, toolName, result string) {
	key := toolKey(toolCallID, toolName, "")
	if key == "" {
		return
	}
	if existing, ok := t.index[key]; ok {
		if existing.Result == nil && result != "" {
			existing.Result = result
		}
		if existing.Name == "" {
			existing.Name = toolName
		}
		if existing.FunctionName == "" {
			existing.FunctionName = toolName
		}
		return
	}
	t.index[key] = &ai.CallingTool{
		Name:         toolName,
		FunctionName: toolName,
		Result:       result,
	}
	t.order = append(t.order, key)
}

func (t *toolTracker) Snapshot() []*ai.CallingTool {
	if len(t.order) == 0 {
		return nil
	}
	tools := make([]*ai.CallingTool, 0, len(t.order))
	for _, key := range t.order {
		if tool, ok := t.index[key]; ok {
			tools = append(tools, tool)
		}
	}
	return tools
}

func toolKey(toolCallID, name, args string) string {
	if toolCallID != "" {
		return "id:" + toolCallID
	}
	if name == "" && args == "" {
		return ""
	}
	if args != "" {
		return "name:" + name + "|args:" + args
	}
	return "name:" + name
}

type searchResponse struct {
	Message string `json:"message"`
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Summary string `json:"summary"`
	} `json:"results"`
}

func parseSearchLinks(toolName, content string) []*ai.QuoteSearchLink {
	if content == "" {
		return nil
	}
	if toolName != "" && toolName != "duckduckgo_search" {
		return nil
	}
	var resp searchResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil
	}
	if len(resp.Results) == 0 {
		return nil
	}
	seen := map[string]bool{}
	links := make([]*ai.QuoteSearchLink, 0, len(resp.Results))
	for _, item := range resp.Results {
		if item.URL == "" || seen[item.URL] {
			continue
		}
		seen[item.URL] = true
		links = append(links, &ai.QuoteSearchLink{
			Url:     item.URL,
			Title:   item.Title,
			Content: item.Summary,
		})
	}
	return links
}
