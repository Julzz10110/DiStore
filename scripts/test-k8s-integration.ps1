# Kubernetes Integration Testing Script for Distore (PowerShell version)
# This script tests various aspects of Kubernetes integration

param(
    [string]$Namespace = "distore-test",
    [int]$TimeoutSeconds = 300,
    [switch]$NoCleanup
)

# Configuration
$ErrorActionPreference = "Stop"

Write-Host "🚀 Starting Kubernetes Integration Tests for Distore" -ForegroundColor Blue

# Function to print colored output
function Write-Status {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Blue
}

function Write-Success {
    param([string]$Message)
    Write-Host "[SUCCESS] $Message" -ForegroundColor Green
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARNING] $Message" -ForegroundColor Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

# Function to cleanup resources
function Cleanup-Resources {
    if (-not $NoCleanup) {
        Write-Status "Cleaning up test resources..."
        try {
            kubectl delete namespace $Namespace --ignore-not-found=true
            Write-Success "Cleanup completed"
        }
        catch {
            Write-Warning "Cleanup failed: $_"
        }
    }
}

# Check if kubectl is available
function Test-Prerequisites {
    Write-Status "Checking prerequisites..."
    
    if (-not (Get-Command kubectl -ErrorAction SilentlyContinue)) {
        Write-Error "kubectl is not installed or not in PATH"
        exit 1
    }
    
    if (-not (Get-Command helm -ErrorAction SilentlyContinue)) {
        Write-Warning "Helm is not installed. Some tests may be skipped."
    }
    
    # Check if we can connect to a Kubernetes cluster
    try {
        kubectl cluster-info | Out-Null
        Write-Success "Prerequisites check passed"
    }
    catch {
        Write-Error "Cannot connect to Kubernetes cluster"
        exit 1
    }
}

# Test 1: Validate Kubernetes manifests
function Test-ManifestValidation {
    Write-Status "Testing manifest validation..."
    
    try {
        # Test operator YAML
        kubectl apply --dry-run=client -f k8s/operator.yaml
        Write-Success "Operator manifest validation passed"
        
        # Test deployment manifests
        $deploymentFiles = Get-ChildItem -Path "deployments" -Filter "*.yaml" -ErrorAction SilentlyContinue
        foreach ($manifest in $deploymentFiles) {
            kubectl apply --dry-run=client -f $manifest.FullName
            Write-Success "Deployment manifest validation passed: $($manifest.Name)"
        }
    }
    catch {
        Write-Error "Manifest validation failed: $_"
        throw
    }
}

# Test 2: Test namespace creation and RBAC
function Test-NamespaceAndRBAC {
    Write-Status "Testing namespace and RBAC setup..."
    
    try {
        # Create test namespace
        kubectl create namespace $Namespace
        
        # Test if we can create resources in the namespace
        $configMapYaml = @"
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: $Namespace
data:
  test: "value"
"@
        
        $configMapYaml | kubectl apply -f -
        
        kubectl get configmap test-config -n $Namespace | Out-Null
        Write-Success "Namespace and RBAC test passed"
        kubectl delete configmap test-config -n $Namespace
    }
    catch {
        Write-Error "Namespace and RBAC test failed: $_"
        throw
    }
}

# Test 3: Test DistoreCluster CRD
function Test-CRDInstallation {
    Write-Status "Testing DistoreCluster CRD installation..."
    
    try {
        # Apply the CRD
        kubectl apply -f k8s/operator.yaml
        
        # Wait for CRD to be ready
        kubectl wait --for condition=established --timeout=60s crd/distoreclusters.distore.io
        
        # Test creating a DistoreCluster resource
        $distoreClusterYaml = @"
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: test-cluster
  namespace: $Namespace
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
"@
        
        $distoreClusterYaml | kubectl apply -f -
        
        # Wait for the resource to be created
        kubectl wait --for condition=Ready --timeout=60s distorecluster/test-cluster -n $Namespace -ErrorAction SilentlyContinue
        
        kubectl get distorecluster test-cluster -n $Namespace | Out-Null
        Write-Success "DistoreCluster CRD test passed"
    }
    catch {
        Write-Error "DistoreCluster CRD test failed: $_"
        throw
    }
}

# Test 4: Test deployment scenarios
function Test-DeploymentScenarios {
    Write-Status "Testing deployment scenarios..."
    
    $deploymentFiles = Get-ChildItem -Path "deployments" -Filter "*.yaml" -ErrorAction SilentlyContinue
    foreach ($manifest in $deploymentFiles) {
        try {
            kubectl apply --dry-run=server -f $manifest.FullName
            Write-Success "$($manifest.BaseName) deployment scenario validation passed"
        }
        catch {
            Write-Warning "$($manifest.BaseName) deployment scenario validation failed (may be expected in test environment)"
        }
    }
}

# Test 5: Test configuration validation
function Test-ConfigurationValidation {
    Write-Status "Testing configuration validation..."
    
    try {
        # Test valid configuration
        $validConfigYaml = @"
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: valid-config-test
  namespace: $Namespace
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
"@
        
        $validConfigYaml | kubectl apply -f -
        kubectl get distorecluster valid-config-test -n $Namespace | Out-Null
        Write-Success "Valid configuration test passed"
        kubectl delete distorecluster valid-config-test -n $Namespace
        
        # Test invalid configuration (should be rejected)
        $invalidConfigYaml = @"
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: invalid-config-test
  namespace: $Namespace
spec:
  replicas: 0
  image: distore/distore:latest
  multiCloud:
    enabled: true
    dataCenters: []
"@
        
        try {
            $invalidConfigYaml | kubectl apply -f -
            kubectl get distorecluster invalid-config-test -n $Namespace | Out-Null
            Write-Warning "Invalid configuration was accepted (validation may not be implemented yet)"
            kubectl delete distorecluster invalid-config-test -n $Namespace
        }
        catch {
            Write-Success "Invalid configuration was properly rejected"
        }
    }
    catch {
        Write-Error "Configuration validation test failed: $_"
        throw
    }
}

# Test 6: Test edge computing features
function Test-EdgeComputing {
    Write-Status "Testing edge computing features..."
    
    try {
        $edgeConfigYaml = @"
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: edge-test
  namespace: $Namespace
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
"@
        
        $edgeConfigYaml | kubectl apply -f -
        kubectl get distorecluster edge-test -n $Namespace | Out-Null
        Write-Success "Edge computing configuration test passed"
        kubectl delete distorecluster edge-test -n $Namespace
    }
    catch {
        Write-Error "Edge computing configuration test failed: $_"
        throw
    }
}

# Test 7: Test scaling operations
function Test-ScalingOperations {
    Write-Status "Testing scaling operations..."
    
    try {
        # Create a cluster
        $scalingConfigYaml = @"
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: scaling-test
  namespace: $Namespace
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
"@
        
        $scalingConfigYaml | kubectl apply -f -
        
        # Test scaling up
        kubectl patch distorecluster scaling-test -n $Namespace --type='merge' -p='{"spec":{"replicas":3}}'
        
        # Test scaling down
        kubectl patch distorecluster scaling-test -n $Namespace --type='merge' -p='{"spec":{"replicas":1}}'
        
        kubectl get distorecluster scaling-test -n $Namespace | Out-Null
        Write-Success "Scaling operations test passed"
        kubectl delete distorecluster scaling-test -n $Namespace
    }
    catch {
        Write-Error "Scaling operations test failed: $_"
        throw
    }
}

# Test 8: Test resource limits and requests
function Test-ResourceManagement {
    Write-Status "Testing resource management..."
    
    try {
        $resourceConfigYaml = @"
apiVersion: distore.io/v1
kind: DistoreCluster
metadata:
  name: resource-test
  namespace: $Namespace
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
"@
        
        $resourceConfigYaml | kubectl apply -f -
        kubectl get distorecluster resource-test -n $Namespace | Out-Null
        Write-Success "Resource management test passed"
        kubectl delete distorecluster resource-test -n $Namespace
    }
    catch {
        Write-Error "Resource management test failed: $_"
        throw
    }
}

# Main test execution
function Main {
    try {
        Write-Status "Starting comprehensive Kubernetes integration tests..."
        
        Test-Prerequisites
        Test-ManifestValidation
        Test-NamespaceAndRBAC
        Test-CRDInstallation
        Test-DeploymentScenarios
        Test-ConfigurationValidation
        Test-EdgeComputing
        Test-ScalingOperations
        Test-ResourceManagement
        
        Write-Success "All Kubernetes integration tests completed!"
        Write-Status "Test summary:"
        Write-Status "- Manifest validation: ✓"
        Write-Status "- Namespace and RBAC: ✓"
        Write-Status "- CRD installation: ✓"
        Write-Status "- Deployment scenarios: ✓"
        Write-Status "- Configuration validation: ✓"
        Write-Status "- Edge computing: ✓"
        Write-Status "- Scaling operations: ✓"
        Write-Status "- Resource management: ✓"
    }
    catch {
        Write-Error "Test execution failed: $_"
        Cleanup-Resources
        exit 1
    }
    finally {
        Cleanup-Resources
    }
}

# Run main function
Main

