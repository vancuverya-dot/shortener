package config

import (
	"github.com/caarlos0/env/v11"
)

type envConfig struct {
	ServerAddress string `env:"SERVER_ADDRESS"`
	BaseUrl       string `env:"BASE_URL"`
	FileStorage   string `env:"FILE_STORAGE_PATH"`
	DatabaseDSN   string `env:"DATABASE_DSN"`
	AuditFile     string `env:"AUDIT_FILE"`
	AuditUrl      string `env:"AUDIT_URL"`
	EnableHTTPS   *bool  `env:"ENABLE_HTTPS"`
	Config        string `env:"CONFIG"`
}

func loadEnv() (*envConfig, error) {
	cfg := &envConfig{}
	err := env.Parse(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
