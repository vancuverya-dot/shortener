package config

import (
	"flag"
	"log"
)

type serverConfig struct {
	ServerAddress string
	BaseUrl       string
	FileStorage   string
}

func LoadServerConfig() *serverConfig {

	var flagRunAddr string
	var baseUrlAddr string
	var fileStorage string

	envConfig, err := loadEnv()
	if err != nil {
		log.Fatalf("ошибка загрузки переменных окружения: %v", err)
	}

	flag.StringVar(&flagRunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&baseUrlAddr, "b", "http://localhost"+flagRunAddr+"/qsd54gFg", "base url address")
	flag.StringVar(&fileStorage, "f", "./urls.base", "file storage path")
	flag.Parse()

	if envConfig.ServerAddress != "" {
		flagRunAddr = envConfig.ServerAddress
	}

	if envConfig.BaseUrl != "" {
		baseUrlAddr = envConfig.BaseUrl
	}

	if envConfig.FileStorage != "" {
		fileStorage = envConfig.FileStorage
	}

	return &serverConfig{
		ServerAddress: flagRunAddr,
		BaseUrl:       baseUrlAddr,
		FileStorage:   fileStorage,
	}

}
