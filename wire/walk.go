package wire

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gammazero/deque"
	"github.com/nyiyui/opt/hinomori/wire/pb"
)

type WalkStep struct {
	Step    pb.Step
	AbsPath string
}

func (w *Walker) Walk2(path string, steps chan<- WalkStep) error {
	defer close(steps)
	var q deque.Deque[qItem]
	q.PushBack(qItem{Name: path})
	var prevName string
	counter := 0
	showCounterNext := 1
	for q.Len() != 0 {
		counter++
		if counter == showCounterNext {
			log.Printf("progress: %d of current %d", counter, q.Len())
			showCounterNext *= 2
			showCounterNext = min(16384, showCounterNext)
		}
		item := q.PopFront()
		entries, err := os.ReadDir(item.Name)
		if err != nil {
			log.Printf("read %s: %s", item.Name, err)
		}
		var up uint32
		var down string
		if prevName != item.Name {
			a := strings.Split(prevName, string(os.PathSeparator))
			if len(a) == 1 && a[0] == "" {
				a = []string{}
			}
			b := strings.Split(item.Name, string(os.PathSeparator))
			if len(b) == 1 && b[0] == "" {
				b = []string{}
			}
			lc := common(a, b)
			down = strings.Join(b[lc:], string(os.PathSeparator))
			up = uint32(len(a[lc:]))
		}
		names := make([]string, len(entries))
		for i, entry := range entries {
			name := filepath.Join(item.Name, entry.Name())
			if w.isBlocked(name) {
				continue
			}
			names[i] = name
			if entry.IsDir() {
				q.PushBack(qItem{Name: names[i]})
			}
		}
		for i, entry := range entries {
			go func(i int, entry fs.DirEntry) {
				name := names[i]
				if name == "" {
					return
				}
				info, err := entry.Info()
				if err != nil {
					log.Printf("info %s: %s", name, err)
					return
				}
				if !(entry.IsDir() || info.Mode().IsRegular()) {
					return
				}
				var hash []byte
				var hashErr error
				if w.hash && safeMode(info.Mode()) {
					hash, hashErr = w.makeHash(name)
					if hashErr != nil {
						log.Printf("hash %s: %s", name, hashErr)
					}
				}
				steps <- WalkStep{
					Step: pb.Step{
						Step: &pb.Step_File{
							File: &pb.StepFile{
								Mode:    uint32(info.Mode()),
								Size:    uint64(info.Size()),
								Name:    entry.Name(),
								Hash:    hash,
								HashErr: fmt.Sprint(hashErr),
							},
						},
					},
					AbsPath: name,
				}
				if i == 0 {
					if up != 0 {
						steps <- WalkStep{
							Step: pb.Step{
								Step: &pb.Step_Up{
									Up: &pb.StepPathUp{
										Up: up,
									},
								},
							},
						}
					}
					if down != "" {
						steps <- WalkStep{
							Step: pb.Step{
								Step: &pb.Step_Down{
									Down: &pb.StepPathDown{
										Down: down,
									},
								},
							},
						}
					}
				}
			}(i, entry)
		}
		prevName = item.Name
	}
	return nil
}

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
