package k8s

import (
	"distore/config"
	"testing"
)

func TestValidateMultiCloudConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.MultiCloudConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid multi-cloud config",
			config: &config.MultiCloudConfig{
				Enabled: true,
				DataCenters: []config.DataCenterConfig{
					{
						ID:           "dc1",
						Region:       "us-east-1",
						Nodes:        []string{"node1:8080", "node2:8080"},
						Priority:     1,
						ReplicaCount: 2,
						LatencyMs:    50,
					},
					{
						ID:           "dc2",
						Region:       "us-west-1",
						Nodes:        []string{"node3:8080", "node4:8080"},
						Priority:     2,
						ReplicaCount: 2,
						LatencyMs:    100,
					},
				},
				EdgeNodes: []config.EdgeNodeConfig{
					{
						ID:        "edge-nyc",
						Location:  "New York",
						Node:      "edge-nyc:8080",
						CacheOnly: true,
						LatencyMs: 20,
					},
				},
				LatencyThresholds: config.LatencyConfig{
					LocalThresholdMs:   10,
					CrossDCThresholdMs: 100,
					EdgeThresholdMs:    200,
				},
				CloudProviders: []config.CloudProvider{
					{
						Name:   "aws",
						Region: "us-east-1",
						Nodes:  []string{"aws-node1:8080"},
						Config: map[string]interface{}{
							"instance_type": "m5.large",
							"disk_size_gb":  100,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "Empty datacenters",
			config: &config.MultiCloudConfig{
				Enabled:        true,
				DataCenters:    []config.DataCenterConfig{},
				EdgeNodes:      []config.EdgeNodeConfig{},
				CloudProviders: []config.CloudProvider{},
			},
			expectError: true,
			errorMsg:    "at least one datacenter is required",
		},
		{
			name: "Invalid datacenter ID",
			config: &config.MultiCloudConfig{
				Enabled: true,
				DataCenters: []config.DataCenterConfig{
					{
						ID:           "", // Empty ID
						Region:       "us-east-1",
						Nodes:        []string{"node1:8080"},
						Priority:     1,
						ReplicaCount: 1,
						LatencyMs:    50,
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid datacenter config: datacenter ID cannot be empty",
		},
		{
			name: "Invalid datacenter nodes",
			config: &config.MultiCloudConfig{
				Enabled: true,
				DataCenters: []config.DataCenterConfig{
					{
						ID:           "dc1",
						Region:       "us-east-1",
						Nodes:        []string{}, // Empty nodes
						Priority:     1,
						ReplicaCount: 1,
						LatencyMs:    50,
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid datacenter config: datacenter must have at least one node",
		},
		{
			name: "Invalid priority",
			config: &config.MultiCloudConfig{
				Enabled: true,
				DataCenters: []config.DataCenterConfig{
					{
						ID:           "dc1",
						Region:       "us-east-1",
						Nodes:        []string{"node1:8080"},
						Priority:     0, // Invalid priority
						ReplicaCount: 1,
						LatencyMs:    50,
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid datacenter config: datacenter priority must be positive",
		},
		{
			name: "Invalid replica count",
			config: &config.MultiCloudConfig{
				Enabled: true,
				DataCenters: []config.DataCenterConfig{
					{
						ID:           "dc1",
						Region:       "us-east-1",
						Nodes:        []string{"node1:8080"},
						Priority:     1,
						ReplicaCount: 0, // Invalid replica count
						LatencyMs:    50,
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid datacenter config: replica count must be positive",
		},
		{
			name: "Invalid edge node ID",
			config: &config.MultiCloudConfig{
				Enabled: true,
				DataCenters: []config.DataCenterConfig{
					{
						ID:           "dc1",
						Region:       "us-east-1",
						Nodes:        []string{"node1:8080"},
						Priority:     1,
						ReplicaCount: 1,
						LatencyMs:    50,
					},
				},
				EdgeNodes: []config.EdgeNodeConfig{
					{
						ID:        "", // Empty ID
						Location:  "New York",
						Node:      "edge-nyc:8080",
						CacheOnly: true,
						LatencyMs: 20,
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid edge node config: edge node ID cannot be empty",
		},
		{
			name: "Invalid edge node location",
			config: &config.MultiCloudConfig{
				Enabled: true,
				DataCenters: []config.DataCenterConfig{
					{
						ID:           "dc1",
						Region:       "us-east-1",
						Nodes:        []string{"node1:8080"},
						Priority:     1,
						ReplicaCount: 1,
						LatencyMs:    50,
					},
				},
				EdgeNodes: []config.EdgeNodeConfig{
					{
						ID:        "edge-nyc",
						Location:  "", // Empty location
						Node:      "edge-nyc:8080",
						CacheOnly: true,
						LatencyMs: 20,
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid edge node config: edge node location cannot be empty",
		},
		{
			name: "Invalid latency thresholds",
			config: &config.MultiCloudConfig{
				Enabled: true,
				DataCenters: []config.DataCenterConfig{
					{
						ID:           "dc1",
						Region:       "us-east-1",
						Nodes:        []string{"node1:8080"},
						Priority:     1,
						ReplicaCount: 1,
						LatencyMs:    50,
					},
				},
				LatencyThresholds: config.LatencyConfig{
					LocalThresholdMs:   -1, // Invalid threshold
					CrossDCThresholdMs: 100,
					EdgeThresholdMs:    200,
				},
			},
			expectError: true,
			errorMsg:    "invalid latency config: latency threshold must be non-negative",
		},
		{
			name: "Invalid cloud provider name",
			config: &config.MultiCloudConfig{
				Enabled: true,
				DataCenters: []config.DataCenterConfig{
					{
						ID:           "dc1",
						Region:       "us-east-1",
						Nodes:        []string{"node1:8080"},
						Priority:     1,
						ReplicaCount: 1,
						LatencyMs:    50,
					},
				},
				CloudProviders: []config.CloudProvider{
					{
						Name:   "", // Empty name
						Region: "us-east-1",
						Nodes:  []string{"aws-node1:8080"},
						Config: map[string]interface{}{},
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid cloud provider config: cloud provider name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMultiCloudConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateDataCenterConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      config.DataCenterConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid datacenter config",
			config: config.DataCenterConfig{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080", "node2:8080"},
				Priority:     1,
				ReplicaCount: 2,
				LatencyMs:    50,
			},
			expectError: false,
		},
		{
			name: "Empty ID",
			config: config.DataCenterConfig{
				ID:           "",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080"},
				Priority:     1,
				ReplicaCount: 1,
				LatencyMs:    50,
			},
			expectError: true,
			errorMsg:    "invalid datacenter config: datacenter ID cannot be empty",
		},
		{
			name: "Empty region",
			config: config.DataCenterConfig{
				ID:           "dc1",
				Region:       "",
				Nodes:        []string{"node1:8080"},
				Priority:     1,
				ReplicaCount: 1,
				LatencyMs:    50,
			},
			expectError: true,
			errorMsg:    "datacenter region cannot be empty",
		},
		{
			name: "No nodes",
			config: config.DataCenterConfig{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{},
				Priority:     1,
				ReplicaCount: 1,
				LatencyMs:    50,
			},
			expectError: true,
			errorMsg:    "invalid datacenter config: datacenter must have at least one node",
		},
		{
			name: "Zero priority",
			config: config.DataCenterConfig{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080"},
				Priority:     0,
				ReplicaCount: 1,
				LatencyMs:    50,
			},
			expectError: true,
			errorMsg:    "invalid datacenter config: datacenter priority must be positive",
		},
		{
			name: "Zero replica count",
			config: config.DataCenterConfig{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080"},
				Priority:     1,
				ReplicaCount: 0,
				LatencyMs:    50,
			},
			expectError: true,
			errorMsg:    "invalid datacenter config: replica count must be positive",
		},
		{
			name: "Negative latency",
			config: config.DataCenterConfig{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080"},
				Priority:     1,
				ReplicaCount: 1,
				LatencyMs:    -10,
			},
			expectError: true,
			errorMsg:    "latency must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDataCenterConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateEdgeNodeConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      config.EdgeNodeConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid edge node config",
			config: config.EdgeNodeConfig{
				ID:        "edge-nyc",
				Location:  "New York",
				Node:      "edge-nyc:8080",
				CacheOnly: true,
				LatencyMs: 20,
			},
			expectError: false,
		},
		{
			name: "Empty ID",
			config: config.EdgeNodeConfig{
				ID:        "",
				Location:  "New York",
				Node:      "edge-nyc:8080",
				CacheOnly: true,
				LatencyMs: 20,
			},
			expectError: true,
			errorMsg:    "invalid edge node config: edge node ID cannot be empty",
		},
		{
			name: "Empty location",
			config: config.EdgeNodeConfig{
				ID:        "edge-nyc",
				Location:  "",
				Node:      "edge-nyc:8080",
				CacheOnly: true,
				LatencyMs: 20,
			},
			expectError: true,
			errorMsg:    "invalid edge node config: edge node location cannot be empty",
		},
		{
			name: "Empty node",
			config: config.EdgeNodeConfig{
				ID:        "edge-nyc",
				Location:  "New York",
				Node:      "",
				CacheOnly: true,
				LatencyMs: 20,
			},
			expectError: true,
			errorMsg:    "edge node address cannot be empty",
		},
		{
			name: "Negative latency",
			config: config.EdgeNodeConfig{
				ID:        "edge-nyc",
				Location:  "New York",
				Node:      "edge-nyc:8080",
				CacheOnly: true,
				LatencyMs: -5,
			},
			expectError: true,
			errorMsg:    "latency must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEdgeNodeConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateCloudProviderConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      config.CloudProvider
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid cloud provider config",
			config: config.CloudProvider{
				Name:   "aws",
				Region: "us-east-1",
				Nodes:  []string{"aws-node1:8080", "aws-node2:8080"},
				Config: map[string]interface{}{
					"instance_type": "m5.large",
					"disk_size_gb":  100,
				},
			},
			expectError: false,
		},
		{
			name: "Empty name",
			config: config.CloudProvider{
				Name:   "",
				Region: "us-east-1",
				Nodes:  []string{"aws-node1:8080"},
				Config: map[string]interface{}{},
			},
			expectError: true,
			errorMsg:    "invalid cloud provider config: cloud provider name cannot be empty",
		},
		{
			name: "Empty region",
			config: config.CloudProvider{
				Name:   "aws",
				Region: "",
				Nodes:  []string{"aws-node1:8080"},
				Config: map[string]interface{}{},
			},
			expectError: true,
			errorMsg:    "cloud provider region cannot be empty",
		},
		{
			name: "No nodes",
			config: config.CloudProvider{
				Name:   "aws",
				Region: "us-east-1",
				Nodes:  []string{},
				Config: map[string]interface{}{},
			},
			expectError: true,
			errorMsg:    "cloud provider must have at least one node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCloudProviderConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateLatencyConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      config.LatencyConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid latency config",
			config: config.LatencyConfig{
				LocalThresholdMs:   10,
				CrossDCThresholdMs: 100,
				EdgeThresholdMs:    200,
			},
			expectError: false,
		},
		{
			name: "Negative local threshold",
			config: config.LatencyConfig{
				LocalThresholdMs:   -1,
				CrossDCThresholdMs: 100,
				EdgeThresholdMs:    200,
			},
			expectError: true,
			errorMsg:    "invalid latency config: latency threshold must be non-negative",
		},
		{
			name: "Negative cross-DC threshold",
			config: config.LatencyConfig{
				LocalThresholdMs:   10,
				CrossDCThresholdMs: -50,
				EdgeThresholdMs:    200,
			},
			expectError: true,
			errorMsg:    "invalid latency config: latency threshold must be non-negative",
		},
		{
			name: "Negative edge threshold",
			config: config.LatencyConfig{
				LocalThresholdMs:   10,
				CrossDCThresholdMs: 100,
				EdgeThresholdMs:    -100,
			},
			expectError: true,
			errorMsg:    "invalid latency config: latency threshold must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLatencyConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateNodeAddress(t *testing.T) {
	tests := []struct {
		name        string
		address     string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid address with port",
			address:     "node1:8080",
			expectError: false,
		},
		{
			name:        "Valid IP address with port",
			address:     "192.168.1.100:8080",
			expectError: false,
		},
		{
			name:        "Empty address",
			address:     "",
			expectError: true,
			errorMsg:    "node address cannot be empty",
		},
		{
			name:        "Address without port",
			address:     "node1",
			expectError: true,
			errorMsg:    "node address must include port",
		},
		{
			name:        "Invalid port",
			address:     "node1:invalid",
			expectError: true,
			errorMsg:    "invalid port in node address",
		},
		{
			name:        "Port out of range",
			address:     "node1:99999",
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNodeAddress(tt.address)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateReplicationConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      config.ReplicationConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid replication config",
			config: config.ReplicationConfig{
				CrossDCEnabled:   true,
				MaxLatencyMs:     1000,
				AsyncReplication: false,
			},
			expectError: false,
		},
		{
			name: "Negative max latency",
			config: config.ReplicationConfig{
				CrossDCEnabled:   true,
				MaxLatencyMs:     -100,
				AsyncReplication: false,
			},
			expectError: true,
			errorMsg:    "max latency must be non-negative",
		},
		{
			name: "Disabled cross-DC with async replication",
			config: config.ReplicationConfig{
				CrossDCEnabled:   false,
				MaxLatencyMs:     1000,
				AsyncReplication: true,
			},
			expectError: false, // This should be valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReplicationConfig(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// Benchmark tests for configuration validation
func BenchmarkValidateMultiCloudConfig(b *testing.B) {
	cfg := &config.MultiCloudConfig{
		Enabled: true,
		DataCenters: []config.DataCenterConfig{
			{
				ID:           "dc1",
				Region:       "us-east-1",
				Nodes:        []string{"node1:8080", "node2:8080", "node3:8080"},
				Priority:     1,
				ReplicaCount: 3,
				LatencyMs:    50,
			},
			{
				ID:           "dc2",
				Region:       "us-west-1",
				Nodes:        []string{"node4:8080", "node5:8080", "node6:8080"},
				Priority:     2,
				ReplicaCount: 3,
				LatencyMs:    100,
			},
		},
		EdgeNodes: []config.EdgeNodeConfig{
			{
				ID:        "edge-nyc",
				Location:  "New York",
				Node:      "edge-nyc:8080",
				CacheOnly: true,
				LatencyMs: 20,
			},
			{
				ID:        "edge-london",
				Location:  "London",
				Node:      "edge-london:8080",
				CacheOnly: false,
				LatencyMs: 50,
			},
		},
		LatencyThresholds: config.LatencyConfig{
			LocalThresholdMs:   10,
			CrossDCThresholdMs: 100,
			EdgeThresholdMs:    200,
		},
		CloudProviders: []config.CloudProvider{
			{
				Name:   "aws",
				Region: "us-east-1",
				Nodes:  []string{"aws-node1:8080", "aws-node2:8080"},
				Config: map[string]interface{}{
					"instance_type": "m5.large",
					"disk_size_gb":  100,
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateMultiCloudConfig(cfg)
	}
}
