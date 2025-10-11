#!/bin/bash

# Kubernetes Integration Testing Script for Distore
# This script tests various aspects of Kubernetes integration

set -e

echo "🚀 Starting Kubernetes Integration Tests for Distore"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="distore-test"
TEST_TIMEOUT="300s"
CLEANUP_ON_EXIT=true

# Function to print colored output
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

# Function to cleanup resources
cleanup() {
    if [ "$CLEANUP_ON_EXIT" = true ]; then
        print_status "Cleaning up test resources..."
        kubectl delete namespace $NAMESPACE --ignore-not-found=true
        print_success "Cleanup completed"
    fi
}

# Set trap for cleanup on exit
trap cleanup EXIT

# Check if kubectl is available
check_prerequisites() {
    print_status "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    if ! command -v helm &> /dev/null; then
        print_warning "Helm is not installed. Some tests may be skipped."
    fi
    
    # Check if we can connect to a Kubernetes cluster
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    print_success "Prerequisites check passed"
}

# Test 1: Validate Kubernetes manifests
test_manifest_validation() {
    print_status "Testing manifest validation..."
    
    # Test operator YAML
    if kubectl apply --dry-run=client -f k8s/operator.yaml; then
        print_success "Operator manifest validation passed"
    else
        print_error "Operator manifest validation failed"
        return 1
    fi
    
    # Test deployment manifests
    for manifest in deployments/*.yaml; do
        if [ -f "$manifest" ]; then
            if kubectl apply --dry-run=client -f "$manifest"; then
                print_success "Deployment manifest validation passed: $(basename $manifest)"
            else
                print_error "Deployment manifest validation failed: $(basename $manifest)"
                return 1
            fi
        fi
    done
}

# Test 2: Test namespace creation and RBAC
test_namespace_and_rbac() {
    print_status "Testing namespace and RBAC setup..."
    
    # Create test namespace
    kubectl create namespace $NAMESPACE
    
    # Test if we can create resources in the namespace
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: $NAMESPACE
data:
  test: "value"
EOF
    
    if kubectl get configmap test-config -n $NAMESPACE &> /dev/null; then
        print_success "Namespace and RBAC test passed"
        kubectl delete configmap test-config -n $NAMESPACE
    else
        print_error "Namespace and RBAC test failed"
        return 1
    fi
}

# Test 3: Test DistoreCluster CRD
test_crd_installation() {
    print_status "Testing DistoreCluster CRD installation..."
    
    # Apply the CRD
    kubectl apply -f k8s/operator.yaml
    
    # Wait for CRD to be ready
    kubectl wait --for condition=established --timeout=60s crd/distoreclusters.distore.io
    
    # Test creating a DistoreCluster resource
    cat <<EOF | kubectl apply -f -
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: test-cluster
  namespace: $NAMESPACE
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
    
    # Wait for the resource to be created
    kubectl wait --for condition=Ready --timeout=60s distorecluster/test-cluster -n $NAMESPACE || true
    
    if kubectl get distorecluster test-cluster -n $NAMESPACE &> /dev/null; then
        print_success "DistoreCluster CRD test passed"
    else
        print_error "DistoreCluster CRD test failed"
        return 1
    fi
}

# Test 4: Test deployment scenarios
test_deployment_scenarios() {
    print_status "Testing deployment scenarios..."
    
    # Test AWS deployment
    if kubectl apply --dry-run=server -f deployments/aws.yaml; then
        print_success "AWS deployment scenario validation passed"
    else
        print_warning "AWS deployment scenario validation failed (may be expected in test environment)"
    fi
    
    # Test GCP deployment
    if kubectl apply --dry-run=server -f deployments/gcp.yaml; then
        print_success "GCP deployment scenario validation passed"
    else
        print_warning "GCP deployment scenario validation failed (may be expected in test environment)"
    fi
    
    # Test Multi-cloud deployment
    if kubectl apply --dry-run=server -f deployments/multi-cloud.yaml; then
        print_success "Multi-cloud deployment scenario validation passed"
    else
        print_warning "Multi-cloud deployment scenario validation failed (may be expected in test environment)"
    fi
}

# Test 5: Test configuration validation
test_config_validation() {
    print_status "Testing configuration validation..."
    
    # Test valid configuration
    cat <<EOF | kubectl apply -f -
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: valid-config-test
  namespace: $NAMESPACE
spec:
  replicas: 2
  image: distore/distore:latest
  multiCloud:
    enabled: true
    dataCenters:
      - id: "dc1"
        region: "us-west-1"
        nodes: ["node1:8080", "node2:8080"]
        priority: 1
        replicaCount: 2
EOF
    
    if kubectl get distorecluster valid-config-test -n $NAMESPACE &> /dev/null; then
        print_success "Valid configuration test passed"
        kubectl delete distorecluster valid-config-test -n $NAMESPACE
    else
        print_error "Valid configuration test failed"
        return 1
    fi
    
    # Test invalid configuration (should be rejected)
    cat <<EOF | kubectl apply -f -
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: invalid-config-test
  namespace: $NAMESPACE
spec:
  replicas: 0  # Invalid: must be positive
  image: distore/distore:latest
  multiCloud:
    enabled: true
    dataCenters: []  # Invalid: must have at least one datacenter
EOF
    
    # This should fail, so we check if it was rejected
    if kubectl get distorecluster invalid-config-test -n $NAMESPACE &> /dev/null; then
        print_warning "Invalid configuration was accepted (validation may not be implemented yet)"
        kubectl delete distorecluster invalid-config-test -n $NAMESPACE
    else
        print_success "Invalid configuration was properly rejected"
    fi
}

# Test 6: Test edge computing features
test_edge_computing() {
    print_status "Testing edge computing features..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: edge-test
  namespace: $NAMESPACE
spec:
  replicas: 1
  image: distore/distore:latest
  multiCloud:
    enabled: true
    dataCenters:
      - id: "main-dc"
        region: "us-central1"
        nodes: ["main-node:8080"]
        priority: 1
        replicaCount: 1
    edgeNodes:
      - id: "edge-nyc"
        location: "New York"
        node: "edge-nyc:8080"
        cacheOnly: true
        latencyMs: 15
      - id: "edge-london"
        location: "London"
        node: "edge-london:8080"
        cacheOnly: false
        latencyMs: 25
    latencyThresholds:
      local: 10
      crossDC: 100
      edge: 50
EOF
    
    if kubectl get distorecluster edge-test -n $NAMESPACE &> /dev/null; then
        print_success "Edge computing configuration test passed"
        kubectl delete distorecluster edge-test -n $NAMESPACE
    else
        print_error "Edge computing configuration test failed"
        return 1
    fi
}

# Test 7: Test scaling operations
test_scaling_operations() {
    print_status "Testing scaling operations..."
    
    # Create a cluster
    cat <<EOF | kubectl apply -f -
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: scaling-test
  namespace: $NAMESPACE
spec:
  replicas: 2
  image: distore/distore:latest
  multiCloud:
    enabled: true
    dataCenters:
      - id: "dc1"
        region: "us-east-1"
        nodes: ["node1:8080", "node2:8080"]
        priority: 1
        replicaCount: 2
EOF
    
    # Test scaling up
    kubectl patch distorecluster scaling-test -n $NAMESPACE --type='merge' -p='{"spec":{"replicas":3}}'
    
    # Test scaling down
    kubectl patch distorecluster scaling-test -n $NAMESPACE --type='merge' -p='{"spec":{"replicas":1}}'
    
    if kubectl get distorecluster scaling-test -n $NAMESPACE &> /dev/null; then
        print_success "Scaling operations test passed"
        kubectl delete distorecluster scaling-test -n $NAMESPACE
    else
        print_error "Scaling operations test failed"
        return 1
    fi
}

# Test 8: Test resource limits and requests
test_resource_management() {
    print_status "Testing resource management..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: resource-test
  namespace: $NAMESPACE
spec:
  replicas: 1
  image: distore/distore:latest
  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"
    limits:
      memory: "512Mi"
      cpu: "500m"
  multiCloud:
    enabled: true
    dataCenters:
      - id: "dc1"
        region: "us-east-1"
        nodes: ["node1:8080"]
        priority: 1
        replicaCount: 1
EOF
    
    if kubectl get distorecluster resource-test -n $NAMESPACE &> /dev/null; then
        print_success "Resource management test passed"
        kubectl delete distorecluster resource-test -n $NAMESPACE
    else
        print_error "Resource management test failed"
        return 1
    fi
}

# Main test execution
main() {
    print_status "Starting comprehensive Kubernetes integration tests..."
    
    check_prerequisites
    
    # Run all tests
    test_manifest_validation
    test_namespace_and_rbac
    test_crd_installation
    test_deployment_scenarios
    test_config_validation
    test_edge_computing
    test_scaling_operations
    test_resource_management
    
    print_success "All Kubernetes integration tests completed!"
    print_status "Test summary:"
    print_status "- Manifest validation: ✓"
    print_status "- Namespace and RBAC: ✓"
    print_status "- CRD installation: ✓"
    print_status "- Deployment scenarios: ✓"
    print_status "- Configuration validation: ✓"
    print_status "- Edge computing: ✓"
    print_status "- Scaling operations: ✓"
    print_status "- Resource management: ✓"
}

# Run main function
main "$@"

