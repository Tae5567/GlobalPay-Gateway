#!/bin/bash
# scripts/setup-local.sh - Complete local development setup

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}🚀 GlobalPay Gateway - Local Setup${NC}"

# Step 1: Check prerequisites
echo -e "\n${YELLOW}Step 1: Checking prerequisites...${NC}"
command -v go >/dev/null 2>&1 || { echo -e "${RED}❌ Go is not installed${NC}"; exit 1; }
command -v docker >/dev/null 2>&1 || { echo -e "${RED}❌ Docker is not installed${NC}"; exit 1; }
command -v docker-compose >/dev/null 2>&1 || { echo -e "${RED}❌ Docker Compose is not installed${NC}"; exit 1; }
echo -e "${GREEN}✅ All prerequisites met${NC}"

# Step 2: Create project directories
echo -e "\n${YELLOW}Step 2: Creating project structure...${NC}"
mkdir -p services/{payment-gateway,currency-conversion,fraud-detection,transaction-ledger}/{cmd/server,internal/{handler,service,repository,models},pkg}
mkdir -p shared/pkg/{logger,middleware,database,redis,tracing,metrics}
mkdir -p infrastructure/{k8s,helm,terraform}
mkdir -p tests/{e2e,integration,performance}
mkdir -p docs/{architecture,adr,api}
mkdir -p scripts
echo -e "${GREEN}✅ Directories created${NC}"

# Step 3: Initialize Go modules
echo -e "\n${YELLOW}Step 3: Initializing Go modules...${NC}"
cd shared
if [ ! -f "go.mod" ]; then
    go mod init shared
fi
go mod tidy
cd ..

for service in payment-gateway currency-conversion fraud-detection transaction-ledger; do
    cd services/$service
    if [ ! -f "go.mod" ]; then
        go mod init $service
        echo -e "\nreplace shared => ../../shared" >> go.mod
    fi
    go mod tidy
    cd ../..
done
echo -e "${GREEN}✅ Go modules initialized${NC}"

# Step 4: Download dependencies
echo -e "\n${YELLOW}Step 4: Downloading dependencies...${NC}"
cd shared
go get -u github.com/gin-gonic/gin
go get -u github.com/google/uuid
go get -u github.com/lib/pq
go get -u github.com/go-redis/redis/v8
go get -u go.uber.org/zap
go get -u github.com/prometheus/client_golang/prometheus
go get -u google.golang.org/grpc
go get -u google.golang.org/protobuf
cd ..

for service in payment-gateway currency-conversion fraud-detection transaction-ledger; do
    echo "Downloading deps for $service..."
    cd services/$service
    go get -u github.com/gin-gonic/gin
    go get -u github.com/google/uuid
    go get -u github.com/lib/pq
    go get -u go.uber.org/zap
    go get -u github.com/prometheus/client_golang/prometheus
    
    if [ "$service" == "payment-gateway" ]; then
        go get -u github.com/stripe/stripe-go/v76
    fi
    
    if [ "$service" == "fraud-detection" ]; then
        go get -u go.mongodb.org/mongo-driver
    fi
    
    cd ../..
done
echo -e "${GREEN}✅ Dependencies downloaded${NC}"

# Step 5: Create environment file
echo -e "\n${YELLOW}Step 5: Creating environment configuration...${NC}"
if [ ! -f ".env.local" ]; then
    cat > .env.local << 'EOF'
# Database Configuration
DATABASE_URL=postgres://postgres:postgres@localhost:5432/globalpay?sslmode=disable
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=globalpay
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres

# Redis Configuration
REDIS_URL=localhost:6379
REDIS_HOST=localhost
REDIS_PORT=6379

# MongoDB Configuration
MONGO_URL=mongodb://admin:admin@localhost:27017
MONGO_HOST=localhost
MONGO_PORT=27017
MONGO_DB=globalpay_fraud
MONGO_USER=admin
MONGO_PASSWORD=admin

# Kafka Configuration
KAFKA_BROKERS=localhost:9092

# API Keys (Replace with your keys)
STRIPE_SECRET_KEY=sk_test_your_stripe_key_here
STRIPE_PUBLISHABLE_KEY=pk_test_your_stripe_key_here
EXCHANGE_RATE_API_KEY=your_exchange_rate_api_key_here

# Service Ports
PAYMENT_GATEWAY_PORT=8080
CURRENCY_SERVICE_PORT=8081
FRAUD_SERVICE_PORT=8082
LEDGER_SERVICE_PORT=8083

# Observability
JAEGER_ENDPOINT=http://localhost:14268/api/traces
PROMETHEUS_PORT=9090

# Environment
ENVIRONMENT=development
LOG_LEVEL=debug
EOF
    echo -e "${GREEN}✅ Created .env.local${NC}"
    echo -e "${YELLOW}⚠️  Please update .env.local with your API keys${NC}"
else
    echo -e "${GREEN}✅ .env.local already exists${NC}"
fi

# Step 6: Create database init script
echo -e "\n${YELLOW}Step 6: Creating database initialization script...${NC}"
mkdir -p infrastructure/scripts
cat > infrastructure/scripts/init-db.sql << 'EOF'
-- Create payments table
CREATE TABLE IF NOT EXISTS payments (
    id VARCHAR(36) PRIMARY KEY,
    amount DECIMAL(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(20) NOT NULL,
    card_last4 VARCHAR(4),
    card_network VARCHAR(20),
    customer_email VARCHAR(255),
    description TEXT,
    stripe_payment_intent_id VARCHAR(255),
    client_secret TEXT,
    requires_3ds BOOLEAN DEFAULT FALSE,
    idempotency_key VARCHAR(255) UNIQUE,
    failure_reason TEXT,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP
);

CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_customer_email ON payments(customer_email);
CREATE INDEX idx_payments_created_at ON payments(created_at);

-- Create exchange rates table
CREATE TABLE IF NOT EXISTS exchange_rates (
    id SERIAL PRIMARY KEY,
    from_currency VARCHAR(3) NOT NULL,
    to_currency VARCHAR(3) NOT NULL,
    rate DECIMAL(19, 6) NOT NULL,
    source VARCHAR(100),
    timestamp TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_exchange_rates_currencies ON exchange_rates(from_currency, to_currency);
CREATE INDEX idx_exchange_rates_timestamp ON exchange_rates(timestamp);

-- Create conversions table
CREATE TABLE IF NOT EXISTS conversions (
    id VARCHAR(36) PRIMARY KEY,
    from_currency VARCHAR(3) NOT NULL,
    to_currency VARCHAR(3) NOT NULL,
    original_amount DECIMAL(19, 4) NOT NULL,
    converted_amount DECIMAL(19, 4) NOT NULL,
    exchange_rate DECIMAL(19, 6) NOT NULL,
    fee DECIMAL(19, 4) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create ledger tables
CREATE TABLE IF NOT EXISTS ledger_transactions (
    id VARCHAR(36) PRIMARY KEY,
    description TEXT NOT NULL,
    payment_id VARCHAR(36),
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id VARCHAR(36) PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL REFERENCES ledger_transactions(id),
    account_id VARCHAR(100) NOT NULL,
    type VARCHAR(10) NOT NULL,
    amount DECIMAL(19, 4) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ledger_entries_transaction ON ledger_entries(transaction_id);
CREATE INDEX idx_ledger_entries_account ON ledger_entries(account_id);

-- Create fraud check results table
CREATE TABLE IF NOT EXISTS fraud_check_results (
    id VARCHAR(36) PRIMARY KEY,
    transaction_id VARCHAR(36) NOT NULL,
    score INT NOT NULL,
    risk_level VARCHAR(20) NOT NULL,
    decision VARCHAR(20) NOT NULL,
    flags TEXT[],
    processing_ms BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fraud_results_transaction ON fraud_check_results(transaction_id);
CREATE INDEX idx_fraud_results_risk_level ON fraud_check_results(risk_level);

GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO postgres;
EOF
echo -e "${GREEN}✅ Database init script created${NC}"

# Step 7: Start infrastructure services
echo -e "\n${YELLOW}Step 7: Starting infrastructure services with Docker Compose...${NC}"
cd infrastructure
docker-compose up -d
echo "Waiting for services to be ready..."
sleep 15

# Check if services are healthy
echo "Checking service health..."
docker-compose ps

echo -e "${GREEN}✅ Infrastructure services started${NC}"
cd ..

# Step 8: Run database migrations
echo -e "\n${YELLOW}Step 8: Running database migrations...${NC}"
export PGPASSWORD=postgres
psql -h localhost -U postgres -d globalpay -f infrastructure/scripts/init-db.sql 2>/dev/null || echo "Database already initialized"
echo -e "${GREEN}✅ Database migrations completed${NC}"

# Step 9: Build all services
echo -e "\n${YELLOW}Step 9: Building all services...${NC}"
for service in payment-gateway currency-conversion fraud-detection transaction-ledger; do
    echo "Building $service..."
    cd services/$service
    go build -o bin/server ./cmd/server
    cd ../..
done
echo -e "${GREEN}✅ All services built${NC}"

# Step 10: Create start script
echo -e "\n${YELLOW}Step 10: Creating service start scripts...${NC}"
cat > scripts/start-all-services.sh << 'EOF'
#!/bin/bash
# Start all services

echo "Starting GlobalPay Gateway services..."

# Load environment
set -a
source .env.local
set +a

# Start services in background
echo "Starting Payment Gateway on port 8080..."
cd services/payment-gateway && ./bin/server > ../../logs/payment-gateway.log 2>&1 &
echo $! > ../../.pids/payment-gateway.pid

sleep 2

echo "Starting Currency Service on port 8081..."
cd ../currency-conversion && PORT=8081 ./bin/server > ../../logs/currency-service.log 2>&1 &
echo $! > ../../.pids/currency-service.pid

sleep 2

echo "Starting Fraud Detection on port 8082..."
cd ../fraud-detection && PORT=8082 ./bin/server > ../../logs/fraud-service.log 2>&1 &
echo $! > ../../.pids/fraud-service.pid

sleep 2

echo "Starting Transaction Ledger on port 8083..."
cd ../transaction-ledger && PORT=8083 ./bin/server > ../../logs/ledger-service.log 2>&1 &
echo $! > ../../.pids/ledger-service.pid

cd ../..

echo ""
echo "✅ All services started!"
echo ""
echo "Service URLs:"
echo "  Payment Gateway:  http://localhost:8080"
echo "  Currency Service: http://localhost:8081"
echo "  Fraud Detection:  http://localhost:8082"
echo "  Ledger Service:   http://localhost:8083"
echo ""
echo "Health checks:"
echo "  curl http://localhost:8080/health"
echo "  curl http://localhost:8081/health"
echo "  curl http://localhost:8082/health"
echo "  curl http://localhost:8083/health"
echo ""
echo "View logs:"
echo "  tail -f logs/payment-gateway.log"
echo ""
echo "Stop all services:"
echo "  ./scripts/stop-all-services.sh"
EOF

chmod +x scripts/start-all-services.sh

cat > scripts/stop-all-services.sh << 'EOF'
#!/bin/bash
# Stop all services

echo "Stopping GlobalPay Gateway services..."

if [ -f .pids/payment-gateway.pid ]; then
    kill $(cat .pids/payment-gateway.pid) 2>/dev/null
    rm .pids/payment-gateway.pid
fi

if [ -f .pids/currency-service.pid ]; then
    kill $(cat .pids/currency-service.pid) 2>/dev/null
    rm .pids/currency-service.pid
fi

if [ -f .pids/fraud-service.pid ]; then
    kill $(cat .pids/fraud-service.pid) 2>/dev/null
    rm .pids/fraud-service.pid
fi

if [ -f .pids/ledger-service.pid ]; then
    kill $(cat .pids/ledger-service.pid) 2>/dev/null
    rm .pids/ledger-service.pid
fi

echo "✅ All services stopped"
EOF

chmod +x scripts/stop-all-services.sh

mkdir -p logs .pids

echo -e "${GREEN}✅ Start/stop scripts created${NC}"

# Final summary
echo -e "\n${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✨ Local setup complete!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "Next steps:"
echo ""
echo "1. Update API keys in .env.local:"
echo "   - Get Stripe keys from: https://dashboard.stripe.com/test/apikeys"
echo "   - Get Exchange Rate API key from: https://www.exchangerate-api.com/"
echo ""
echo "2. Start all services:"
echo "   ./scripts/start-all-services.sh"
echo ""
echo "3. Test the services:"
echo "   curl http://localhost:8080/health"
echo ""
echo "4. View infrastructure:"
echo "   - Grafana: http://localhost:3000 (admin/admin)"
echo "   - Prometheus: http://localhost:9090"
echo "   - Jaeger: http://localhost:16686"
echo ""
echo "5. Run tests:"
echo "   make test"
echo ""
echo "6. Stop services:"
echo "   ./scripts/stop-all-services.sh"
echo ""
echo "7. Stop infrastructure:"
echo "   cd infrastructure && docker-compose down"
echo ""