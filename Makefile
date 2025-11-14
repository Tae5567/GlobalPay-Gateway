# Makefile for GlobalPay Gateway

.PHONY: help
help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Variables
PROJECT_ID ?= your-gcp-project-id
CLUSTER_NAME ?= globalpay-cluster
REGION ?= us-central1
ZONE ?= us-central1-a
NAMESPACE ?= globalpay

SERVICES := payment-gateway currency-conversion fraud-detection transaction-ledger
GO_VERSION := 1.21

# Setup
.PHONY: setup-local
setup-local: ## Setup local development environment
	@echo "Setting up local environment..."
	./scripts/setup-local.sh
	docker-compose -f infrastructure/docker-compose.yml up -d
	@echo "Waiting for services to start..."
	sleep 10
	$(MAKE) db-migrate
	@echo "✅ Local environment ready!"

.PHONY: setup-gcp
setup-gcp: ## Setup GCP project and GKE cluster
	@echo "Setting up GCP infrastructure..."
	gcloud config set project $(PROJECT_ID)
	gcloud services enable container.googleapis.com
	gcloud services enable containerregistry.googleapis.com
	gcloud services enable cloudbuild.googleapis.com
	$(MAKE) create-cluster

.PHONY: create-cluster
create-cluster: ## Create GKE cluster
	gcloud container clusters create $(CLUSTER_NAME) \
		--zone=$(ZONE) \
		--num-nodes=3 \
		--machine-type=e2-medium \
		--disk-size=20 \
		--enable-autoscaling \
		--min-nodes=3 \
		--max-nodes=10 \
		--enable-autorepair \
		--enable-autoupgrade \
		--enable-ip-alias \
		--network=default \
		--subnetwork=default \
		--enable-stackdriver-kubernetes
	gcloud container clusters get-credentials $(CLUSTER_NAME) --zone=$(ZONE)

# Development
.PHONY: proto
proto: ## Generate protobuf code
	./scripts/generate-proto.sh

.PHONY: dev
dev: ## Run services in development mode
	docker-compose -f infrastructure/docker-compose.yml up

.PHONY: dev-down
dev-down: ## Stop development environment
	docker-compose -f infrastructure/docker-compose.yml down

# Testing
.PHONY: test
test: ## Run unit tests
	@echo "Running unit tests..."
	@for service in $(SERVICES); do \
		echo "Testing $$service..."; \
		cd services/$$service && go test -v -race -coverprofile=coverage.out ./... || exit 1; \
		cd ../..; \
	done
	cd shared && go test -v -race -coverprofile=coverage.out ./...

.PHONY: test-coverage
test-coverage: test ## Generate test coverage report
	@echo "Generating coverage report..."
	@for service in $(SERVICES); do \
		cd services/$$service && go tool cover -html=coverage.out -o coverage.html; \
		cd ../..; \
	done

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v -tags=integration ./tests/integration/...

.PHONY: test-e2e
test-e2e: ## Run E2E tests
	@echo "Running E2E tests..."
	go test -v ./tests/e2e/...

.PHONY: test-performance
test-performance: ## Run performance tests with k6
	k6 run tests/performance/k6-script.js

# Code Quality
.PHONY: lint
lint: ## Run linters
	golangci-lint run --timeout=5m ./...

.PHONY: format
format: ## Format code
	@echo "Formatting code..."
	gofmt -s -w .
	goimports -w .

.PHONY: vet
vet: ## Run go vet
	@for service in $(SERVICES); do \
		echo "Vetting $$service..."; \
		cd services/$$service && go vet ./...; \
		cd ../..; \
	done

# Build
.PHONY: build
build: ## Build all services
	@echo "Building services..."
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		cd services/$$service && CGO_ENABLED=0 go build -o bin/server ./cmd/server; \
		cd ../..; \
	done

.PHONY: docker-build
docker-build: ## Build Docker images
	@echo "Building Docker images..."
	@for service in $(SERVICES); do \
		echo "Building $$service image..."; \
		docker build -t gcr.io/$(PROJECT_ID)/$$service:latest \
			--build-arg SERVICE_NAME=$$service \
			-f services/$$service/Dockerfile .; \
	done

.PHONY: docker-push
docker-push: ## Push Docker images to GCR
	@echo "Pushing images to GCR..."
	@for service in $(SERVICES); do \
		echo "Pushing $$service..."; \
		docker push gcr.io/$(PROJECT_ID)/$$service:latest; \
	done

# Database
.PHONY: db-migrate
db-migrate: ## Run database migrations
	@echo "Running migrations..."
	go run cmd/migrate/main.go up

.PHONY: db-rollback
db-rollback: ## Rollback last migration
	go run cmd/migrate/main.go down

.PHONY: db-reset
db-reset: ## Reset database
	docker-compose -f infrastructure/docker-compose.yml down -v
	docker-compose -f infrastructure/docker-compose.yml up -d postgres
	sleep 5
	$(MAKE) db-migrate

# Kubernetes
.PHONY: k8s-deploy
k8s-deploy: ## Deploy to Kubernetes using Helm
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install globalpay ./infrastructure/helm/globalpay \
		--namespace=$(NAMESPACE) \
		--set global.imageRegistry=gcr.io/$(PROJECT_ID) \
		--wait

.PHONY: k8s-delete
k8s-delete: ## Delete Kubernetes deployment
	helm uninstall globalpay --namespace=$(NAMESPACE)
	kubectl delete namespace $(NAMESPACE)

.PHONY: k8s-status
k8s-status: ## Show Kubernetes deployment status
	kubectl get all -n $(NAMESPACE)

.PHONY: k8s-logs
k8s-logs: ## Tail logs from payment-gateway
	kubectl logs -f -l app=payment-gateway -n $(NAMESPACE)

.PHONY: k8s-port-forward
k8s-port-forward: ## Port forward to payment gateway
	kubectl port-forward -n $(NAMESPACE) svc/payment-gateway 8080:80

# Monitoring
.PHONY: monitoring-install
monitoring-install: ## Install monitoring stack
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo add grafana https://grafana.github.io/helm-charts
	helm repo update
	helm install prometheus prometheus-community/kube-prometheus-stack -n $(NAMESPACE)
	helm install jaeger jaegertracing/jaeger -n $(NAMESPACE)

.PHONY: monitoring-dashboard
monitoring-dashboard: ## Open Grafana dashboard
	kubectl port-forward -n $(NAMESPACE) svc/prometheus-grafana 3000:80

# Utilities
.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	@for service in $(SERVICES); do \
		rm -rf services/$$service/bin; \
		rm -f services/$$service/coverage.out; \
	done
	docker-compose -f infrastructure/docker-compose.yml down -v

.PHONY: deps
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@for service in $(SERVICES); do \
		cd services/$$service && go mod download && go mod tidy; \
		cd ../..; \
	done
	cd shared && go mod download && go mod tidy

.PHONY: update-deps
update-deps: ## Update dependencies
	@echo "Updating dependencies..."
	@for service in $(SERVICES); do \
		cd services/$$service && go get -u ./...; \
		cd ../..; \
	done

.PHONY: generate-docs
generate-docs: ## Generate API documentation
	swag init -g cmd/server/main.go -o docs/api

# Quick Commands
.PHONY: run-payment-gateway
run-payment-gateway: ## Run payment gateway locally
	cd services/payment-gateway && go run cmd/server/main.go

.PHONY: curl-health
curl-health: ## Check health endpoints
	@echo "Payment Gateway:"
	@curl -s http://localhost:8080/health | jq .
	@echo "\nCurrency Service:"
	@curl -s http://localhost:8081/health | jq .
	@echo "\nFraud Service:"
	@curl -s http://localhost:8082/health | jq .
	@echo "\nLedger Service:"
	@curl -s http://localhost:8083/health | jq .

# CI/CD
.PHONY: ci
ci: lint test test-integration ## Run full CI pipeline locally
	@echo "✅ CI checks passed!"

.PHONY: pre-commit
pre-commit: format lint test ## Run pre-commit checks
	@echo "✅ Pre-commit checks passed!"