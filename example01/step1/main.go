// Step 1: Basic Eino Terminal Chat
//
// This example shows you how to create a simple terminal-based chat agent
// using the Eino framework with Gemini.
//
// # Running the example:
//
//	go run examples/step1/main.go
//
// # Requirements:
//
//	GEMINI_API_KEY environment variable must be set

package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/olusolaa/goforai/foundation"
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

	// Use foundation component - no setup boilerplate!
	chatModel, err := foundation.NewChatModel(ctx)
	if err != nil {
		return nil, err
	}

	return &Agent{
		chatModel:      chatModel,
		getUserMessage: getUserMessage,
	}, nil
}

func (a *Agent) Run(ctx context.Context) error {
	var conversation []*schema.Message

	fmt.Printf("Chat with %s (use 'ctrl-c' to quit)\n", foundation.ChatModelName)

	for {
		fmt.Print("\nYou: ")
		userInput, ok := a.getUserMessage()
		if !ok {
			break
		}

		conversation = append(conversation, schema.UserMessage(userInput))

		resp, err := a.chatModel.Generate(ctx, conversation)
		if err != nil {
			fmt.Printf("\n\nERROR: %s\n\n", err)
			continue
		}

		fmt.Printf("\n%s: %s\n", foundation.ChatModelName, resp.Content)

		conversation = append(conversation, resp)
	}

	return nil
}
