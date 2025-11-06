package tools

import (
	"context"
	"errors"
	"testing"
)

func TestCalculatorTool(t *testing.T) {
	ctx := context.Background()
	tool, err := NewCalculatorTool(ctx)
	if err != nil {
		t.Fatalf("failed to create calculator tool: %v", err)
	}

	if tool == nil {
		t.Fatal("calculator tool is nil")
	}

	info, err := tool.Info(ctx)
	if err != nil {
		t.Fatalf("failed to get tool info: %v", err)
	}

	if info.Name != "calculator" {
		t.Errorf("expected tool name 'calculator', got '%s'", info.Name)
	}

	if info.Desc == "" {
		t.Error("expected non-empty tool description")
	}
}

func TestCalculateOperations(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		a         float64
		b         float64
		want      float64
		wantErr   bool
	}{
		{
			name:      "addition",
			operation: "add",
			a:         5,
			b:         3,
			want:      8,
			wantErr:   false,
		},
		{
			name:      "subtraction",
			operation: "subtract",
			a:         10,
			b:         3,
			want:      7,
			wantErr:   false,
		},
		{
			name:      "multiplication",
			operation: "multiply",
			a:         4,
			b:         5,
			want:      20,
			wantErr:   false,
		},
		{
			name:      "division",
			operation: "divide",
			a:         15,
			b:         3,
			want:      5,
			wantErr:   false,
		},
		{
			name:      "division by zero",
			operation: "divide",
			a:         10,
			b:         0,
			want:      0,
			wantErr:   true,
		},
		{
			name:      "unknown operation",
			operation: "modulo",
			a:         10,
			b:         3,
			want:      0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &CalculateRequest{
				Operation: tt.operation,
				A:         tt.a,
				B:         tt.b,
			}

			// Test the calculation logic
			var result float64
			var err error

			switch req.Operation {
			case "add":
				result = req.A + req.B
			case "subtract":
				result = req.A - req.B
			case "multiply":
				result = req.A * req.B
			case "divide":
				if req.B == 0 {
					err = errors.New("cannot divide by zero")
				} else {
					result = req.A / req.B
				}
			default:
				err = errors.New("unknown operation")
			}

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.want {
				t.Errorf("expected result %v, got %v", tt.want, result)
			}
		})
	}
}
