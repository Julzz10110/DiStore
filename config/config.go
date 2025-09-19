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

type Config struct {
	HTTPPort       int        `json:"http_port"`
	Nodes          []string   `json:"nodes"`
	ReplicaCount   int        `json:"replica_count"`
	DataDir        string     `json:"data_dir"`
	Auth           AuthConfig `json:"auth"`
	TLS            TLSConfig  `json:"tls"`
	PrometheusPort int        `json:"prometheus_port"`
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
