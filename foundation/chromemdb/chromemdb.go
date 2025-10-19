package chromemdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	chromem "github.com/philippgille/chromem-go"
)

// Constants for default values improve readability and maintainability.
const (
	defaultTopK = 5
)

// ChromemDB is a wrapper around chromem.DB that implements the Indexer and Retriever interfaces.
// It is designed to be configured via functional options and relies on a dependency-injected embedder.
type ChromemDB struct {
	collection *chromem.Collection
	db         *chromem.DB
	embedder   embedding.Embedder
	topK       int
}

// config holds the optional configuration for creating a new ChromemDB instance.
// It is unexported as it's an implementation detail of the constructor.
type config struct {
	db     *chromem.DB
	dbPath string
	topK   int
}

// Option defines the functional option type for configuring ChromemDB.
type Option func(*config)

// WithDB provides an existing chromem.DB instance.
// This is useful when the DB lifecycle is managed externally.
func WithDB(db *chromem.DB) Option {
	return func(c *config) {
		c.db = db
	}
}

// WithDBPath specifies a file path to load an existing database from.
// If the file doesn't exist, initialization will fail.
func WithDBPath(path string) Option {
	return func(c *config) {
		c.dbPath = path
	}
}

// WithTopK sets the number of results to retrieve in a search query.
func WithTopK(topK int) Option {
	return func(c *config) {
		c.topK = topK
	}
}

func New(ctx context.Context, collectionName string, embedder embedding.Embedder, opts ...Option) (*ChromemDB, error) {
	// --- 1. Validate Required Arguments (Fail Fast) ---
	if collectionName == "" {
		return nil, errors.New("collectionName cannot be empty")
	}
	if embedder == nil {
		return nil, errors.New("embedder cannot be nil")
	}

	cfg := &config{
		topK: defaultTopK, 
	}
	for _, opt := range opts {
		opt(cfg) 
	}

	var db *chromem.DB
	switch {
	case cfg.db != nil:
		db = cfg.db
	case cfg.dbPath != "":
		db = chromem.NewDB()
		if _, err := os.Stat(cfg.dbPath); errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("database not found at %s: run indexing and export first", cfg.dbPath)
		}
		if err := db.ImportFromFile(cfg.dbPath, ""); err != nil {
			return nil, fmt.Errorf("failed to import database from %s: %w", cfg.dbPath, err)
		}
	default:
		return nil, errors.New("configuration requires one of WithDB() or WithDBPath()")
	}

	embeddingFunc := func(ctx context.Context, text string) ([]float32, error) {
		embeddings, err := embedder.EmbedStrings(ctx, []string{text})
		if err != nil {
			return nil, fmt.Errorf("embedding failed: %w", err)
		}
		if len(embeddings) == 0 || len(embeddings[0]) == 0 {
			return nil, errors.New("embedder returned no embeddings")
		}
		return convertToFloat32(embeddings[0]), nil
	}

	collection, err := db.GetOrCreateCollection(collectionName, nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create collection '%s': %w", collectionName, err)
	}

	fmt.Printf("✅ Initialized ChromemDB with %d documents in collection '%s'.\n", collection.Count(), collectionName)

	return &ChromemDB{
		collection: collection,
		db:         db,
		embedder:   embedder,
		topK:       cfg.topK,
	}, nil
}

func (c *ChromemDB) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	chromemDocs := make([]chromem.Document, len(docs))
	ids := make([]string, len(docs))

	for i, doc := range docs {
		docID := doc.ID
		if docID == "" {
			docID = uuid.New().String()
		}
		ids[i] = docID

		metadata := make(map[string]string)
		if doc.MetaData != nil {
			for k, v := range doc.MetaData {
				metadata[k] = fmt.Sprint(v)
			}
		}

		chromemDocs[i] = chromem.Document{
			ID:       docID,
			Content:  doc.Content,
			Metadata: metadata,
		}
	}

	if err := c.collection.AddDocuments(ctx, chromemDocs, runtime.NumCPU()); err != nil {
		return nil, fmt.Errorf("failed to batch add documents: %w", err)
	}

	return ids, nil
}

// Retrieve finds relevant documents for a given query.
func (c *ChromemDB) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	embeddings, err := c.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for query: %w", err)
	}
	if len(embeddings) == 0 || len(embeddings[0]) == 0 {
		return nil, errors.New("embedder generated an empty embedding for the query")
	}

	embedding32 := convertToFloat32(embeddings[0])

	results, err := c.collection.QueryEmbedding(ctx, embedding32, c.topK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}

	outDocs := make([]*schema.Document, len(results))
	for i, result := range results {
		metadata := make(map[string]any, len(result.Metadata))
		for k, v := range result.Metadata {
			metadata[k] = v
		}

		doc := &schema.Document{
			ID:       result.ID,
			Content:  result.Content,
			MetaData: metadata,
		}
		doc.WithScore(float64(result.Similarity))
		outDocs[i] = doc
	}

	return outDocs, nil
}

func ExportDB(db *chromem.DB, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := db.ExportToFile(path, false, ""); err != nil {
		return fmt.Errorf("failed to export database to %s: %w", path, err)
	}
	return nil
}

func convertToFloat32(embeddings []float64) []float32 {
	embedding32 := make([]float32, len(embeddings))
	for i, v := range embeddings {
		embedding32[i] = float32(v)
	}
	return embedding32
}

var _ indexer.Indexer = (*ChromemDB)(nil)
var _ retriever.Retriever = (*ChromemDB)(nil)
