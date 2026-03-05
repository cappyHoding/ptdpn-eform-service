# BPR Perdana E-Form Backend — Makefile
# Run these commands from the project root directory.
#
# Usage:
#   make run        → Start the server (development mode)
#   make build      → Compile the binary
#   make test       → Run all tests
#   make keys       → Generate RSA key pair for JWT

# ── Configuration ─────────────────────────────────────────────────────────────
BINARY_NAME=eform-backend
BINARY_PATH=./bin/$(BINARY_NAME)
MAIN_PATH=./cmd/server/main.go

# ── Development ───────────────────────────────────────────────────────────────

# Run the server in development mode (reads .env file)
.PHONY: run
run:
	go run $(MAIN_PATH)

# Build the binary for the current OS
.PHONY: build
build:
	go build -o $(BINARY_PATH) $(MAIN_PATH)
	@echo "Binary built: $(BINARY_PATH)"

# Build for Linux (for deployment from Windows)
.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_PATH)-linux $(MAIN_PATH)
	@echo "Linux binary built: $(BINARY_PATH)-linux"

# ── Dependencies ──────────────────────────────────────────────────────────────

# Download all dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy
	@echo "Dependencies updated"

# ── Testing ───────────────────────────────────────────────────────────────────

# Run all tests
.PHONY: test
test:
	go test ./... -v

# Run tests with coverage report
.PHONY: test-coverage
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ── Security Keys ─────────────────────────────────────────────────────────────

# Generate RSA key pair for JWT signing
# Run this ONCE per environment (dev, staging, production) and store keys securely
.PHONY: keys
keys:
	@mkdir -p keys
	openssl genrsa -out keys/private.pem 2048
	openssl rsa -in keys/private.pem -pubout -out keys/public.pem
	@echo "Keys generated in ./keys/"
	@echo "IMPORTANT: Add ./keys/ to .gitignore — NEVER commit these files"

# ── Database ──────────────────────────────────────────────────────────────────

# Run all migrations in order
.PHONY: migrate
migrate:
	@echo "Running migrations..."
	@for file in migrations/*.sql; do \
		echo "  Running $$file..."; \
		mysql -u $${DB_USER} -p$${DB_PASSWORD} -h $${DB_HOST} $${DB_NAME} < $$file; \
	done
	@echo "Migrations complete"

# ── Docker ────────────────────────────────────────────────────────────────────

# Build Docker image
.PHONY: docker-build
docker-build:
	docker build -t bpr-perdana-eform:latest .

# Start all services (MySQL, Redis, app)
.PHONY: docker-up
docker-up:
	docker compose up -d

# Stop all services
.PHONY: docker-down
docker-down:
	docker compose down

# ── Utilities ─────────────────────────────────────────────────────────────────

# Format all Go code
.PHONY: fmt
fmt:
	go fmt ./...

# Run linter
.PHONY: lint
lint:
	golangci-lint run ./...

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf ./bin/
	rm -f coverage.out coverage.html