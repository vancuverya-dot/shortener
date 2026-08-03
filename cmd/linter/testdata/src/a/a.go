package a

import (
	"log"
	"os"
)

func doSomething() {
	panic("boom") // want "use of panic is not allowed"
}

func fatalHelper() {
	log.Fatal("fail") // want "call to log.Fatal is not allowed outside main function of package main"
}

func fatalfHelper() {
	log.Fatalf("fail: %d", 1) // want "call to log.Fatalf is not allowed outside main function of package main"
}

func fatallnHelper() {
	log.Fatalln("fail") // want "call to log.Fatalln is not allowed outside main function of package main"
}

func exitHelper() {
	os.Exit(1) // want "call to os.Exit is not allowed outside main function of package main"
}

// shadowedPanic проверяет, что локальная функция с именем panic
// (переопределяющая встроенную) не считается использованием builtin panic.
func shadowedPanic() {
	panic := func(v interface{}) {}
	panic("not a real panic")
}
