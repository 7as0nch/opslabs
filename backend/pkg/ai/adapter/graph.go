/* *
 * @Author: chengjiang
 * @Date: 2026-01-18 22:41:45
 * @Description: Graph Adapter for Eino Workflows
**/
package adapter

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/example/aichat/backend/pkg/ai"
	"github.com/example/aichat/backend/pkg/ai/chatmodel"
)

type GraphAdapter struct {
	config    *ai.AgentConfig
	runnable  compose.Runnable[[]*schema.Message, *schema.Message]
	subAgents []ai.Agent
}

// Close implements [ai.Agent].
func (g *GraphAdapter) Close() error {
	for _, sub := range g.subAgents {
		_ = sub.Close()
	}
	return nil
}

// Name implements [ai.Agent].
func (g *GraphAdapter) Name() string {
	return g.config.Name
}

// Stream implements [ai.Agent].
func (g *GraphAdapter) Stream(ctx context.Context, req ai.Request) (<-chan ai.Response, error) {
	if g.runnable == nil {
		return nil, fmt.Errorf("graph not compiled")
	}

	messages := g.convertToSchemaMessages(append(req.History, req.Message))

	// For streaming, we use the graph's Stream method
	sr, err := g.runnable.Stream(ctx, messages)
	if err != nil {
		return nil, err
	}

	out := make(chan ai.Response)
	go func() {
		defer close(out)
		defer sr.Close()
		for {
			msg, err := sr.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				out <- ai.Response{Error: err}
				break
			}

			// For now, simple conversion back to ai.Message
			resMsg := &ai.Message{
				Role:             ai.RoleType(msg.Role),
				Content:          msg.Content,
				ReasoningContent: msg.ReasoningContent,
			}
			out <- ai.Response{Message: resMsg}
		}
	}()

	return out, nil
}

func (g *GraphAdapter) convertToSchemaMessages(msgs []*ai.Message) []*schema.Message {
	var result []*schema.Message
	for _, msg := range msgs {
		result = append(result, &schema.Message{
			Role:             schema.RoleType(msg.Role),
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
		})
	}
	return result
}

func NewGraph(ctx context.Context, config *ai.AgentConfig, subAgents []ai.Agent) (ai.Agent, error) {
	if config.WorkflowConfig == nil {
		return nil, fmt.Errorf("workflow config is required for Graph adapter")
	}

	// Create a new graph
	// We standardize on []*schema.Message as input and *schema.Message as output for Agent compatibility
	graph := compose.NewGraph[[]*schema.Message, *schema.Message]()

	// Build nodes
	for _, node := range config.WorkflowConfig.Nodes {
		err := addNodeToGraph(ctx, graph, node, config, subAgents)
		if err != nil {
			return nil, fmt.Errorf("failed to add node %s: %w", node.ID, err)
		}
	}

	// Build edges
	for _, edge := range config.WorkflowConfig.Edges {
		source := edge.Source
		if source == "start" {
			source = compose.START
		}
		target := edge.Target
		if target == "end" {
			target = compose.END
		}
		err := graph.AddEdge(source, target)
		if err != nil {
			return nil, fmt.Errorf("failed to add edge from %s to %s: %w", edge.Source, edge.Target, err)
		}
	}

	// Compile the graph
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to compile graph: %w", err)
	}

	return &GraphAdapter{
		config:    config,
		runnable:  runnable,
		subAgents: subAgents,
	}, nil
}

func addNodeToGraph(ctx context.Context, graph *compose.Graph[[]*schema.Message, *schema.Message], node ai.NodeConfig, agentConfig *ai.AgentConfig, subAgents []ai.Agent) error {
	switch node.Type {
	case ai.NodeTypeModel:
		// Logic to create ChatModel from node.Config
		cm, err := chatmodel.NewModel(ctx, chatmodel.ModelConfig{
			ModelType: chatmodel.ModelType(agentConfig.ModelConfig.ModelType),
			ModelName: agentConfig.ModelConfig.ModelName,
			ApiKey:    agentConfig.ModelConfig.APIKey,
			BaseURL:   agentConfig.ModelConfig.BaseURL,
			Thinking:  agentConfig.ModelConfig.Thinking,
		})
		if err != nil {
			return err
		}
		return graph.AddChatModelNode(node.ID, cm)

	case ai.NodeTypePrompt:
		// Logic to create PromptTemplate
		template, ok := node.Config.(string)
		if !ok {
			return fmt.Errorf("prompt node config must be a string template")
		}
		pt := prompt.FromMessages(schema.FString, schema.UserMessage(template))
		return graph.AddChatTemplateNode(node.ID, pt)

	case ai.NodeTypeAgent:
		// Find sub-agent by name or ID in node.Config
		agentName, ok := node.Config.(string)
		if !ok {
			return fmt.Errorf("agent node config must be the agent name")
		}
		var targetAgent ai.Agent
		for _, sa := range subAgents {
			if sa.Name() == agentName {
				targetAgent = sa
				break
			}
		}
		if targetAgent == nil {
			return fmt.Errorf("sub-agent %s not found", agentName)
		}

		// Wrap ai.Agent as an Eino Lambda
		lambda := compose.InvokableLambda(func(ctx context.Context, in []*schema.Message) (*schema.Message, error) {
			// Convert input back to ai.Request
			req := ai.Request{
				Message: &ai.Message{
					Role:    ai.RoleUser,
					Content: in[len(in)-1].Content,
				},
				History: nil, // Simplified for now
			}
			respChan, err := targetAgent.Stream(ctx, req)
			if err != nil {
				return nil, err
			}

			var fullContent string
			for res := range respChan {
				if res.Error != nil {
					return nil, res.Error
				}
				if res.Message != nil {
					fullContent += res.Message.Content
				}
			}
			return schema.AssistantMessage(fullContent, nil), nil
		})
		return graph.AddLambdaNode(node.ID, lambda)

	case ai.NodeTypeStart:
		return graph.AddLambdaNode(node.ID, compose.InvokableLambda(func(ctx context.Context, in []*schema.Message) ([]*schema.Message, error) {
			return in, nil
		}))

	case ai.NodeTypeEnd:
		return graph.AddLambdaNode(node.ID, compose.InvokableLambda(func(ctx context.Context, in *schema.Message) (*schema.Message, error) {
			return in, nil
		}))

	default:
		return fmt.Errorf("unsupported node type: %s", node.Type)
	}
}
