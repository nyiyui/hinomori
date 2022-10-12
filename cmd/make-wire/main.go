package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nyiyui/opt/hinomori/wire"
)

func main() {
	var root string
	flag.StringVar(&root, "root", "/", "root of tree")
	flag.Parse()

	ch := make(chan wire.FileInfo)
	go func() {
		err := wire.Walk(root, ch)
		if err != nil {
			log.Fatal(err)
		}
	}()
	fmt.Fprintf(os.Stdout, wire.WireMagic)
	for file := range ch {
		err := wire.EncodeWire(os.Stdout, file)
		if err != nil {
			log.Printf("%s: %s", file.AbsPath, err)
		}
	}
}
