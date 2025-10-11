package k8s

import (
	"distore/config"
	"fmt"
	"strconv"
	"strings"
)

// ValidateMultiCloudConfig validates the entire multi-cloud configuration
func ValidateMultiCloudConfig(cfg *config.MultiCloudConfig) error {
	if cfg == nil {
		return fmt.Errorf("multi-cloud config cannot be nil")
	}

	// Validate datacenters
	if len(cfg.DataCenters) == 0 {
		return fmt.Errorf("at least one datacenter is required")
	}

	for _, dc := range cfg.DataCenters {
		if err := validateDataCenterConfig(dc); err != nil {
			return fmt.Errorf("invalid datacenter config: %w", err)
		}
	}

	// Validate edge nodes
	for _, edgeNode := range cfg.EdgeNodes {
		if err := validateEdgeNodeConfig(edgeNode); err != nil {
			return fmt.Errorf("invalid edge node config: %w", err)
		}
	}

	// Validate latency thresholds
	if err := validateLatencyConfig(cfg.LatencyThresholds); err != nil {
		return fmt.Errorf("invalid latency config: %w", err)
	}

	// Validate cloud providers
	for _, provider := range cfg.CloudProviders {
		if err := validateCloudProviderConfig(provider); err != nil {
			return fmt.Errorf("invalid cloud provider config: %w", err)
		}
	}

	// Note: Replication config validation would be added here if the field exists
	// if err := validateReplicationConfig(cfg.Replication); err != nil {
	//     return fmt.Errorf("invalid replication config: %w", err)
	// }

	return nil
}

// validateDataCenterConfig validates a single datacenter configuration
func validateDataCenterConfig(dc config.DataCenterConfig) error {
	if dc.ID == "" {
		return fmt.Errorf("datacenter ID cannot be empty")
	}

	if dc.Region == "" {
		return fmt.Errorf("datacenter region cannot be empty")
	}

	if len(dc.Nodes) == 0 {
		return fmt.Errorf("datacenter must have at least one node")
	}

	for _, node := range dc.Nodes {
		if err := validateNodeAddress(node); err != nil {
			return fmt.Errorf("invalid node address %s: %w", node, err)
		}
	}

	if dc.Priority <= 0 {
		return fmt.Errorf("datacenter priority must be positive")
	}

	if dc.ReplicaCount <= 0 {
		return fmt.Errorf("replica count must be positive")
	}

	if dc.LatencyMs < 0 {
		return fmt.Errorf("latency must be non-negative")
	}

	return nil
}

// validateEdgeNodeConfig validates a single edge node configuration
func validateEdgeNodeConfig(edgeNode config.EdgeNodeConfig) error {
	if edgeNode.ID == "" {
		return fmt.Errorf("edge node ID cannot be empty")
	}

	if edgeNode.Location == "" {
		return fmt.Errorf("edge node location cannot be empty")
	}

	if edgeNode.Node == "" {
		return fmt.Errorf("edge node address cannot be empty")
	}

	if err := validateNodeAddress(edgeNode.Node); err != nil {
		return fmt.Errorf("invalid edge node address %s: %w", edgeNode.Node, err)
	}

	if edgeNode.LatencyMs < 0 {
		return fmt.Errorf("latency must be non-negative")
	}

	return nil
}

// validateCloudProviderConfig validates a single cloud provider configuration
func validateCloudProviderConfig(provider config.CloudProvider) error {
	if provider.Name == "" {
		return fmt.Errorf("cloud provider name cannot be empty")
	}

	if provider.Region == "" {
		return fmt.Errorf("cloud provider region cannot be empty")
	}

	if len(provider.Nodes) == 0 {
		return fmt.Errorf("cloud provider must have at least one node")
	}

	for _, node := range provider.Nodes {
		if err := validateNodeAddress(node); err != nil {
			return fmt.Errorf("invalid cloud provider node address %s: %w", node, err)
		}
	}

	return nil
}

// validateLatencyConfig validates latency threshold configuration
func validateLatencyConfig(latency config.LatencyConfig) error {
	if latency.LocalThresholdMs < 0 {
		return fmt.Errorf("latency threshold must be non-negative")
	}

	if latency.CrossDCThresholdMs < 0 {
		return fmt.Errorf("latency threshold must be non-negative")
	}

	if latency.EdgeThresholdMs < 0 {
		return fmt.Errorf("latency threshold must be non-negative")
	}

	return nil
}

// validateReplicationConfig validates replication configuration
func validateReplicationConfig(replication config.ReplicationConfig) error {
	if replication.MaxLatencyMs < 0 {
		return fmt.Errorf("max latency must be non-negative")
	}

	return nil
}

// validateNodeAddress validates a node address format (host:port)
func validateNodeAddress(address string) error {
	if address == "" {
		return fmt.Errorf("node address cannot be empty")
	}

	parts := strings.Split(address, ":")
	if len(parts) != 2 {
		return fmt.Errorf("node address must include port")
	}

	portStr := parts[1]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid port in node address")
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	return nil
}

// ValidateKubernetesManifest validates a Kubernetes manifest for Distore
func ValidateKubernetesManifest(manifest map[string]interface{}) error {
	if manifest == nil {
		return fmt.Errorf("manifest cannot be nil")
	}

	// Check required fields
	kind, ok := manifest["kind"].(string)
	if !ok || kind == "" {
		return fmt.Errorf("manifest must have a valid 'kind' field")
	}

	if kind != "DistoreCluster" {
		return fmt.Errorf("manifest kind must be 'DistoreCluster'")
	}

	metadata, ok := manifest["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("manifest must have 'metadata' field")
	}

	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("manifest must have a valid 'name' field in metadata")
	}

	spec, ok := manifest["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("manifest must have 'spec' field")
	}

	// Validate required spec fields
	requiredFields := []string{"replicas", "image", "dataDir", "httpPort"}
	for _, field := range requiredFields {
		if _, exists := spec[field]; !exists {
			return fmt.Errorf("manifest spec must have '%s' field", field)
		}
	}

	// Validate replicas
	replicas, ok := spec["replicas"].(float64)
	if !ok || replicas < 1 {
		return fmt.Errorf("replicas must be a positive integer")
	}

	// Validate image
	image, ok := spec["image"].(string)
	if !ok || image == "" {
		return fmt.Errorf("image must be a non-empty string")
	}

	// Validate dataDir
	dataDir, ok := spec["dataDir"].(string)
	if !ok || dataDir == "" {
		return fmt.Errorf("dataDir must be a non-empty string")
	}

	// Validate httpPort
	httpPort, ok := spec["httpPort"].(float64)
	if !ok || httpPort < 1 || httpPort > 65535 {
		return fmt.Errorf("httpPort must be a valid port number (1-65535)")
	}

	return nil
}

// ValidateDeploymentConfig validates a deployment configuration
func ValidateDeploymentConfig(cfg map[string]interface{}) error {
	if cfg == nil {
		return fmt.Errorf("deployment config cannot be nil")
	}

	// Validate required fields
	requiredFields := []string{"namespace", "image", "replicas"}
	for _, field := range requiredFields {
		if _, exists := cfg[field]; !exists {
			return fmt.Errorf("deployment config must have '%s' field", field)
		}
	}

	// Validate namespace
	namespace, ok := cfg["namespace"].(string)
	if !ok || namespace == "" {
		return fmt.Errorf("namespace must be a non-empty string")
	}

	// Validate image
	image, ok := cfg["image"].(string)
	if !ok || image == "" {
		return fmt.Errorf("image must be a non-empty string")
	}

	// Validate replicas
	replicas, ok := cfg["replicas"].(float64)
	if !ok || replicas < 1 {
		return fmt.Errorf("replicas must be a positive integer")
	}

	// Validate optional fields if present
	if ports, exists := cfg["ports"]; exists {
		portList, ok := ports.([]interface{})
		if !ok {
			return fmt.Errorf("ports must be a list")
		}

		for i, port := range portList {
			portMap, ok := port.(map[string]interface{})
			if !ok {
				return fmt.Errorf("port %d must be a map", i)
			}

			if name, exists := portMap["name"]; exists {
				if _, ok := name.(string); !ok {
					return fmt.Errorf("port %d name must be a string", i)
				}
			}

			if portNum, exists := portMap["port"]; exists {
				if portVal, ok := portNum.(float64); !ok || portVal < 1 || portVal > 65535 {
					return fmt.Errorf("port %d must be a valid port number (1-65535)", i)
				}
			}
		}
	}

	return nil
}
