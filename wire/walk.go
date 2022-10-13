package wire

import (
	"fmt"
	"io/ioutil"
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
	for q.Len() != 0 {
		item := q.PopFront()
		entries, err := ioutil.ReadDir(item.Name)
		if err != nil {
			return err
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
		for i, entry := range entries {
			name := filepath.Join(item.Name, entry.Name())
			if w.isBlocked(name) {
				continue
			}
			var hash []byte
			var hashErr error
			if w.hash && safeMode(entry.Mode()) {
				hash, hashErr = w.makeHash(name)
				if hashErr != nil {
					log.Printf("hash %s: %s", name, err)
				}
			}
			steps <- WalkStep{
				Step: pb.Step{
					Step: &pb.Step_File{
						File: &pb.StepFile{
							Mode:    uint32(entry.Mode()),
							Size:    uint64(entry.Size()),
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
			if entry.IsDir() {
				q.PushBack(qItem{Name: name})
			}
		}
		prevName = item.Name
	}
	return nil
}
