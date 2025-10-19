// Package tools provides reusable tool definitions.
// Single source of truth for all agent tools.
package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// CalculateRequest represents calculator tool input.
type CalculateRequest struct {
	Operation string  `json:"operation" jsonschema:"description=The operation to perform: add, subtract, multiply, or divide"`
	A         float64 `json:"a" jsonschema:"description=First number"`
	B         float64 `json:"b" jsonschema:"description=Second number"`
}

// CalculateResponse represents calculator tool output.
type CalculateResponse struct {
	Result float64 `json:"result" jsonschema:"description=The result of the calculation"`
}

// NewCalculatorTool creates a calculator tool for basic arithmetic.
func NewCalculatorTool(ctx context.Context) (tool.BaseTool, error) {
	return utils.InferTool(
		"calculator",
		"Perform basic arithmetic operations: add, subtract, multiply, or divide two numbers",
		func(ctx context.Context, req *CalculateRequest) (*CalculateResponse, error) {
			var result float64

			switch req.Operation {
			case "add":
				result = req.A + req.B
			case "subtract":
				result = req.A - req.B
			case "multiply":
				result = req.A * req.B
			case "divide":
				if req.B == 0 {
					return nil, fmt.Errorf("cannot divide by zero")
				}
				result = req.A / req.B
			default:
				return nil, fmt.Errorf("unknown operation: %s", req.Operation)
			}

			return &CalculateResponse{Result: result}, nil
		},
	)
}


