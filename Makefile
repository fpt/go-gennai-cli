run: ## Run the application
	go run ./gennai

run-anthropic: ## Run the application with Anthropic backend
	go run ./gennai -b anthropic

run-openai: ## Run the application with OpenAI backend
	go run ./gennai -b openai

run-gemini: ## Run the application with Gemini backend
	go run ./gennai -b gemini

build: ## Build the application
	go build -o output/gennai ./gennai

install: ## Install the application
	go install ./gennai

test: ## Run tests
	go test ./...

lint: ## Run linters
	golangci-lint run

fmt: ## Format code
	gofmt -s -w .

fix: ## Fix code issues
	golangci-lint run --fix

integ: build ## Matrix integration test (testcases × backends)
	CLI=output/gennai ./testsuite/matrix_runner.sh

test-capabilities: ## Capability testing
	go build -o output/test-capabilities ./cmd/test-capabilities
	./output/test-capabilities

zip: ## Create a minimal zip archive of source files (excludes build outputs and .gennai)
	@echo "Creating minimal source archive..."
	@mkdir -p output
	zip -r output/gennai-source.zip . \
		-x "output/*" \
		-x ".gennai/*" \
		-x "*.zip" \
		-x ".git/*" \
		-x "*.log" \
		-x "*.tmp" \
		-x "*~" \
		-x ".DS_Store" \
		-x ".claude/*" \
		-x "gennai" \
		-x "testsuite/results/*"
	@echo "Archive created: output/gennai-source.zip"
	@echo "Archive size: $$(du -h output/gennai-source.zip | cut -f1)"

help: ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "%-20s %s\n", $$1, $$2}'
