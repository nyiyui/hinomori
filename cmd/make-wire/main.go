package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/pkg/profile"

	"github.com/nyiyui/opt/hinomori/wire"
)

func main() {
	var root string
	var block string
	var hash bool
	var prof bool
	flag.StringVar(&root, "root", "/", "root of tree")
	flag.StringVar(&block, "block", "[]", "paths to block in JSON")
	flag.BoolVar(&hash, "hash", false, "hash all files")
	flag.BoolVar(&prof, "prof", false, "enable profiling")
	flag.Parse()

	if prof {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}

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

	ch := make(chan *wire.WalkStep)
	go func() {
		err := walker.Walk2(root, ch)
		if err != nil {
			log.Fatalf("walk: %s", err)
		}
	}()
	out := bufio.NewWriter(os.Stdout)
	_, err := fmt.Fprintf(out, wire.WireMagic)
	if err != nil {
		log.Printf("writing magic: %s", err)
	}
	for step := range ch {
		err = wire.EncodeStep(out, &step.Step)
		if err != nil {
			log.Printf("%s: %s", step.AbsPath, err)
		}
	}
}
