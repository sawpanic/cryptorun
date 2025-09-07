# CryptoRun Makefile - Build, Test, and Database Operations
# Supports Windows, Linux, and macOS development environments

.PHONY: help build test lint clean dbtest dbup dbdown dbmigrate docker deps check

# Default target
help: ## Show this help
	@echo "CryptoRun v3.2.1 - Build and Development Commands"
	@echo ""
	@echo "Build Commands:"
	@echo "  build       - Build cryptorun binary for development"
	@echo "  build-release - Build cryptorun with release flags and buildstamp"
	@echo "  clean       - Clean build artifacts"
	@echo ""
	@echo "Test Commands:"
	@echo "  test        - Run all tests"
	@echo "  test-count  - Run tests with count=1 (recommended before PRs)"
	@echo "  test-unit   - Run unit tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  lint        - Run golangci-lint"
	@echo ""
	@echo "Database Commands:"
	@echo "  dbtest      - Run tests with PostgreSQL (spins up container)"
	@echo "  dbup        - Start PostgreSQL container for development"
	@echo "  dbdown      - Stop PostgreSQL container"
	@echo "  dbmigrate   - Apply database migrations"
	@echo ""
	@echo "Development Commands:"
	@echo "  deps        - Install/update dependencies"
	@echo "  check       - Run full quality checks (build, lint, test)"
	@echo "  docker      - Build Docker image"

# Build commands
build: ## Build development binary
	go build -o cryptorun.exe ./cmd/cryptorun

build-release: ## Build release binary with buildstamp
	go run ./tools/buildstamp
	@$(eval STAMP := $(shell cat .buildstamp 2>/dev/null || echo "unknown"))
	go build -ldflags "-X main.BuildStamp=$(STAMP)" -o cryptorun.exe ./cmd/cryptorun

clean: ## Clean build artifacts
	rm -f cryptorun.exe cryptorun
	rm -f .buildstamp
	go clean -cache -testcache -modcache

# Test commands
test: ## Run all tests
	go test ./...

test-count: ## Run tests with count=1 (recommended before PRs)
	go test ./... -count=1

test-unit: ## Run unit tests only
	go test ./tests/unit/...

test-integration: ## Run integration tests only
	go test ./tests/integration/...

lint: ## Run golangci-lint
	golangci-lint run ./...

# Database commands
dbtest: dbup ## Run tests with PostgreSQL
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 5
	$(MAKE) dbmigrate
	@echo "Running tests with database..."
	PG_DSN="postgres://cryptorun:cryptorun@localhost:5432/cryptorun_test?sslmode=disable" go test ./internal/persistence/... -count=1
	$(MAKE) dbdown

dbup: ## Start PostgreSQL container for development
	@echo "Starting PostgreSQL container..."
	docker run -d \
		--name cryptorun-postgres \
		-p 5432:5432 \
		-e POSTGRES_DB=cryptorun_test \
		-e POSTGRES_USER=cryptorun \
		-e POSTGRES_PASSWORD=cryptorun \
		-e POSTGRES_INITDB_ARGS="--encoding=UTF-8 --lc-collate=C --lc-ctype=C" \
		--rm \
		postgres:15-alpine
	@echo "PostgreSQL container started on port 5432"
	@echo "Connection: postgres://cryptorun:cryptorun@localhost:5432/cryptorun_test"

dbdown: ## Stop PostgreSQL container
	@echo "Stopping PostgreSQL container..."
	-docker stop cryptorun-postgres
	@echo "PostgreSQL container stopped"

dbmigrate: ## Apply database migrations
	@echo "Applying database migrations..."
	@if command -v psql >/dev/null 2>&1; then \
		export PGPASSWORD=cryptorun; \
		for file in db/migrations/*.sql; do \
			echo "Applying $$file..."; \
			psql -h localhost -p 5432 -U cryptorun -d cryptorun_test -f "$$file"; \
		done; \
	else \
		echo "psql not found. Install PostgreSQL client or use Docker:"; \
		echo "docker exec -i cryptorun-postgres psql -U cryptorun -d cryptorun_test < db/migrations/0001_create_trades.sql"; \
		echo "docker exec -i cryptorun-postgres psql -U cryptorun -d cryptorun_test < db/migrations/0002_create_regime_snapshots.sql"; \
		echo "docker exec -i cryptorun-postgres psql -U cryptorun -d cryptorun_test < db/migrations/0003_create_premove_artifacts.sql"; \
	fi

# Development commands
deps: ## Install/update dependencies
	go mod tidy
	go mod download
	go mod verify

check: deps lint test ## Run full quality checks
	@echo "✅ All quality checks passed"

docker: ## Build Docker image
	docker build -t cryptorun:latest .

# Database migration helpers
.PHONY: dbmigrate-docker
dbmigrate-docker: ## Apply migrations using Docker (no local psql required)
	@echo "Applying migrations via Docker..."
	@for file in db/migrations/*.sql; do \
		echo "Applying $$file..."; \
		docker exec -i cryptorun-postgres psql -U cryptorun -d cryptorun_test < "$$file"; \
	done

# Quick development workflow
.PHONY: dev-setup dev-test
dev-setup: deps dbup dbmigrate ## Setup development environment
	@echo "✅ Development environment ready"

dev-test: test-count lint ## Quick test and lint for development
	@echo "✅ Development checks passed"

# CI/CD helpers
.PHONY: ci-test ci-build
ci-test: ## CI test target
	go test ./... -race -cover -count=1

ci-build: ## CI build target
	go build -race ./cmd/cryptorun
