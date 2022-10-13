package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/nyiyui/opt/hinomori/wire"
	"github.com/nyiyui/opt/hinomori/wire/pb"
)

func main() {
	var count int
	stepCh := make(chan *pb.Step)
	fiCh := make(chan wire.FileInfo2)
	errCh := make(chan error)
	// DecodeSteps -[stepCh]> ConvertSteps -[fiCh]> fmt.Printf
	//                                     -[errCh]> log.Printf
	go func() {
		wire.ConvertSteps(stepCh, fiCh, errCh)
	}()
	go func() {
		for err := range errCh {
			log.Printf("convert: %s", err)
		}
	}()
	go func() {
		for f := range fiCh {
			fmt.Printf("%11s %8d %16x %s\n", f.Mode, f.Size, f.Hash, filepath.Join(f.Path, f.Name))
			count++
		}
	}()
	err := wire.DecodeSteps(os.Stdin, stepCh)
	if err != nil {
		if errors.Is(err, io.EOF) {
			log.Printf("read %d files", count)
			return
		}
		log.Fatal(err)
	}
}
