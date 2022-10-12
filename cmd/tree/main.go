package main

import (
	"fmt"
	"log"
	"os"

	"github.com/nyiyui/opt/hinomori/wire"
)

func main() {
	ch := make(chan wire.FileInfo2)
	go func() {
		for file := range ch {
			fmt.Printf("%s\n", &file)
		}
	}()
	var b [4]byte
	_, err := os.Stdin.Read(b[:])
	if err != nil {
		log.Fatal(err)
	}
	if string(b[:]) != wire.WireMagic {
		log.Fatal("no magic")
	}
	err = wire.DecodeWire(os.Stdin, ch)
	if err != nil {
		log.Fatal(err)
	}
}
