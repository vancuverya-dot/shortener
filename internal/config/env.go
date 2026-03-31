package config

import (
	"log"

	"github.com/caarlos0/env/v6"
)

type config struct {
	ServerAddress string `env:"SERVER_ADDRESS"`
	BaseUrl       string `env:"BASE_URL"`
	FileStorage   string `env:"FILE_STORAGE_PATH"`
}

var EnvCfg config

func GetEnvs() {
	err := env.Parse(&EnvCfg)
	if err != nil {
		log.Fatal(err)
	}
}
