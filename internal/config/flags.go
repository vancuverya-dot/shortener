package config

import (
	"flag"
	"fmt"
)

type serverConfig struct {
	ServerAddress string
	BaseUrl       string
	FileStorage   string
	DatabaseDSN   string
	AuditFile     string
	AuditUrl      string
	EnableHTTPS   bool
	configPath    string
}

func LoadServerConfig() (*serverConfig, error) {
	var (
		flagRunAddr string
		baseUrlAddr string
		fileStorage string
		databaseDsn string
		auditFile   string
		auditUrl    string
		enableHTTPS bool
		configPath  string
	)

	flag.StringVar(&flagRunAddr, "a", "", "address and port to run server")
	flag.StringVar(&baseUrlAddr, "b", "", "base url address")
	flag.StringVar(&fileStorage, "f", "", "file storage path")
	flag.StringVar(&databaseDsn, "d", "", "database DSN")
	flag.StringVar(&auditFile, "audit-file", "", "audit file path")
	flag.StringVar(&auditUrl, "audit-url", "", "audit url")
	flag.BoolVar(&enableHTTPS, "s", false, "enable HTTPS")
	flag.StringVar(&configPath, "c", "", "path to JSON config file")
	flag.StringVar(&configPath, "config", "", "path to JSON config file")
	flag.Parse()

	setFlags := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	envConfig, err := loadEnv()
	if err != nil {
		return nil, fmt.Errorf("error while loading environment variables: %w", err)
	}

	if envConfig.Config != "" {
		configPath = envConfig.Config
	}

	fileConfig, err := loadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := &serverConfig{
		ServerAddress: "localhost:8080",
		FileStorage:   "./urls.base",
	}

	applyString(&cfg.ServerAddress, fileConfig.ServerAddress)
	applyString(&cfg.BaseUrl, fileConfig.BaseURL)
	applyString(&cfg.FileStorage, fileConfig.FileStorage)
	applyString(&cfg.DatabaseDSN, fileConfig.DatabaseDSN)
	applyString(&cfg.AuditFile, fileConfig.AuditFile)
	applyString(&cfg.AuditUrl, fileConfig.AuditURL)
	if fileConfig.EnableHTTPS != nil {
		cfg.EnableHTTPS = *fileConfig.EnableHTTPS
	}

	if setFlags["a"] {
		cfg.ServerAddress = flagRunAddr
	}
	if setFlags["b"] {
		cfg.BaseUrl = baseUrlAddr
	}
	if setFlags["f"] {
		cfg.FileStorage = fileStorage
	}
	if setFlags["d"] {
		cfg.DatabaseDSN = databaseDsn
	}
	if setFlags["audit-file"] {
		cfg.AuditFile = auditFile
	}
	if setFlags["audit-url"] {
		cfg.AuditUrl = auditUrl
	}
	if setFlags["s"] {
		cfg.EnableHTTPS = enableHTTPS
	}

	applyEnvString(&cfg.ServerAddress, envConfig.ServerAddress)
	applyEnvString(&cfg.BaseUrl, envConfig.BaseUrl)
	applyEnvString(&cfg.FileStorage, envConfig.FileStorage)
	applyEnvString(&cfg.DatabaseDSN, envConfig.DatabaseDSN)
	applyEnvString(&cfg.AuditFile, envConfig.AuditFile)
	applyEnvString(&cfg.AuditUrl, envConfig.AuditUrl)
	if envConfig.EnableHTTPS != nil {
		cfg.EnableHTTPS = *envConfig.EnableHTTPS
	}

	if cfg.BaseUrl == "" {
		cfg.BaseUrl = "http://" + cfg.ServerAddress + "/qsd54gFg"
	}

	return cfg, nil
}

func applyString(dst *string, src *string) {
	if src != nil {
		*dst = *src
	}
}

func applyEnvString(dst *string, src string) {
	if src != "" {
		*dst = src
	}
}
