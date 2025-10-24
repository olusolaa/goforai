package main

import (
	"context"
	"fmt"
	"github.com/olusolaa/goforai/foundation/gemini"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/olusolaa/goforai/foundation/chromemdb"

	"github.com/cloudwego/eino-ext/components/document/loader/file"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/compose"
	chromem "github.com/philippgille/chromem-go"
)

var db *chromem.DB

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	fmt.Println("🚀 GopherCon Knowledge Indexing with Eino")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Using Eino's document processing pipeline:")
	fmt.Println("  FileLoader → ChromemIndexer")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n🔧 Building indexing graph...")
	runner, err := buildIndexingGraph(ctx)
	if err != nil {
		return fmt.Errorf("failed to build indexing graph: %w", err)
	}

	docsDir := "./foundation/indexing/gophercon-docs"
	fmt.Printf("\n📖 Processing markdown files from: %s\n\n", docsDir)

	fileCount := 0
	chunkCount := 0

	err = filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk dir failed: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		fileCount++
		fmt.Printf("  [processing] %s\n", filepath.Base(path))

		ids, err := runner.Invoke(ctx, document.Source{URI: path})
		if err != nil {
			return fmt.Errorf("failed to index %s: %w", path, err)
		}

		chunkCount += len(ids)
		fmt.Printf("  [✓ done] indexed %d chunks from %s\n\n", len(ids), filepath.Base(path))

		return nil
	})

	if err != nil {
		return err
	}

	fmt.Println("💾 Persisting database to disk...")
	dbPath := "data/chromem.gob"
	if err := chromemdb.ExportDB(db, dbPath); err != nil {
		return fmt.Errorf("failed to export database: %w", err)
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("✅ Indexing complete!\n")
	fmt.Printf("   Files: %d markdown files → %d chunks\n", fileCount, chunkCount)
	fmt.Printf("   💾 Saved to: %s\n", dbPath)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("\n🎯 Next step: Run the agent")
	fmt.Println("   cd ../../ && GEMINI_API_KEY=xxx ./bin/agent")

	return nil
}

func buildIndexingGraph(ctx context.Context) (compose.Runnable[document.Source, []string], error) {
	embedder, err := gemini.NewEmbedder(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	db = chromem.NewDB()

	chromemIndexer, err := chromemdb.New(ctx, "gophercon-knowledge", embedder, chromemdb.WithDB(db))
	if err != nil {
		return nil, fmt.Errorf("failed to create chromem indexer: %w", err)
	}

	g := compose.NewGraph[document.Source, []string]()

	fileLoader, err := file.NewFileLoader(ctx, &file.FileLoaderConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create file loader: %w", err)
	}
	_ = g.AddLoaderNode("FileLoader", fileLoader)

	// Simple document pass-through (no splitting needed for small docs)
	// For production, use a proper text splitter
	_ = g.AddIndexerNode("ChromemIndexer", chromemIndexer)

	_ = g.AddEdge(compose.START, "FileLoader")
	_ = g.AddEdge("FileLoader", "ChromemIndexer")
	_ = g.AddEdge("ChromemIndexer", compose.END)

	r, err := g.Compile(ctx, compose.WithGraphName("KnowledgeIndexing"))
	if err != nil {
		return nil, fmt.Errorf("failed to compile graph: %w", err)
	}

	return r, nil
}
