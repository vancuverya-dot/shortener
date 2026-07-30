package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// fileConfig — конфигурация приложения в формате JSON.
type fileConfig struct {
	ServerAddress *string `json:"server_address"`
	BaseURL       *string `json:"base_url"`
	FileStorage   *string `json:"file_storage_path"`
	DatabaseDSN   *string `json:"database_dsn"`
	AuditFile     *string `json:"audit_file"`
	AuditURL      *string `json:"audit_url"`
	EnableHTTPS   *bool   `json:"enable_https"`
}

// loadFile читает и разбирает JSON-файл конфигурации.
// Если path пуст, возвращает пустую конфигурацию без ошибки.
func loadFile(path string) (*fileConfig, error) {
	if path == "" {
		return &fileConfig{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("чтение файла конфигурации %q: %w", path, err)
	}

	cfg := &fileConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("разбор файла конфигурации %q: %w", path, err)
	}
	return cfg, nil
}
