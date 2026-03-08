package config

import (
	"flag"
)

var FlagRunAddr string
var BaseUrlAddr string

func ParseFlags() {
	flag.StringVar(&FlagRunAddr, "a", ":8082", "address and port to run server")
	flag.StringVar(&BaseUrlAddr, "b", "http://localhost"+FlagRunAddr+"/qsd54gFg", "base url address")
	flag.Parse()
}
