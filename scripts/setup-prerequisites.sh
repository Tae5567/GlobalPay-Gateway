#!/bin/bash
# setup-prerequisites.sh - Install all required tools

set -e

echo "🚀 Setting up GlobalPay Gateway Development Environment"

# Detect OS
OS="$(uname -s)"

install_go() {
    echo "📦 Installing Go 1.21..."
    if command -v go &> /dev/null; then
        echo "✅ Go already installed: $(go version)"
    else
        case "$OS" in
            Linux*)
                wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
                sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
                echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
                ;;
            Darwin*)
                brew install go@1.21
                ;;
        esac
    fi
    
    # For macOS, ensure Go is in PATH for this session
    if [[ "$OS" == "Darwin"* ]]; then
        export PATH="/opt/homebrew/opt/go@1.21/bin:$PATH"
        export GOPATH="$HOME/go"
        export PATH="$PATH:$GOPATH/bin"
    fi
}

install_docker() {
    echo "🐳 Checking Docker..."
    if command -v docker &> /dev/null; then
        echo "✅ Docker already installed: $(docker --version)"
    else
        echo "❌ Please install Docker Desktop from https://www.docker.com/products/docker-desktop"
        exit 1
    fi
}

install_kubectl() {
    echo "☸️  Installing kubectl..."
    if command -v kubectl &> /dev/null; then
        echo "✅ kubectl already installed: $(kubectl version --client)"
    else
        case "$OS" in
            Linux*)
                curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
                sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
                ;;
            Darwin*)
                brew install kubectl
                ;;
        esac
    fi
}

install_helm() {
    echo "⎈ Installing Helm..."
    if command -v helm &> /dev/null; then
        echo "✅ Helm already installed: $(helm version --short)"
    else
        curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
    fi
}

install_gcloud() {
    echo "☁️  Installing Google Cloud SDK..."
    if command -v gcloud &> /dev/null; then
        echo "✅ gcloud already installed"
    else
        case "$OS" in
            Linux*)
                curl https://sdk.cloud.google.com | bash
                exec -l $SHELL
                ;;
            Darwin*)
                brew install --cask google-cloud-sdk
                ;;
        esac
    fi
}

install_protoc() {
    echo "🔧 Installing Protocol Buffers..."
    if command -v protoc &> /dev/null; then
        echo "✅ protoc already installed: $(protoc --version)"
    else
        case "$OS" in
            Linux*)
                curl -LO https://github.com/protocolbuffers/protobuf/releases/download/v24.4/protoc-24.4-linux-x86_64.zip
                unzip protoc-24.4-linux-x86_64.zip -d $HOME/.local
                echo 'export PATH="$PATH:$HOME/.local/bin"' >> ~/.bashrc
                ;;
            Darwin*)
                brew install protobuf
                ;;
        esac
        
        # Verify go is available before installing Go tools
        if command -v go &> /dev/null; then
            echo "Installing Go Protocol Buffer plugins..."
            go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
            go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
        else
            echo "⚠️  Go not found in PATH. Skipping protoc-gen-go installation."
            echo "   Run these commands manually after sourcing your shell:"
            echo "   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
            echo "   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
        fi
    fi
}

install_tools() {
    echo "🛠️  Installing additional tools..."
    
    # k6 for load testing
    if ! command -v k6 &> /dev/null; then
        case "$OS" in
            Linux*)
                sudo gpg -k
                sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
                echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
                sudo apt-get update
                sudo apt-get install k6
                ;;
            Darwin*)
                brew install k6
                ;;
        esac
    fi
    
    # golangci-lint for code quality
    if ! command -v golangci-lint &> /dev/null; then
        if command -v go &> /dev/null; then
            curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2
        else
            echo "⚠️  Skipping golangci-lint installation (Go not in PATH)"
        fi
    fi
}

setup_env() {
    echo "📝 Creating environment files..."
    
    cat > .env.local << 'EOF'
# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=globalpay
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# MongoDB
MONGO_HOST=localhost
MONGO_PORT=27017
MONGO_DB=globalpay_fraud

# Kafka
KAFKA_BROKERS=localhost:9092

# Stripe (Test Mode)
STRIPE_SECRET_KEY=sk_test_your_key_here
STRIPE_PUBLISHABLE_KEY=pk_test_your_key_here

# Exchange Rate API
EXCHANGE_RATE_API_KEY=your_api_key_here

# Service Ports
PAYMENT_GATEWAY_PORT=8080
CURRENCY_SERVICE_PORT=8081
FRAUD_SERVICE_PORT=8082
LEDGER_SERVICE_PORT=8083

# Observability
JAEGER_ENDPOINT=http://localhost:14268/api/traces
PROMETHEUS_PORT=9090
EOF

    echo "✅ Created .env.local"
}

# Run installations
install_go
install_docker
install_kubectl
install_helm
install_gcloud
install_protoc
install_tools
setup_env

echo ""
echo "✨ Setup complete! Next steps:"
echo "1. Add Go to your PATH permanently:"
echo "   echo 'export PATH=\"/opt/homebrew/opt/go@1.21/bin:\$PATH\"' >> ~/.zshrc"
echo "   echo 'export GOPATH=\"\$HOME/go\"' >> ~/.zshrc"
echo "   echo 'export PATH=\"\$PATH:\$GOPATH/bin\"' >> ~/.zshrc"
echo "   source ~/.zshrc"
echo ""
echo "2. Install Go tools (run after step 1):"
echo "   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
echo "   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
echo ""
echo "3. Initialize GKE: gcloud auth login && gcloud init"
echo "4. Get API keys:"
echo "   - Stripe: https://dashboard.stripe.com/test/apikeys"
echo "   - Exchange Rate: https://www.exchangerate-api.com/"
echo "5. Update .env.local with your API keys"
echo "6. Run: make setup-local"