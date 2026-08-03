package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type serverConfig struct {
	ServerAddress string
	BaseUrl       string
	FileStorage   string
	DatabaseDSN   string
	AuditFile     string
	AuditUrl      string
	EnableHTTPS   bool
}

type flagValue struct {
	f       *flag.Flag
	changed bool
	typ     string
}

func (v flagValue) HasChanged() bool    { return v.changed }
func (v flagValue) Name() string        { return v.f.Name }
func (v flagValue) ValueString() string { return v.f.Value.String() }
func (v flagValue) ValueType() string   { return v.typ }

// LoadServerConfig собирает конфигурацию сервера через viper.
// Приоритет по убыванию: флаги командной строки, переменные окружения,
// файл конфигурации в формате JSON, значения по умолчанию.
func LoadServerConfig() (*serverConfig, error) {
	var (
		flagRunAddr string
		baseURLAddr string
		fileStorage string
		databaseDSN string
		auditFile   string
		auditURL    string
		enableHTTPS bool
		configPath  string
	)

	flag.StringVar(&flagRunAddr, "a", "", "address and port to run server")
	flag.StringVar(&baseURLAddr, "b", "", "base url address")
	flag.StringVar(&fileStorage, "f", "", "file storage path")
	flag.StringVar(&databaseDSN, "d", "", "database DSN")
	flag.StringVar(&auditFile, "audit-file", "", "audit file path")
	flag.StringVar(&auditURL, "audit-url", "", "audit url")
	flag.BoolVar(&enableHTTPS, "s", false, "enable HTTPS")
	flag.StringVar(&configPath, "c", "", "path to JSON config file")
	flag.StringVar(&configPath, "config", "", "path to JSON config file")
	flag.Parse()

	changed := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { changed[f.Name] = true })

	v := viper.New()

	v.SetDefault("server_address", "localhost:8080")
	v.SetDefault("file_storage_path", "./urls.base")
	v.SetDefault("base_url", "")
	v.SetDefault("database_dsn", "")
	v.SetDefault("audit_file", "")
	v.SetDefault("audit_url", "")
	v.SetDefault("enable_https", false)

	if err := bindEnv(v); err != nil {
		return nil, err
	}

	if err := bindFlags(v, changed); err != nil {
		return nil, err
	}

	if envPath := os.Getenv("CONFIG"); envPath != "" {
		configPath = envPath
	}
	if configPath != "" {
		v.SetConfigFile(configPath)
		v.SetConfigType("json")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("чтение файла конфигурации %q: %w", configPath, err)
		}
	}

	cfg := &serverConfig{
		ServerAddress: v.GetString("server_address"),
		BaseUrl:       v.GetString("base_url"),
		FileStorage:   v.GetString("file_storage_path"),
		DatabaseDSN:   v.GetString("database_dsn"),
		AuditFile:     v.GetString("audit_file"),
		AuditUrl:      v.GetString("audit_url"),
		EnableHTTPS:   v.GetBool("enable_https"),
	}

	if cfg.BaseUrl == "" {
		cfg.BaseUrl = "http://" + cfg.ServerAddress + "/qsd54gFg"
	}

	return cfg, nil
}

// bindEnv связывает ключи конфигурации с переменными окружения.
func bindEnv(v *viper.Viper) error {
	bindings := map[string]string{
		"server_address":    "SERVER_ADDRESS",
		"base_url":          "BASE_URL",
		"file_storage_path": "FILE_STORAGE_PATH",
		"database_dsn":      "DATABASE_DSN",
		"audit_file":        "AUDIT_FILE",
		"audit_url":         "AUDIT_URL",
		"enable_https":      "ENABLE_HTTPS",
	}
	for key, env := range bindings {
		if err := v.BindEnv(key, env); err != nil {
			return fmt.Errorf("привязка переменной окружения %s: %w", env, err)
		}
	}
	return nil
}

// bindFlags связывает ключи конфигурации с флагами командной строки.
func bindFlags(v *viper.Viper, changed map[string]bool) error {
	bindings := []struct {
		key  string
		name string
		typ  string
	}{
		{"server_address", "a", "string"},
		{"base_url", "b", "string"},
		{"file_storage_path", "f", "string"},
		{"database_dsn", "d", "string"},
		{"audit_file", "audit-file", "string"},
		{"audit_url", "audit-url", "string"},
		{"enable_https", "s", "bool"},
	}

	for _, b := range bindings {
		f := flag.Lookup(b.name)
		if f == nil {
			continue
		}
		fv := flagValue{f: f, changed: changed[b.name], typ: b.typ}
		if err := v.BindFlagValue(b.key, fv); err != nil {
			return fmt.Errorf("привязка флага -%s: %w", b.name, err)
		}
	}
	return nil
}
