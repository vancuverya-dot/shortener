package config

import (
	"flag"
)

var FlagRunAddr string
var BaseUrlAddr string
var FileStorage string

func ParseFlags() {
	flag.StringVar(&FlagRunAddr, "a", ":8080", "address and port to run server")
	flag.StringVar(&BaseUrlAddr, "b", "http://localhost"+FlagRunAddr+"/qsd54gFg", "base url address")
	flag.StringVar(&FileStorage, "f", "D:/insomnia binary/urls.base", "file storage path")
	flag.Parse()
}
