# Kubernetes Deployment for Distore

This directory contains Kubernetes manifests and operators for deploying Distore across multiple cloud providers.

## Files

- `operator.yaml` - Kubernetes operator for managing Distore clusters
- `edge-computing.go` - Edge computing management logic
- `README.md` - This file

## Deployment Manifests

The `../deployments/` directory contains cloud-specific deployment manifests:

- `aws.yaml` - AWS EKS deployment
- `gcp.yaml` - Google Cloud GKE deployment  
- `azure.yaml` - Azure AKS deployment
- `multi-cloud.yaml` - Multi-cloud deployment with edge nodes

## Multi-Cloud Features

### Cross-Datacenter Replication
- Latency-aware replication between data centers
- Configurable timeout and priority settings
- Automatic failover between regions

### Edge Computing Support
- Cache-only edge nodes for low-latency access
- Full replica edge nodes for offline capability
- Geographic routing based on location

### Cloud-Agnostic Deployment
- Kubernetes operators for automated management
- Consistent deployment across AWS, GCP, and Azure
- Configurable storage classes and resource limits

## Usage

### Single Cloud Deployment
```bash
kubectl apply -f deployments/aws.yaml
```

### Multi-Cloud Deployment
```bash
kubectl apply -f deployments/multi-cloud.yaml
```

### Operator Deployment
```bash
kubectl apply -f operator.yaml
```

## Configuration

The deployment uses ConfigMaps to configure:
- Cross-datacenter replication settings
- Edge node configurations
- Latency thresholds
- Cloud provider specific settings

## Monitoring

Each deployment includes:
- Prometheus metrics endpoint on port 9090
- Health check endpoints
- Resource usage monitoring
- Cross-DC latency tracking

