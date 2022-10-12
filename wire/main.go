package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/exp/constraints"

	"github.com/gammazero/deque"
)

type FileInfo struct {
	AbsPath string
	Info    fs.FileInfo
	up      uint32
	down    string
}

type FileInfo2 struct {
	Mode fs.FileMode
	Size uint64
	Name string
	Path string
}

func (f *FileInfo2) String() string {
	b := new(strings.Builder)
	fmt.Fprintf(b, "%s %d %s %s", f.Mode, f.Size, f.Path, f.Name)
	return b.String()
}

const (
	WireTypeInvalid = iota
	WireTypeFile
	WireTypePathUp
	WireTypePathDown
)

const WireMagic string = "hino"

func EncodeWire(w io.Writer, fi FileInfo) error {
	if fi.up != 0 {
		var b [1 + 4]byte
		b[0] = WireTypePathUp
		binary.LittleEndian.PutUint32(b[1:], fi.up)
		_, err := w.Write(b[:])
		if err != nil {
			return err
		}
	}
	if fi.down != "" {
		var b [1 + 4]byte
		b[0] = WireTypePathDown
		binary.LittleEndian.PutUint32(b[1:], uint32(len(fi.down)))
		_, err := w.Write(b[:])
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(w, fi.down)
		if err != nil {
			return err
		}
	}
	if fi.Info != nil {
		var b [1 + 4 + 8 + 4]byte
		b[0] = WireTypeFile
		binary.LittleEndian.PutUint32(b[1:], uint32(fi.Info.Mode()))
		binary.LittleEndian.PutUint64(b[1+4:], uint64(fi.Info.Size()))
		name := fi.Info.Name()
		binary.LittleEndian.PutUint32(b[1+4+8:], uint32(len(name)))
		_, err := w.Write(b[:])
		if err != nil {
			return err
		}
		fmt.Fprint(w, name)
	}
	return nil
}

func DecodeWire(r io.Reader, ch chan<- FileInfo2) error {
	defer close(ch)
	currentPath := "/"
	for {
		var b [1]byte
		_, err := r.Read(b[:])
		if err != nil {
			return err
		}
		switch b[0] {
		case WireTypeFile:
			var fi FileInfo2
			err = decodeWireFile(r, &fi)
			if err != nil {
				return err
			}
			fi.Path = currentPath
			ch <- fi
		case WireTypePathUp:
			var b [4]byte
			_, err := r.Read(b[:])
			if err != nil {
				return err
			}
			up := int(binary.LittleEndian.Uint32(b[:]))
			for i := 0; i < up; i++ {
				currentPath = filepath.Join(currentPath, "..")
			}
		case WireTypePathDown:
			var b [4]byte
			_, err := r.Read(b[:])
			if err != nil {
				return err
			}
			nameSize := int(binary.LittleEndian.Uint32(b[:]))
			name := make([]byte, nameSize)
			_, err = r.Read(name)
			if err != nil {
				return err
			}
			currentPath = filepath.Join(currentPath, string(name))
		default:
		case WireTypeInvalid:
			return errors.New("invalid WireType")
		}
	}
}

func decodeWireFile(r io.Reader, fi *FileInfo2) error {
	var b [4 + 8 + 4]byte
	_, err := r.Read(b[:])
	if err != nil {
		return err
	}
	fi.Mode = fs.FileMode(binary.LittleEndian.Uint32(b[:]))
	fi.Size = binary.LittleEndian.Uint64(b[4:])
	nameSize := binary.LittleEndian.Uint32(b[4+8:])
	name := make([]byte, nameSize)
	_, err = r.Read(name)
	if err != nil {
		return err
	}
	fi.Name = string(name)
	return nil
}

type qItem struct {
	End       bool
	Down      string
	DownCount int
	Name      string
}

func max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func common[T comparable](a, b []T) int {
	la, lb := len(a), len(b)
	lm := max(la, lb)
	ln := min(la, lb)
	for i := 0; i < lm; i++ {
		if i >= ln {
			return i
		}
		log.Printf("if %d %v %v", i, a, b)
		if a[i] != b[i] {
			return i
		}
	}
	return ln
}

func Walk(path string, files chan<- FileInfo) error {
	defer close(files)
	//path, err := filepath.Abs(path)
	//if err != nil {
	//	return err
	//}
	var q deque.Deque[qItem]
	q.PushBack(qItem{Name: path})
	var prevName string
	for q.Len() != 0 {
		item := q.PopFront()
		log.Printf("qItem %#v", item)
		entries, err := ioutil.ReadDir(item.Name)
		if err != nil {
			return err
		}
		var up uint32
		var down string
		if prevName != item.Name {
			log.Printf("down %s %s", prevName, item.Name)
			a := strings.Split(prevName, string(os.PathSeparator))
			if len(a) == 1 && a[0] == "" {
				a = []string{}
			}
			b := strings.Split(item.Name, string(os.PathSeparator))
			if len(b) == 1 && b[0] == "" {
				b = []string{}
			}
			lc := common(a, b)
			log.Printf("common %d %s", len(a[:lc]), a[:lc])
			log.Printf("uncommonA %d %s", len(a[lc:]), a[lc:])
			log.Printf("uncommonB %d %s", len(b[lc:]), b[lc:])
			down = strings.Join(b[lc:], string(os.PathSeparator))
			up = uint32(len(a[lc:]))
			log.Printf("up %d", up)
			log.Printf("down %s", down)
		}
		for i, entry := range entries {
			name := filepath.Join(item.Name, entry.Name())
			fi := FileInfo{
				AbsPath: name,
				Info:    entry,
			}
			if i == 0 {
				if up != 0 {
					fi.up = up
				}
				if down != "" {
					fi.down = down
				}
			}
			files <- fi
			log.Printf("files %#v", fi)
			//log.Printf("i %d len %d", i, len(entries))
			if entry.IsDir() {
				//q.PushBack(qItem{Down: entry.Name(), DownCount: item.DownCount + 1, Name: name})
				q.PushBack(qItem{Name: name})
			}
		}
		prevName = item.Name
	}
	return nil
}
