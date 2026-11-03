package main

import (
	"log"

	"golearn/wire/wire"
)

func main() {
	log.Println("This is wire learning demo")
	wire.InitHandler().Process()

	log.Println("This is wire advanced demo")
	handler, cleanup, err := wire.InitAdvancedHandler()
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()
	handler.Process()
}
