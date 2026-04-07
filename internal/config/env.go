package config

import (
	"github.com/caarlos0/env/v11"
)

type envConfig struct {
	ServerAddress string `env:"SERVER_ADDRESS"`
	BaseUrl       string `env:"BASE_URL"`
	FileStorage   string `env:"FILE_STORAGE_PATH"`
}

func loadEnv() (*envConfig, error) {
	cfg := &envConfig{}
	err := env.Parse(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
