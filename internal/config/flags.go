package config

import (
	"flag"
	"log"
)

type serverConfig struct {
	ServerAddress string
	BaseUrl       string
	FileStorage   string
	DatabaseDSN   string
	AuditFile     string
	AuditUrl      string
}

func LoadServerConfig() *serverConfig {

	var flagRunAddr string
	var baseUrlAddr string
	var fileStorage string
	var databaseDsn string
	var auditFile string
	var auditUrl string

	envConfig, err := loadEnv()
	if err != nil {
		log.Fatalf("ошибка загрузки переменных окружения: %v", err)
	}

	flag.StringVar(&flagRunAddr, "a", "localhost:8080", "address and port to run server")
	flag.StringVar(&baseUrlAddr, "b", "", "base url address")
	flag.StringVar(&fileStorage, "f", "./urls.base", "file storage path")
	flag.StringVar(&databaseDsn, "d", "", "database DSN")
	flag.StringVar(&auditFile, "audit-file", "", "audit file path")
	flag.StringVar(&auditUrl, "audit-url", "", "audit url")
	flag.Parse()

	if len(baseUrlAddr) == 0 {
		baseUrlAddr = "http://" + flagRunAddr + "/qsd54gFg"
	}

	if envConfig.ServerAddress != "" {
		flagRunAddr = envConfig.ServerAddress
	}

	if envConfig.BaseUrl != "" {
		baseUrlAddr = envConfig.BaseUrl
	}

	if envConfig.FileStorage != "" {
		fileStorage = envConfig.FileStorage
	}

	if envConfig.DatabaseDSN != "" {
		databaseDsn = envConfig.DatabaseDSN
	}

	if envConfig.AuditFile != "" {
		auditFile = envConfig.AuditFile
	}

	if envConfig.AuditUrl != "" {
		auditUrl = envConfig.AuditUrl
	}

	return &serverConfig{
		ServerAddress: flagRunAddr,
		BaseUrl:       baseUrlAddr,
		FileStorage:   fileStorage,
		DatabaseDSN:   databaseDsn,
		AuditFile:     auditFile,
		AuditUrl:      auditUrl,
	}

}
