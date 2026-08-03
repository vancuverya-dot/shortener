package main

import (
	"log"
	"os"
)

func helper() {
	log.Fatal("fail") // want "call to log.Fatal is not allowed outside main function of package main"
}

func main() {
	// log.Fatal и os.Exit внутри main разрешены — диагностик быть не должно.
	if false {
		log.Fatal("ok in main")
	}

	// panic репортится всегда, даже внутри main.
	if false {
		panic("boom") // want "use of panic is not allowed"
	}

	os.Exit(0)
}
