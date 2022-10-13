package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/nyiyui/opt/hinomori/wire"
)

func main() {
	var count int
	ch := make(chan wire.FileInfo2)
	go func() {
		for f := range ch {
			fmt.Printf("%11s %8d %16x %s\n", f.Mode, f.Size, f.Hash, filepath.Join(f.Path, f.Name))
			count++
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
		if errors.Is(err, io.EOF) {
			log.Print("EOF")
			log.Printf("read %d files", count)
			return
		}
		log.Fatal(err)
	}
}
