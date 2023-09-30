package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/nyiyui/opt/hinomori/wire"
	"github.com/nyiyui/opt/hinomori/wire/pb"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "%s < [wire.hino] > [human-readable tree]\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

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
		stdin := bufio.NewReader(os.Stdin)
		err := wire.DecodeSteps(stdin, stepCh)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Printf("read %d files", count)
				return
			}
			log.Fatal(err)
		}
	}()
	log.Printf("waiting for input...")
	fmt.Printf("%11s %8s %6s %6s %16s %s\n", "mode", "size", "own", "grp", "hash", "path")
	for f := range fiCh {
		fmt.Printf("%11s %8d %6d %6d %16x %s\n", f.Mode, f.Size, f.Owner, f.Group, f.Hash, filepath.Join(f.Path, f.Name))
		count++
	}
}
