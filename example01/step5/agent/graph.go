// internal/agent/graph.go
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"github.com/olusolaa/goforai/foundation/gemini"
)

// buildEinoGraph encapsulates the declarative orchestration logic. It defines
// the flow of data between components using Eino's type-safe graph primitives.
func buildEinoGraph(ctx context.Context) (compose.Runnable[*UserMessage, *schema.Message], error) {
	// Using constants for node names is a best practice for clarity and maintainability.
	const (
		NodeInputToHistory = "InputToHistory"
		NodeChatTemplate   = "ChatTemplate"
		NodeReactAgent     = "ReactAgent"
	)

	// The graph is statically typed with its input and output structs.
	// This prevents entire classes of runtime errors.
	g := compose.NewGraph[*UserMessage, *schema.Message]()

	// Node 1: A simple lambda to format the input for the prompt template.
	g.AddLambdaNode(NodeInputToHistory, compose.InvokableLambda(extractVariables))

	// Node 2: The prompt template that structures the input for the LLM.
	chatTemplate := createChatTemplate()
	g.AddChatTemplateNode(NodeChatTemplate, chatTemplate)

	// Node 3: The core ReAct agent, which handles the tool-use loop.
	reactAgentNode, err := createReactAgentNode(ctx)
	if err != nil {
		return nil, err
	}
	g.AddLambdaNode(NodeReactAgent, reactAgentNode)

	// Define the data flow through the graph by connecting the nodes.
	// The compiler validates that the output type of a node matches the
	// input type of the next, ensuring type safety.
	g.AddEdge(compose.START, NodeInputToHistory)
	g.AddEdge(NodeInputToHistory, NodeChatTemplate)
	g.AddEdge(NodeChatTemplate, NodeReactAgent)
	g.AddEdge(NodeReactAgent, compose.END)

	// Compile the graph into an executable Runnable. This validates the
	// graph's structure (e.g., checking for cycles) and optimizes it.
	return g.Compile(ctx, compose.WithGraphName("GopherConAgent"))
}

// extractVariables is a pure function that transforms the agent input
// into the map required by the chat template.
func extractVariables(_ context.Context, input *UserMessage) (map[string]any, error) {
	return map[string]any{
		"content": input.Query,
		"history": input.History,
		"date":    time.Now().Format("2006-01-02"),
	}, nil
}

// createChatTemplate defines the system prompt and message structure.
func createChatTemplate() prompt.ChatTemplate {
	systemPrompt := `You are an expert Go coding assistant. You are concise, proactive, and use your tools to answer questions.
- Use tools to find information instead of asking the user.
- **Analyze Errors:** If a tool fails, read the error, adapt, and try again.
- **Code Editing Strategy:** When a small, targeted code replacement fails with a syntax error, it means the tool requires 
	a larger, complete declaration. The correct recovery is to ESCALATE your scope: replace the entire parent function containing the bug.
- Current Date: {date}`

	return prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(systemPrompt),
		schema.MessagesPlaceholder("history", true),
		schema.UserMessage("{content}"),
	)
}

// createReactAgentNode builds the ReAct agent component, which includes
// the LLM, the list of available tools, and its configuration.
func createReactAgentNode(ctx context.Context) (*compose.Lambda, error) {
	chatModel, err := gemini.NewChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	toolsList, err := setupTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to set up tools: %w", err)
	}

	return buildReactAgent(ctx, chatModel, toolsList)
}

// buildReactAgent configures and constructs the Eino ReAct agent.
func buildReactAgent(ctx context.Context, chatModel model.ToolCallingChatModel, toolsList []tool.BaseTool) (*compose.Lambda, error) {
	config := &react.AgentConfig{
		MaxStep:          20,
		ToolCallingModel: chatModel,
	}
	config.ToolsConfig.Tools = toolsList

	reactAgent, err := react.NewAgent(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create react agent: %w", err)
	}

	// Wrap the agent's methods in a generic Lambda to make it compatible with the graph.
	return compose.AnyLambda(reactAgent.Generate, reactAgent.Stream, nil, nil)
}
