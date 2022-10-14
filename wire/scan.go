package wire

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

type qItem2 struct {
	Name string
}

type scanRes struct {
	Mode    fs.FileMode
	Size    int64
	Name    string
	Hash    []byte
	HashErr string

	AbsPath string
}

type walk2ScanState struct {
	q     chan qItem2
	steps chan<- scanRes
}

// TODO: concurrent scan

func (w *Walker) walk2Scan(s *walk2ScanState) {
	for item := range s.q {
		/*
			counter++
			if counter == showCounterNext {
				log.Printf("progress: %d of current %d", counter, s.q.Len())
				showCounterNext *= 2
				showCounterNext = min(16384, showCounterNext)
			}
		*/
		entries, err := os.ReadDir(item.Name)
		if err != nil {
			log.Printf("read %s: %s", item.Name, err)
		}
		for _, entry := range entries {
			name := filepath.Join(item.Name, entry.Name())
			if w.isBlocked(name) {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				log.Printf("info %s: %s", name, err)
				continue
			}
			if !(entry.IsDir() || info.Mode().IsRegular()) {
				continue
			}
			var hash []byte
			var hashErr error
			if w.hash && safeMode(info.Mode()) {
				hash, hashErr = w.makeHash(name)
				if hashErr != nil {
					log.Printf("hash %s: %s", name, hashErr)
				}
			}
			s.steps <- scanRes{
				Mode:    info.Mode(),
				Size:    info.Size(),
				Name:    entry.Name(),
				Hash:    hash,
				HashErr: fmt.Sprint(hashErr),
				AbsPath: name,
			}
			if entry.IsDir() {
				s.q <- qItem2{Name: name}
			}
		}
	}
}
