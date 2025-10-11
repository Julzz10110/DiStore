#!/bin/bash

# Kubernetes Configuration Validation Script
# Validates Distore Kubernetes configurations

set -e

echo "🔍 Validating Kubernetes configurations for Distore"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    print_error "kubectl is not installed or not in PATH"
    exit 1
fi

# Check if we can connect to a cluster
if ! kubectl cluster-info &> /dev/null; then
    print_warning "Cannot connect to Kubernetes cluster. Running client-side validation only."
    DRY_RUN_MODE="client"
else
    print_success "Connected to Kubernetes cluster. Running server-side validation."
    DRY_RUN_MODE="server"
fi

# Validate operator YAML
print_status "Validating operator configuration..."
if kubectl apply --dry-run=$DRY_RUN_MODE -f k8s/operator.yaml; then
    print_success "Operator configuration is valid"
else
    print_error "Operator configuration validation failed"
    exit 1
fi

# Validate deployment configurations
print_status "Validating deployment configurations..."
for manifest in deployments/*.yaml; do
    if [ -f "$manifest" ]; then
        if kubectl apply --dry-run=$DRY_RUN_MODE -f "$manifest"; then
            print_success "Deployment configuration is valid: $(basename $manifest)"
        else
            print_error "Deployment configuration validation failed: $(basename $manifest)"
            exit 1
        fi
    fi
done

# Validate CRD schema
if [ "$DRY_RUN_MODE" = "server" ]; then
    print_status "Validating CRD schema..."
    
    # Apply CRD temporarily
    kubectl apply -f k8s/operator.yaml
    
    # Wait for CRD to be ready
    kubectl wait --for condition=established --timeout=60s crd/distoreclusters.distore.io
    
    # Test valid configuration
    cat <<EOF | kubectl apply --dry-run=$DRY_RUN_MODE -f -
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: validation-test
  namespace: default
spec:
  replicas: 3
  image: distore/distore:latest
  multiCloud:
    enabled: true
    dataCenters:
      - id: "dc1"
        region: "us-east-1"
        nodes: ["node1:8080", "node2:8080", "node3:8080"]
        priority: 1
        replicaCount: 3
    edgeNodes:
      - id: "edge1"
        location: "New York"
        node: "edge1:8080"
        cacheOnly: true
        latencyMs: 20
    latencyThresholds:
      local: 10
      crossDC: 100
      edge: 50
    cloudProviders:
      - name: "aws"
        region: "us-east-1"
        nodes: ["aws-node1:8080", "aws-node2:8080"]
EOF
    
    if [ $? -eq 0 ]; then
        print_success "CRD schema validation passed"
    else
        print_error "CRD schema validation failed"
        exit 1
    fi
    
    # Clean up CRD
    kubectl delete crd distoreclusters.distore.io --ignore-not-found=true
fi

# Validate YAML syntax
print_status "Validating YAML syntax..."
if command -v yamllint &> /dev/null; then
    for file in k8s/*.yaml deployments/*.yaml; do
        if [ -f "$file" ]; then
            if yamllint "$file"; then
                print_success "YAML syntax is valid: $(basename $file)"
            else
                print_warning "YAML syntax issues found in: $(basename $file)"
            fi
        fi
    done
else
    print_warning "yamllint not found. Skipping YAML syntax validation."
fi

# Check for common issues
print_status "Checking for common configuration issues..."

# Check for hardcoded values
if grep -r "localhost\|127.0.0.1" k8s/ deployments/ &> /dev/null; then
    print_warning "Found hardcoded localhost addresses"
fi

# Check for missing resource limits
if ! grep -r "resources:" k8s/ deployments/ &> /dev/null; then
    print_warning "No resource limits found in configurations"
fi

# Check for security contexts
if ! grep -r "securityContext:" k8s/ deployments/ &> /dev/null; then
    print_warning "No security contexts found in configurations"
fi

print_success "Configuration validation completed!"
print_status "Summary:"
print_status "- Operator configuration: ✓"
print_status "- Deployment configurations: ✓"
if [ "$DRY_RUN_MODE" = "server" ]; then
    print_status "- CRD schema validation: ✓"
fi
print_status "- Common issues check: ✓"

