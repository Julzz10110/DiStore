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
}

type FailoverConfig struct {
	CheckInterval int `json:"check_interval_seconds"`
	Timeout       int `json:"timeout_seconds"`
}

type RepairConfig struct {
	SyncInterval int `json:"sync_interval_seconds"`
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
