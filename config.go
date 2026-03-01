package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port int `yaml:"port"`
	} `yaml:"server"`
	Password string `yaml:"password"`
	AI struct {
		Endpoint string `yaml:"endpoint"`
		Key      string `yaml:"key"`
		Model    string `yaml:"model"`
		Mock     bool   `yaml:"mock"`
	} `yaml:"ai"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.AI.Model == "" {
		cfg.AI.Model = "gpt-4o"
	}
	return &cfg, nil
}
