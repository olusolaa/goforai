# GopherCon Africa 2025 - Building AI Agents with Eino
# Root Makefile (Bill Kennedy Style)

# ==============================================================================
# Auto-load .env file if it exists

ifneq (,$(wildcard .env))
    include .env
    export
endif

# ==============================================================================
# Environment Check

.PHONY: check-env
check-env:
	@if [ -z "$$GEMINI_API_KEY" ]; then \
		echo "❌ Error: GEMINI_API_KEY is not set"; \
		echo "   Run: export GEMINI_API_KEY='your-api-key'"; \
		echo "   Or create .env file from .env.example"; \
		exit 1; \
	fi
	@echo "✅ Environment ready (GEMINI_API_KEY set)"

# ==============================================================================
# Setup

.PHONY: setup
setup: check-env
	@echo "📦 Creating GopherCon Africa knowledge base..."
	@go run ./example02/cmd/indexing
	@echo "✅ Knowledge base ready: ./data/chromem.gob"

# ==============================================================================
# Example01 - Progressive Learning Steps (Bill Kennedy Style)

.PHONY: step1
step1: check-env
	go run example01/step1/main.go

.PHONY: step2
step2: check-env
	go run example01/step2/main.go

.PHONY: step3
step3: check-env
	@if [ ! -f "data/chromem.gob" ]; then \
		echo "⚠️  Running setup first..."; \
		make setup; \
	fi
	go run example01/step3/main.go

.PHONY: step4
step4: check-env
	go run example01/step4/main.go

.PHONY: step5
step5: check-env
	@if [ ! -f "data/chromem.gob" ]; then \
		echo "⚠️  Running setup first..."; \
		make setup; \
	fi
	go run example01/step5/main.go

# ==============================================================================
# Example02 - Production Apps

.PHONY: cli
cli: check-env
	@if [ ! -f "data/chromem.gob" ]; then \
		echo "⚠️  Running setup first..."; \
		make setup; \
	fi
	go run ./example02/cmd/cli

.PHONY: web
web: check-env
	@if [ ! -f "data/chromem.gob" ]; then \
		echo "⚠️  Running setup first..."; \
		make setup; \
	fi
	@echo "🌐 Starting web server on http://localhost:8080"
	go run ./example02/cmd/web

.PHONY: indexing
indexing: check-env
	go run ./example02/cmd/indexing

# ==============================================================================
# Presentation Shortcuts

.PHONY: demo
demo: step5

.PHONY: demo-rag
demo-rag: step3

.PHONY: demo-tools
demo-tools: step4

.PHONY: demo-full
demo-full: step5

# ==============================================================================
# Testing

.PHONY: test-steps
test-steps: check-env setup
	@echo "🧪 Testing all presentation steps..."
	@echo ""
	@echo "Testing Step 3 (RAG)..."
	@make step3
	@echo ""
	@echo "Testing Step 4 (Tools)..."
	@make step4
	@echo ""
	@echo "✅ All steps work!"

# ==============================================================================
# Utilities

.PHONY: clean
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf data/chromem.gob
	@rm -rf data/repos/
	@echo "✅ Cleaned"

.PHONY: deps
deps:
	@echo "📦 Downloading dependencies..."
	@go mod download
	@echo "✅ Dependencies ready"

# ==============================================================================
# Help

.PHONY: help
help:
	@echo ""
	@echo "╔═══════════════════════════════════════════════════════════╗"
	@echo "║  GopherCon Africa 2025 - AI Coding Agent                 ║"
	@echo "╚═══════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "🎬 PRESENTATION (type these during your 20-min talk!):"
	@echo ""
	@echo "  make setup          Create knowledge base (run first!)"
	@echo "  make step3          Demo: RAG in isolation"
	@echo "  make step4          Demo: Tools in isolation"
	@echo "  make step5          Demo: Full coding agent ⭐"
	@echo ""
	@echo "🚀 PRODUCTION APPS:"
	@echo ""
	@echo "  make cli            Run CLI coding agent"
	@echo "  make web            Run web interface (http://localhost:8080)"
	@echo ""
	@echo "🛠️  UTILITIES:"
	@echo ""
	@echo "  make check-env      Verify GEMINI_API_KEY is set"
	@echo "  make test-steps     Test all presentation steps"
	@echo "  make clean          Remove generated files"
	@echo "  make deps           Download Go dependencies"
	@echo ""
	@echo "📚 Learn more: cat README.md"
	@echo ""

.DEFAULT_GOAL := help

