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

const counterCutoff = 65536

type WalkStep struct {
	Step    pb.Step
	AbsPath string
}

type stepRes struct {
	File    bool
	Mode    fs.FileMode
	Size    int64
	Name    string
	Hash    []byte
	HashErr string
	AbsPath string

	Up uint32

	Down string
}

func (w *Walker) Walk2(path string, steps chan<- *WalkStep) {
	stepRess := make(chan stepRes)
	go w.walk2(path, stepRess)
	for res := range stepRess {
		if res.Up != 0 {
			steps <- &WalkStep{
				Step: pb.Step{
					Step: &pb.Step_Up{
						Up: &pb.StepPathUp{
							Up: res.Up,
						},
					},
				},
			}
		}
		if res.Down != "" {
			steps <- &WalkStep{
				Step: pb.Step{
					Step: &pb.Step_Down{
						Down: &pb.StepPathDown{
							Down: res.Down,
						},
					},
				},
			}
		}
		if res.File {
			steps <- &WalkStep{
				Step: pb.Step{
					Step: &pb.Step_File{
						File: &pb.StepFile{
							Mode:    uint32(res.Mode),
							Size:    uint64(res.Size),
							Name:    res.Name,
							Hash:    res.Hash,
							HashErr: res.HashErr,
						},
					},
				},
				AbsPath: res.Name,
			}
		}
	}
}

func (w *Walker) walk2(path string, stepRess chan<- stepRes) {
	defer close(stepRess)
	var q deque.Deque[qItem]
	q.PushBack(qItem{Name: path})
	var prevName string
	counter := 0
	showCounterNext := 1
	for q.Len() != 0 {
		counter++
		if counter == showCounterNext {
			log.Printf("progress: %d of current %d", counter, q.Len())
			if showCounterNext < counterCutoff {
				showCounterNext *= 2
			} else {
				showCounterNext += counterCutoff
			}
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
				if w.hash && safeMode(info.Mode()) && (w.hashPaths == nil || w.isHashPath(name)) {
					hash, hashErr = w.makeHash(name)
					if hashErr != nil {
						log.Printf("hash %s: %s", name, hashErr)
					}
				}
				res := stepRes{
					File:    true,
					Mode:    info.Mode(),
					Size:    info.Size(),
					Name:    entry.Name(),
					Hash:    hash,
					HashErr: fmt.Sprint(hashErr),
					AbsPath: name,
				}
				if i == 0 {
					if up != 0 {
						res.Up = up
					}
					if down != "" {
						res.Down = down
					}
				}
				stepRess <- res
			}(i, entry)
		}
		prevName = item.Name
	}
}
