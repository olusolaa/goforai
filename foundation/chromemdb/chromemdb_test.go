package chromemdb

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/embedding"
)

func TestNewChromemDB_EmptyCollectionName(t *testing.T) {
	ctx := context.Background()
	mockEmbedder := &mockEmbedder{}

	_, err := New(ctx, "", mockEmbedder)
	if err == nil {
		t.Error("expected error with empty collection name, got nil")
	}

	expectedMsg := "collectionName cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestNewChromemDB_NilEmbedder(t *testing.T) {
	ctx := context.Background()

	_, err := New(ctx, "test-collection", nil)
	if err == nil {
		t.Error("expected error with nil embedder, got nil")
	}

	expectedMsg := "embedder cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestNewChromemDB_NoConfigProvided(t *testing.T) {
	ctx := context.Background()
	mockEmbedder := &mockEmbedder{}

	_, err := New(ctx, "test-collection", mockEmbedder)
	if err == nil {
		t.Error("expected error when no config is provided, got nil")
	}

	expectedMsg := "configuration requires one of WithDB() or WithDBPath()"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestNewChromemDB_NonExistentPath(t *testing.T) {
	ctx := context.Background()
	mockEmbedder := &mockEmbedder{}

	_, err := New(ctx, "test-collection", mockEmbedder, WithDBPath("/nonexistent/path/db.gob"))
	if err == nil {
		t.Error("expected error with non-existent path, got nil")
	}
}

func TestConvertToFloat32(t *testing.T) {
	tests := []struct {
		name  string
		input []float64
		want  []float32
	}{
		{
			name:  "empty slice",
			input: []float64{},
			want:  []float32{},
		},
		{
			name:  "single value",
			input: []float64{1.5},
			want:  []float32{1.5},
		},
		{
			name:  "multiple values",
			input: []float64{1.0, 2.5, 3.14159},
			want:  []float32{1.0, 2.5, 3.14159},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToFloat32(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				// Use approximate comparison for floats
				diff := got[i] - tt.want[i]
				if diff < -0.0001 || diff > 0.0001 {
					t.Errorf("index %d: got %f, want %f", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDefaultConstants(t *testing.T) {
	if defaultTopK <= 0 {
		t.Errorf("defaultTopK should be positive, got %d", defaultTopK)
	}

	expectedTopK := 5
	if defaultTopK != expectedTopK {
		t.Errorf("expected defaultTopK to be %d, got %d", expectedTopK, defaultTopK)
	}
}

// mockEmbedder is a simple mock for testing
type mockEmbedder struct{}

func (m *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = []float64{0.1, 0.2, 0.3}
	}
	return result, nil
}
