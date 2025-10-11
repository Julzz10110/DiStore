package config

import (
	"encoding/json"
	"os"
)

type AuthConfig struct {
	Enabled       bool     `json:"enabled"`
	PrivateKey    string   `json:"private_key"`
	PublicKey     string   `json:"public_key"`
	TokenDuration int      `json:"token_duration"` // in seconds
	DefaultRoles  []string `json:"default_roles"`
}

type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

type ReplicationConfig struct {
	WriteQuorum          int    `json:"write_quorum"`
	ReadQuorum           int    `json:"read_quorum"`
	HintedHandoffEnabled bool   `json:"hinted_handoff_enabled"`
	ConflictResolution   string `json:"conflict_resolution"` // "lww" or "vector"
	CrossDCEnabled       bool   `json:"cross_dc_enabled"`
	MaxLatencyMs         int    `json:"max_latency_ms"`
	AsyncReplication     bool   `json:"async_replication"`
}

type FailoverConfig struct {
	CheckInterval int `json:"check_interval_seconds"`
	Timeout       int `json:"timeout_seconds"`
}

type RepairConfig struct {
	SyncInterval int `json:"sync_interval_seconds"`
}

type AdvancedConfig struct {
	TTLEnabled      bool `json:"ttl_enabled"`
	AtomicEnabled   bool `json:"atomic_enabled"`
	BatchEnabled    bool `json:"batch_enabled"`
	CASEnabled      bool `json:"cas_enabled"`
	LockingEnabled  bool `json:"locking_enabled"`
	DefaultTTL      int  `json:"default_ttl"`      // in seconds
	CleanupInterval int  `json:"cleanup_interval"` // in seconds
}

type PerformanceConfig struct {
	Enabled              bool `json:"enabled"`
	CacheSize            int  `json:"cache_size"`
	CacheTTL             int  `json:"cache_ttl"` // in seconds
	CompressionEnabled   bool `json:"compression_enabled"`
	CompressionThreshold int  `json:"compression_threshold"` // min size for compression
	BloomFilterEnabled   bool `json:"bloom_filter_enabled"`
	ExpectedElements     int  `json:"expected_elements"` // for Bloom filter
	WALEnabled           bool `json:"wal_enabled"`
}

type Config struct {
	HTTPPort       int               `json:"http_port"`
	Nodes          []string          `json:"nodes"`
	ReplicaCount   int               `json:"replica_count"`
	DataDir        string            `json:"data_dir"`
	Auth           AuthConfig        `json:"auth"`
	TLS            TLSConfig         `json:"tls"`
	PrometheusPort int               `json:"prometheus_port"`
	Replication    ReplicationConfig `json:"replication"`
	Failover       FailoverConfig    `json:"failover"`
	Repair         RepairConfig      `json:"repair"`
	Advanced       AdvancedConfig    `json:"advanced"`
	Performance    PerformanceConfig `json:"performance"`
	MultiCloud     MultiCloudConfig  `json:"multi_cloud"`
}

type MultiCloudConfig struct {
	Enabled           bool               `json:"enabled"`
	DataCenters       []DataCenterConfig `json:"data_centers"`
	EdgeNodes         []EdgeNodeConfig   `json:"edge_nodes"`
	LatencyThresholds LatencyConfig      `json:"latency_thresholds"`
	CloudProviders    []CloudProvider    `json:"cloud_providers"`
}

type DataCenterConfig struct {
	ID           string   `json:"id"`
	Region       string   `json:"region"`
	Nodes        []string `json:"nodes"`
	Priority     int      `json:"priority"`
	ReplicaCount int      `json:"replica_count"`
	LatencyMs    int      `json:"latency_ms"`
}

type EdgeNodeConfig struct {
	ID        string `json:"id"`
	Location  string `json:"location"`
	Node      string `json:"node"`
	CacheOnly bool   `json:"cache_only"`
	LatencyMs int    `json:"latency_ms"`
}

type LatencyConfig struct {
	LocalThresholdMs   int `json:"local_threshold_ms"`
	CrossDCThresholdMs int `json:"cross_dc_threshold_ms"`
	EdgeThresholdMs    int `json:"edge_threshold_ms"`
}

type CloudProvider struct {
	Name   string                 `json:"name"`
	Region string                 `json:"region"`
	Nodes  []string               `json:"nodes"`
	Config map[string]interface{} `json:"config"`
}

func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
