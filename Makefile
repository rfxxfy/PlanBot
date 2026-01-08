.PHONY: help build run test clean docker-build docker-up docker-down docker-logs db-setup db-migrate db-reset lint fmt vet install dev

# Variables
APP_NAME=planbot
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GOFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Colors for output
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m
COLOR_BLUE=\033[34m

help: ## Show this help message
	@echo "$(COLOR_BOLD)PlanBot - Available commands:$(COLOR_RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_GREEN)%-15s$(COLOR_RESET) %s\n", $$1, $$2}'
	@echo ""

# Build commands
build: ## Build the application
	@echo "$(COLOR_BLUE)Building $(APP_NAME)...$(COLOR_RESET)"
	@go build $(GOFLAGS) -o $(APP_NAME) main.go
	@echo "$(COLOR_GREEN)✓ Build complete: ./$(APP_NAME)$(COLOR_RESET)"

build-linux: ## Build for Linux
	@echo "$(COLOR_BLUE)Building for Linux...$(COLOR_RESET)"
	@GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o $(APP_NAME)-linux main.go
	@echo "$(COLOR_GREEN)✓ Linux build complete$(COLOR_RESET)"

build-all: ## Build for all platforms
	@echo "$(COLOR_BLUE)Building for all platforms...$(COLOR_RESET)"
	@GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o bin/$(APP_NAME)-linux-amd64 main.go
	@GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -o bin/$(APP_NAME)-darwin-amd64 main.go
	@GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -o bin/$(APP_NAME)-darwin-arm64 main.go
	@GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o bin/$(APP_NAME)-windows-amd64.exe main.go
	@echo "$(COLOR_GREEN)✓ All builds complete$(COLOR_RESET)"

# Run commands
run: ## Run the application locally
	@echo "$(COLOR_BLUE)Running $(APP_NAME)...$(COLOR_RESET)"
	@go run main.go

dev: ## Run in development mode with auto-reload (requires air)
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "$(COLOR_YELLOW)air not found. Install with: go install github.com/cosmtrek/air@latest$(COLOR_RESET)"; \
		echo "Running without hot reload..."; \
		go run main.go; \
	fi

install: ## Install dependencies
	@echo "$(COLOR_BLUE)Installing dependencies...$(COLOR_RESET)"
	@go mod download
	@go mod tidy
	@echo "$(COLOR_GREEN)✓ Dependencies installed$(COLOR_RESET)"

# Docker commands
docker-build: ## Build Docker image
	@echo "$(COLOR_BLUE)Building Docker image...$(COLOR_RESET)"
	@docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .
	@echo "$(COLOR_GREEN)✓ Docker image built$(COLOR_RESET)"

docker-up: ## Start all services with Docker Compose
	@echo "$(COLOR_BLUE)Starting services...$(COLOR_RESET)"
	@docker-compose up -d
	@echo "$(COLOR_GREEN)✓ Services started$(COLOR_RESET)"
	@echo "Run 'make docker-logs' to view logs"

docker-up-dev: ## Start services including Adminer (dev profile)
	@echo "$(COLOR_BLUE)Starting services (dev mode)...$(COLOR_RESET)"
	@docker-compose --profile dev up -d
	@echo "$(COLOR_GREEN)✓ Services started$(COLOR_RESET)"
	@echo "Adminer available at http://localhost:8081"

docker-down: ## Stop all services
	@echo "$(COLOR_BLUE)Stopping services...$(COLOR_RESET)"
	@docker-compose down
	@echo "$(COLOR_GREEN)✓ Services stopped$(COLOR_RESET)"

docker-restart: docker-down docker-up ## Restart all services

docker-logs: ## Show logs from all services
	@docker-compose logs -f

docker-logs-bot: ## Show bot logs only
	@docker-compose logs -f bot

docker-logs-db: ## Show database logs only
	@docker-compose logs -f postgres

docker-clean: ## Remove containers, volumes, and images
	@echo "$(COLOR_YELLOW)Cleaning Docker resources...$(COLOR_RESET)"
	@docker-compose down -v
	@docker rmi $(APP_NAME):latest $(APP_NAME):$(VERSION) 2>/dev/null || true
	@echo "$(COLOR_GREEN)✓ Docker resources cleaned$(COLOR_RESET)"

# Database commands
db-setup: ## Create database and apply schema
	@echo "$(COLOR_BLUE)Setting up database...$(COLOR_RESET)"
	@createdb planbot 2>/dev/null || echo "Database already exists"
	@psql planbot < database/schema.sql
	@echo "$(COLOR_GREEN)✓ Database setup complete$(COLOR_RESET)"

db-migrate: ## Apply database migrations
	@echo "$(COLOR_BLUE)Applying migrations...$(COLOR_RESET)"
	@psql planbot < database/migrations.sql
	@echo "$(COLOR_GREEN)✓ Migrations applied$(COLOR_RESET)"

db-reset: ## Drop and recreate database
	@echo "$(COLOR_YELLOW)Resetting database...$(COLOR_RESET)"
	@dropdb planbot 2>/dev/null || true
	@createdb planbot
	@psql planbot < database/schema.sql
	@echo "$(COLOR_GREEN)✓ Database reset complete$(COLOR_RESET)"

db-test-data: ## Load test data
	@echo "$(COLOR_BLUE)Loading test data...$(COLOR_RESET)"
	@psql planbot < database/test_data.sql
	@echo "$(COLOR_GREEN)✓ Test data loaded$(COLOR_RESET)"

db-shell: ## Open PostgreSQL shell
	@psql planbot

# Code quality commands
lint: ## Run linter
	@echo "$(COLOR_BLUE)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "$(COLOR_YELLOW)golangci-lint not found. Install from: https://golangci-lint.run/usage/install/$(COLOR_RESET)"; \
	fi

fmt: ## Format code
	@echo "$(COLOR_BLUE)Formatting code...$(COLOR_RESET)"
	@go fmt ./...
	@echo "$(COLOR_GREEN)✓ Code formatted$(COLOR_RESET)"

vet: ## Run go vet
	@echo "$(COLOR_BLUE)Running go vet...$(COLOR_RESET)"
	@go vet ./...
	@echo "$(COLOR_GREEN)✓ No issues found$(COLOR_RESET)"

test: ## Run tests
	@echo "$(COLOR_BLUE)Running tests...$(COLOR_RESET)"
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "$(COLOR_GREEN)✓ Tests complete$(COLOR_RESET)"

test-coverage: test ## Run tests with coverage report
	@go tool cover -html=coverage.out

# Utility commands
clean: ## Clean build artifacts
	@echo "$(COLOR_BLUE)Cleaning...$(COLOR_RESET)"
	@rm -f $(APP_NAME) $(APP_NAME)-linux $(APP_NAME)-*.exe
	@rm -rf bin/
	@rm -f coverage.out
	@echo "$(COLOR_GREEN)✓ Cleaned$(COLOR_RESET)"

check: fmt vet lint ## Run all checks

health: ## Check application health
	@curl -s http://localhost:8080/health | jq . || echo "Application not running or jq not installed"

version: ## Show version
	@echo "$(COLOR_BOLD)Version:$(COLOR_RESET) $(VERSION)"
	@echo "$(COLOR_BOLD)Build Time:$(COLOR_RESET) $(BUILD_TIME)"

# Combined commands
all: clean install fmt vet build ## Clean, install deps, format, vet, and build

deploy-prep: clean test build-linux ## Prepare for deployment

# Default target
.DEFAULT_GOAL := help

