package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	HTTPPort     int      `json:"http_port"`
	Nodes        []string `json:"nodes"`
	ReplicaCount int      `json:"replica_count"`
	DataDir      string   `json:"data_dir"`
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
