// Step 2: Add System Prompt & Better Terminal Formatting
//
// This example adds a system prompt to guide the agent's behavior and
// improves the UI with color-coded output and streaming responses.
//
// # Running the example:
//
//	go run examples/step2/main.go
//
// # Requirements:
//
//	GEMINI_API_KEY environment variable must be set

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/olusolaa/goforai/foundation"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		if !scanner.Scan() {
			return "", false
		}
		return scanner.Text(), true
	}

	agent, err := NewAgent(getUserMessage)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	return agent.Run(context.Background())
}

type Agent struct {
	chatModel      model.ToolCallingChatModel
	getUserMessage func() (string, bool)
}

func NewAgent(getUserMessage func() (string, bool)) (*Agent, error) {
	ctx := context.Background()

	// Use foundation component - focus on what's new!
	chatModel, err := foundation.NewChatModel(ctx)
	if err != nil {
		return nil, err
	}

	return &Agent{
		chatModel:      chatModel,
		getUserMessage: getUserMessage,
	}, nil
}

// System prompt to guide agent behavior
const systemPrompt = `You are a helpful coding assistant for the Eino framework.

The Eino framework is a Go-based AI application framework that helps developers
build LLM-powered applications with composable components.

Be concise and helpful in your responses.`

func (a *Agent) Run(ctx context.Context) error {
	var conversation []*schema.Message

	// Add system prompt
	conversation = append(conversation, schema.SystemMessage(systemPrompt))

	fmt.Printf("\nChat with %s (use 'ctrl-c' to quit)\n", foundation.ChatModelName)

	for {
		// Color-coded user prompt (Blue)
		fmt.Print("\u001b[94m\nYou\u001b[0m: ")
		userInput, ok := a.getUserMessage()
		if !ok {
			break
		}

		conversation = append(conversation, schema.UserMessage(userInput))

		// Color-coded model name (Yellow)
		fmt.Printf("\u001b[93m\n%s\u001b[0m: ", foundation.ChatModelName)

		// Use streaming for real-time responses
		streamReader, err := a.chatModel.Stream(ctx, conversation)
		if err != nil {
			fmt.Printf("\n\n\u001b[91mERROR: %s\u001b[0m\n\n", err)
			continue
		}

		// Collect chunks as they stream
		var chunks []*schema.Message
		for {
			chunk, err := streamReader.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Printf("\n\n\u001b[91mERROR: %s\u001b[0m\n\n", err)
				break
			}

			// Display content as it streams
			fmt.Print(chunk.Content)
			chunks = append(chunks, chunk)
		}

		fmt.Print("\n")

		// Add complete response to conversation history
		if len(chunks) > 0 {
			fullMsg, err := schema.ConcatMessages(chunks)
			if err != nil {
				fmt.Printf("\n\n\u001b[91mERROR concatenating messages: %s\u001b[0m\n\n", err)
			} else {
				conversation = append(conversation, fullMsg)
			}
		}
	}

	return nil
}
