.PHONY: help build build-all test lint clean dev-dev-up dev-down proto

help:
	@echo "VeritasVPN Makefile"
	@echo "==================="
	@echo "make dev-up       - Start development environment (Docker Compose)"
	@echo "make dev-down     - Stop development environment"
	@echo "make build-all    - Build all Go services"
	@echo "make build-auth   - Build auth-svc"
	@echo "make build-wg     - Build wg-manager"
	@echo "make build-billing- Build billing-svc"
	@echo "make build-agent  - Build veritas-agent"
	@echo "make build-cli    - Build CLI client"
	@echo "make test         - Run all tests"
	@echo "make lint         - Run linter"
	@echo "make clean        - Clean build artifacts"
	@echo "make proto        - Generate proto Go stubs"

BUILD_DIR ?= $(CURDIR)/build

dev-up:
	docker compose up -d
	@echo "Services starting at:"
	@echo "  auth-svc:   :8081"
	@echo "  wg-manager: :8082"
	@echo "  billing-svc::8083"
	@echo "  postgres:   :5432"
	@echo "  redis:      :6379"
	@echo "  nats:       :4222 (monitoring :8222)"

dev-down:
	docker compose down

dev-logs:
	docker compose logs -f

build-all: build-auth build-wg build-billing build-agent build-cli

build-auth:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/auth-svc ./services/auth-svc/cmd/server/
	@echo "Built auth-svc -> $(BUILD_DIR)/auth-svc"

build-wg:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/wg-manager ./services/wg-manager/cmd/server/
	@echo "Built wg-manager -> $(BUILD_DIR)/wg-manager"

build-billing:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/billing-svc ./services/billing-svc/cmd/server/
	@echo "Built billing-svc -> $(BUILD_DIR)/billing-svc"

build-agent:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/veritas-agent ./services/veritas-agent/cmd/agent/
	@echo "Built veritas-agent -> $(BUILD_DIR)/veritas-agent"

build-cli:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/veritas ./clients/cli/cmd/
	@echo "Built CLI client -> $(BUILD_DIR)/veritas"

test:
	go test ./lib/... ./services/... ./clients/...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)

proto:
	buf generate api/
	@echo "Proto stubs generated"

go-mod-tidy:
	cd lib/config && go mod tidy
	cd lib/logging && go mod tidy
	cd lib/crypto && go mod tidy
	cd lib/jwt && go mod tidy
	cd services/auth-svc && go mod tidy
	cd services/wg-manager && go mod tidy
	cd services/billing-svc && go mod tidy
	cd services/veritas-agent && go mod tidy
