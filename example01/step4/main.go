// Step 4: Tool Calling in ISOLATION
//
// This example demonstrates the tool calling workflow in concept.
// We show how the model requests tools and how the conversation flow works.
//
// The workflow:
//  1. Model receives a question and available tools
//  2. Model decides it needs a tool and responds with tool request
//  3. We execute the tool
//  4. We send tool result back to model
//  5. Model provides final answer using tool result
//
// # Running the example:
//
//	go run examples/step4/main.go
//
// # Requirements:
//
//	GEMINI_API_KEY environment variable must be set

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/olusolaa/goforai/foundation"
	"github.com/olusolaa/goforai/foundation/tools"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	// Hardcoded question to demonstrate tool calling
	question := "What is 25 multiplied by 17?"

	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Question: %s\n", question)
	fmt.Println(strings.Repeat("=", 80))

	// STEP 1: CREATE THE CALCULATOR TOOL
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("STEP 1: Registering Calculator Tool")
	fmt.Println(strings.Repeat("=", 80))

	// Use foundation component!
	calcTool, err := tools.NewCalculatorTool(ctx)
	if err != nil {
		return fmt.Errorf("failed to create calculator tool: %w", err)
	}

	toolInfo, err := calcTool.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tool info: %w", err)
	}

	fmt.Println("\n✅ Tool Registered:")
	fmt.Printf("   Name: %s\n", toolInfo.Name)
	fmt.Printf("   Description: %s\n", toolInfo.Desc)
	fmt.Printf("   Parameters: %v\n", toolInfo.ParamsOneOf)

	// STEP 2: CREATE CHAT MODEL WITH TOOL
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("STEP 2: Sending Question to Model with Tool Definition")
	fmt.Println(strings.Repeat("=", 80))

	// Use foundation component!
	chatModel, err := foundation.NewChatModel(ctx)
	if err != nil {
		return fmt.Errorf("failed to create chat model: %w", err)
	}

	// STEP 3: FIRST MODEL CALL - MODEL SHOULD REQUEST TOOL
	conversation := []*schema.Message{
		schema.UserMessage(question),
	}

	toolInfos := []*schema.ToolInfo{toolInfo}

	fmt.Println("\n📤 Sending to model with calculator tool available...")

	resp, err := chatModel.Generate(ctx, conversation, model.WithTools(toolInfos))
	if err != nil {
		return fmt.Errorf("failed to generate response: %w", err)
	}

	fmt.Printf("\n📥 Model Response Role: %s\n", resp.Role)

	// Check if model requested tool calls
	if len(resp.ToolCalls) == 0 {
		fmt.Println("\n⚠️  Model did not request any tool calls")
		fmt.Printf("Instead, it responded directly: %s\n", resp.Content)
		return nil
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("STEP 3: Model Requested Tool Call!")
	fmt.Println(strings.Repeat("=", 80))

	fmt.Printf("\n\u001b[92m✅ Tool Call Request:\u001b[0m\n")
	for _, tc := range resp.ToolCalls {
		fmt.Printf("   Tool Call ID: %s\n", tc.ID)
		fmt.Printf("   Function: %s\n", tc.Function.Name)
		fmt.Printf("   Arguments: %v\n\n", tc.Function.Arguments)
	}

	// STEP 4: SIMULATE TOOL EXECUTION
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("STEP 4: Executing Tool")
	fmt.Println(strings.Repeat("=", 80))

	conversation = append(conversation, resp)

	toolCall := resp.ToolCalls[0]

	fmt.Printf("\nExecuting: %s with args: %v\n", toolCall.Function.Name, toolCall.Function.Arguments)

	// Parse and execute
	argsJSON, err := json.Marshal(toolCall.Function.Arguments)
	if err != nil {
		return fmt.Errorf("failed to marshal arguments: %w", err)
	}

	var calcArgs tools.CalculateRequest
	if err := json.Unmarshal(argsJSON, &calcArgs); err != nil {
		return fmt.Errorf("failed to unmarshal arguments: %w", err)
	}

	// Calculate result
	var result float64
	switch calcArgs.Operation {
	case "multiply":
		result = calcArgs.A * calcArgs.B
	case "add":
		result = calcArgs.A + calcArgs.B
	case "subtract":
		result = calcArgs.A - calcArgs.B
	case "divide":
		if calcArgs.B != 0 {
			result = calcArgs.A / calcArgs.B
		}
	}

	toolResult := fmt.Sprintf(`{"result": %.0f}`, result)
	fmt.Printf("\n\u001b[92m✅ Tool Result: %s\u001b[0m\n", toolResult)

	// Create tool response message
	toolMsg := &schema.Message{
		Role:       schema.Tool,
		Content:    toolResult,
		ToolCallID: toolCall.ID,
	}
	conversation = append(conversation, toolMsg)

	// STEP 5: SECOND MODEL CALL - MODEL SHOULD PROVIDE FINAL ANSWER
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("STEP 5: Sending Tool Result Back to Model")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	fmt.Println("📤 Sending tool result to model...")

	finalResp, err := chatModel.Generate(ctx, conversation)
	if err != nil {
		return fmt.Errorf("failed to generate final response: %w", err)
	}

	fmt.Printf("\n📥 \u001b[93m%s\u001b[0m: %s\n", foundation.ChatModelName, finalResp.Content)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("✅ Tool Calling Workflow Complete!")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("\nWhat happened:")
	fmt.Println("  1. ✅ Model identified need for calculator tool")
	fmt.Println("  2. ✅ Model requested tool call with specific arguments")
	fmt.Println("  3. ✅ We executed the tool and got result")
	fmt.Println("  4. ✅ We sent result back to model")
	fmt.Println("  5. ✅ Model provided natural language answer using result")
	fmt.Println(strings.Repeat("=", 80))

	return nil
}
