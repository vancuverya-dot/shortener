package config

import (
	"log"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	ServerAddress string `env:"SERVER_ADDRESS"`
	BaseUrl       string `env:"BASE_URL"`
}

var EnvCfg Config

func GetEnvs() {
	err := env.Parse(&EnvCfg)
	if err != nil {
		log.Fatal(err)
	}
}
