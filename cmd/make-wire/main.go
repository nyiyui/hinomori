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

func jsonPaths(s string) ([]*regexp.Regexp, error) {
	paths := make([]string, 0)
	err := json.Unmarshal([]byte(s), &paths)
	if err != nil {
		return nil, err
	}
	paths2 := make([]*regexp.Regexp, len(paths))
	for i, path := range paths {
		paths2[i], err = regexp.Compile(path)
		if err != nil {
			return nil, fmt.Errorf("path %d: %w", i, err)
		}
	}
	return paths2, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	var root string
	var block string
	var hashAll bool
	var hash string
	var prof bool
	flag.StringVar(&root, "root", "/", "root of tree")
	flag.StringVar(&block, "block", "[]", "paths to block in JSON")
	flag.BoolVar(&hashAll, "hash-all", false, "hash all files")
	flag.StringVar(&hash, "hash", "[]", "paths to hash in JSON")
	flag.BoolVar(&prof, "prof", false, "enable profiling")
	flag.Parse()

	if prof {
		defer profile.Start(profile.ProfilePath(".")).Stop()
	}

	walker := wire.NewWalker()
	walker.HashAll(hashAll)
	paths, err := jsonPaths(hash)
	if err != nil {
		log.Fatalf("hash paths: %s", err)
	}
	walker.Hash(paths)
	paths, err = jsonPaths(block)
	if err != nil {
		log.Fatalf("block paths: %s", err)
	}
	walker.Block(paths)

	out := bufio.NewWriter(os.Stdout)
	_, err = fmt.Fprintf(out, wire.WireMagic)
	if err != nil {
		log.Printf("writing magic: %s", err)
	}
	defer out.Flush()
	err = walker.Walk2(root, out)
	if err != nil {
		log.Fatalf("walk: %s", err)
	}
}
