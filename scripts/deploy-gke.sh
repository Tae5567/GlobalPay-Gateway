#!/bin/bash
# scripts/deploy-gke.sh - Deploy GlobalPay to GKE

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ID=${GCP_PROJECT_ID:-"your-project-id"}
CLUSTER_NAME=${CLUSTER_NAME:-"globalpay-production"}
REGION=${REGION:-"us-central1"}
ZONE=${ZONE:-"us-central1-a"}
NAMESPACE=${NAMESPACE:-"globalpay"}
ENVIRONMENT=${ENVIRONMENT:-"production"}

echo -e "${GREEN}🚀 GlobalPay Gateway Deployment Script${NC}"
echo "Project: $PROJECT_ID"
echo "Cluster: $CLUSTER_NAME"
echo "Environment: $ENVIRONMENT"
echo ""

# Check prerequisites
check_prerequisites() {
    echo -e "${YELLOW}📋 Checking prerequisites...${NC}"
    
    command -v gcloud >/dev/null 2>&1 || { echo -e "${RED}❌ gcloud CLI not installed${NC}"; exit 1; }
    command -v kubectl >/dev/null 2>&1 || { echo -e "${RED}❌ kubectl not installed${NC}"; exit 1; }
    command -v helm >/dev/null 2>&1 || { echo -e "${RED}❌ helm not installed${NC}"; exit 1; }
    
    echo -e "${GREEN}✅ All prerequisites met${NC}"
}

# Authenticate with GCP
authenticate_gcp() {
    echo -e "${YELLOW}🔐 Authenticating with GCP...${NC}"
    
    gcloud config set project $PROJECT_ID
    gcloud auth configure-docker
    
    echo -e "${GREEN}✅ Authenticated${NC}"
}

# Create or get existing cluster
setup_cluster() {
    echo -e "${YELLOW}☸️  Setting up GKE cluster...${NC}"
    
    # Check if cluster exists
    if gcloud container clusters describe $CLUSTER_NAME --zone=$ZONE 2>/dev/null; then
        echo "Cluster already exists, using existing cluster"
    else
        echo "Creating new GKE cluster..."
        gcloud container clusters create $CLUSTER_NAME \
            --zone=$ZONE \
            --num-nodes=3 \
            --machine-type=e2-standard-2 \
            --disk-size=20 \
            --enable-autoscaling \
            --min-nodes=3 \
            --max-nodes=10 \
            --enable-autorepair \
            --enable-autoupgrade \
            --enable-ip-alias \
            --network=default \
            --subnetwork=default \
            --enable-stackdriver-kubernetes \
            --addons=HorizontalPodAutoscaling,HttpLoadBalancing,GcePersistentDiskCsiDriver
    fi
    
    # Get cluster credentials
    gcloud container clusters get-credentials $CLUSTER_NAME --zone=$ZONE
    
    echo -e "${GREEN}✅ Cluster ready${NC}"
}

# Build and push Docker images
build_and_push_images() {
    echo -e "${YELLOW}🐳 Building and pushing Docker images...${NC}"
    
    SERVICES=("payment-gateway" "currency-conversion" "fraud-detection" "transaction-ledger")
    
    for service in "${SERVICES[@]}"; do
        echo "Building $service..."
        docker build -t gcr.io/$PROJECT_ID/$service:latest \
            --build-arg SERVICE_NAME=$service \
            -f services/$service/Dockerfile .
        
        echo "Pushing $service..."
        docker push gcr.io/$PROJECT_ID/$service:latest
    done
    
    echo -e "${GREEN}✅ All images built and pushed${NC}"
}

# Setup Kubernetes namespace and secrets
setup_namespace() {
    echo -e "${YELLOW}📦 Setting up Kubernetes namespace...${NC}"
    
    kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
    
    echo -e "${GREEN}✅ Namespace created${NC}"
}

# Create secrets
create_secrets() {
    echo -e "${YELLOW}🔒 Creating secrets...${NC}"
    
    # Prompt for secrets if not in environment
    if [ -z "$DATABASE_URL" ]; then
        read -sp "Enter PostgreSQL URL: " DATABASE_URL
        echo
    fi
    
    if [ -z "$STRIPE_SECRET_KEY" ]; then
        read -sp "Enter Stripe Secret Key: " STRIPE_SECRET_KEY
        echo
    fi
    
    if [ -z "$EXCHANGE_RATE_API_KEY" ]; then
        read -sp "Enter Exchange Rate API Key: " EXCHANGE_RATE_API_KEY
        echo
    fi
    
    # Create secret
    kubectl create secret generic globalpay-secrets \
        --from-literal=database-url="$DATABASE_URL" \
        --from-literal=stripe-secret-key="$STRIPE_SECRET_KEY" \
        --from-literal=exchange-rate-api-key="$EXCHANGE_RATE_API_KEY" \
        --namespace=$NAMESPACE \
        --dry-run=client -o yaml | kubectl apply -f -
    
    echo -e "${GREEN}✅ Secrets created${NC}"
}

# Install dependencies with Helm
install_dependencies() {
    echo -e "${YELLOW}📚 Installing dependencies...${NC}"
    
    # Add Helm repositories
    helm repo add bitnami https://charts.bitnami.com/bitnami
    helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
    helm repo add jaegertracing https://jaegertracing.github.io/helm-charts
    helm repo update
    
    # Install PostgreSQL
    echo "Installing PostgreSQL..."
    helm upgrade --install postgresql bitnami/postgresql \
        --namespace=$NAMESPACE \
        --set auth.database=globalpay \
        --set primary.persistence.size=20Gi \
        --wait
    
    # Install Redis
    echo "Installing Redis..."
    helm upgrade --install redis bitnami/redis \
        --namespace=$NAMESPACE \
        --set master.persistence.size=10Gi \
        --wait
    
    # Install MongoDB
    echo "Installing MongoDB..."
    helm upgrade --install mongodb bitnami/mongodb \
        --namespace=$NAMESPACE \
        --set persistence.size=10Gi \
        --wait
    
    # Install Kafka
    echo "Installing Kafka..."
    helm upgrade --install kafka bitnami/kafka \
        --namespace=$NAMESPACE \
        --set persistence.size=20Gi \
        --set replicaCount=3 \
        --wait
    
    echo -e "${GREEN}✅ Dependencies installed${NC}"
}

# Install monitoring stack
install_monitoring() {
    echo -e "${YELLOW}📊 Installing monitoring stack...${NC}"
    
    # Prometheus + Grafana
    helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
        --namespace=$NAMESPACE \
        --set prometheus.prometheusSpec.retention=30d \
        --set prometheus.prometheusSpec.storageSpec.volumeClaimTemplate.spec.resources.requests.storage=50Gi \
        --wait
    
    # Jaeger
    helm upgrade --install jaeger jaegertracing/jaeger \
        --namespace=$NAMESPACE \
        --wait
    
    echo -e "${GREEN}✅ Monitoring stack installed${NC}"
}

# Deploy application with Helm
deploy_application() {
    echo -e "${YELLOW}🚢 Deploying GlobalPay application...${NC}"
    
    helm upgrade --install globalpay ./infrastructure/helm/globalpay \
        --namespace=$NAMESPACE \
        --set global.imageRegistry=gcr.io/$PROJECT_ID \
        --set global.environment=$ENVIRONMENT \
        --values=./infrastructure/helm/globalpay/values-${ENVIRONMENT}.yaml \
        --wait \
        --timeout=10m
    
    echo -e "${GREEN}✅ Application deployed${NC}"
}

# Verify deployment
verify_deployment() {
    echo -e "${YELLOW}🔍 Verifying deployment...${NC}"
    
    echo "Checking pod status..."
    kubectl get pods -n $NAMESPACE
    
    echo ""
    echo "Checking services..."
    kubectl get svc -n $NAMESPACE
    
    echo ""
    echo "Waiting for all pods to be ready..."
    kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=globalpay -n $NAMESPACE --timeout=300s
    
    echo -e "${GREEN}✅ Deployment verified${NC}"
}

# Run smoke tests
run_smoke_tests() {
    echo -e "${YELLOW}🧪 Running smoke tests...${NC}"
    
    # Port forward to payment gateway
    kubectl port-forward -n $NAMESPACE svc/payment-gateway 8080:80 &
    PF_PID=$!
    
    sleep 5
    
    # Health check
    if curl -f http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Health check passed${NC}"
    else
        echo -e "${RED}❌ Health check failed${NC}"
        kill $PF_PID
        exit 1
    fi
    
    # Kill port forward
    kill $PF_PID
    
    echo -e "${GREEN}✅ Smoke tests passed${NC}"
}

# Print access information
print_access_info() {
    echo ""
    echo -e "${GREEN}🎉 Deployment Complete!${NC}"
    echo ""
    echo "Access your services:"
    echo ""
    echo "Payment Gateway:"
    echo "  kubectl port-forward -n $NAMESPACE svc/payment-gateway 8080:80"
    echo "  Then access: http://localhost:8080"
    echo ""
    echo "Grafana Dashboard:"
    echo "  kubectl port-forward -n $NAMESPACE svc/prometheus-grafana 3000:80"
    echo "  Then access: http://localhost:3000"
    echo "  Default credentials: admin / prom-operator"
    echo ""
    echo "Jaeger UI:"
    echo "  kubectl port-forward -n $NAMESPACE svc/jaeger-query 16686:16686"
    echo "  Then access: http://localhost:16686"
    echo ""
    echo "Get External IP (once LoadBalancer is provisioned):"
    echo "  kubectl get svc -n $NAMESPACE payment-gateway"
    echo ""
}

# Main execution
main() {
    check_prerequisites
    authenticate_gcp
    setup_cluster
    build_and_push_images
    setup_namespace
    create_secrets
    install_dependencies
    install_monitoring
    deploy_application
    verify_deployment
    run_smoke_tests
    print_access_info
}

# Run main function
main