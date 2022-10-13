package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/nyiyui/opt/hinomori/wire"
)

func main() {
	var root string
	var block string
	var hash bool
	flag.StringVar(&root, "root", "/", "root of tree")
	flag.StringVar(&block, "block", "[]", "paths to block in JSON")
	flag.BoolVar(&hash, "hash", false, "hash all files")
	flag.Parse()

	walker := wire.NewWalker()
	walker.Hash(hash)
	{
		paths := make([]string, 0)
		err := json.Unmarshal([]byte(block), &paths)
		if err != nil {
			log.Fatal(err)
		}
		paths2 := make([]*regexp.Regexp, len(paths))
		for i, path := range paths {
			paths2[i], err = regexp.Compile(path)
			if err != nil {
				log.Fatalf("path %d: %s", i, err)
			}
		}
		walker.Block(paths2)
	}

	ch := make(chan wire.FileInfo)
	go func() {
		err := walker.Walk(root, ch)
		if err != nil {
			log.Fatalf("walk: %s", err)
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
