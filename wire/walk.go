package wire

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

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
	Owner   uint32
	Group   uint32

	Up uint32

	Down string
}

func (w *Walker) Walk2(path string, out io.Writer) error {
	stepRess := make(chan stepRes)
	go w.walk2(path, stepRess)
	for res := range stepRess {
		if res.Up != 0 {
			err := EncodeStep(out, &pb.Step{
				Step: &pb.Step_Up{
					Up: &pb.StepPathUp{
						Up: res.Up,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("%s up: %w", res.AbsPath, err)
			}
		}
		if res.Down != "" {
			err := EncodeStep(out, &pb.Step{
				Step: &pb.Step_Down{
					Down: &pb.StepPathDown{
						Down: res.Down,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("%s down: %w", res.AbsPath, err)
			}
		}
		if res.File {
			err := EncodeStep(out, &pb.Step{
				Step: &pb.Step_File{
					File: &pb.StepFile{
						Mode:    uint32(res.Mode),
						Size:    uint64(res.Size),
						Name:    res.Name,
						Hash:    res.Hash,
						HashErr: res.HashErr,
						Own:     res.Owner,
						Grp:     res.Group,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("%s file: %w", res.AbsPath, err)
			}
		}
	}
	log.Printf("finished stepRess")
	return nil
}

func (w *Walker) walk2(path string, stepRess chan<- stepRes) {
	defer close(stepRess)

	stepRess <- stepRes{
		Down: path,
	}

	var q deque.Deque[qItem]
	q.PushBack(qItem{Name: path, First: true})
	var prevName string
	counter := 0
	showCounterNext := 1
	defer func() {
		log.Printf("end counter: %d", counter)
	}()
	for q.Len() != 0 {
		func() {
			var wg sync.WaitGroup
			defer wg.Wait()
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
			{
				var up uint32
				var down string
				if prevName != item.Name && !item.First {
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
					/*
						log.Printf("prevName %s", prevName)
						log.Printf("itemName %s", item.Name)
						log.Printf("down %s", down)
						log.Printf("up %d", up)
					*/
				}
				var res stepRes
				if up != 0 {
					res.Up = up
				}
				if down != "" {
					res.Down = down
				}
				stepRess <- res
			}
			wg.Add(len(entries))
			for i, entry := range entries {
				go func(i int, entry fs.DirEntry) {
					defer wg.Done()
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
					if info.Size() != 0 && (w.hashAll || w.isHashPath(name)) {
						hash, hashErr = w.makeHash(name)
						if hashErr != nil {
							log.Printf("hash %s: %s", name, hashErr)
						}
					}
					hashErr2 := ""
					if hashErr != nil {
						hashErr2 = hashErr.Error()
						// no recover but should be fine enough
					}
					res := stepRes{
						File:    true,
						Mode:    info.Mode(),
						Size:    info.Size(),
						Name:    entry.Name(),
						Hash:    hash,
						HashErr: hashErr2,
						AbsPath: name,
					}
					sys := info.Sys()
					switch sys := sys.(type) {
					case *syscall.Stat_t:
						res.Owner = sys.Uid
						res.Group = sys.Gid
					}
					stepRess <- res
				}(i, entry)
			}
			if item.First {
				prevName = filepath.Join(item.Name, "..")
			} else {
				prevName = item.Name
			}
		}()
	}
}
