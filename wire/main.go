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
	"regexp"
	"strings"

	"golang.org/x/exp/constraints"

	"github.com/gammazero/deque"
)

type FileInfo struct {
	AbsPath string
	Info    fs.FileInfo
	up      uint32
	down    string
	hash    []byte
	hashErr error
}

type FileInfo2 struct {
	Mode fs.FileMode
	Size uint64
	Name string
	Path string
	Hash []byte
}

func (f *FileInfo2) String() string {
	b := new(strings.Builder)
	fmt.Fprintf(b, "%s %d %16x %s %s", f.Mode, f.Size, f.Hash, f.Path, f.Name)
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
		_, err = fmt.Fprint(w, name)
		if err != nil {
			return err
		}
	}
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(len(fi.hash)))
	_, err := w.Write(b[:])
	if err != nil {
		return err
	}
	if len(fi.hash) != 0 {
		_, err = fmt.Fprint(w, string(fi.hash))
		if err != nil {
			return err
		}
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
	{
		var b [4]byte
		_, err := r.Read(b[:])
		if err != nil {
			return err
		}
		hashSize := binary.LittleEndian.Uint32(b[:])
		hash := make([]byte, hashSize)
		_, err = r.Read(hash)
		if err != nil {
			return err
		}
		fi.Hash = hash
	}
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
		if a[i] != b[i] {
			return i
		}
	}
	return ln
}

type Walker struct {
	blockedPaths []*regexp.Regexp
	hash         bool
}

var defaultBlockedPaths = []*regexp.Regexp{
	regexp.MustCompile("^/dev.*"),
	// /dev/console, /dev/stdin, /dev/u?random, etc
	regexp.MustCompile("^/proc.*"),
}

// NewWalker returns a new Walker with sane default.
func NewWalker() *Walker {
	w := new(Walker)
	w.Block(defaultBlockedPaths)
	return w
}

func (w *Walker) Block(paths []*regexp.Regexp) {
	w.blockedPaths = append(w.blockedPaths, paths...)
}

func (w *Walker) isBlocked(path string) bool {
	for _, blocked := range w.blockedPaths {
		if blocked.Match([]byte(path)) {
			return true
		}
	}
	return false
}

func (w *Walker) Walk(path string, files chan<- FileInfo) error {
	defer close(files)
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
			fi := FileInfo{
				AbsPath: name,
				Info:    entry,
			}
			if w.isBlocked(name) {
				continue
			}
			if w.hash && safeMode(entry.Mode()) {
				fi.hash, err = w.makeHash(name)
				if err != nil {
					fi.hashErr = err
					log.Printf("hash %s: %s", name, err)
				}
			}
			if i == 0 {
				if up != 0 {
					fi.up = up
					log.Printf("up %s", item.Name)
				}
				if down != "" {
					fi.down = down
				}
			}
			files <- fi
			if entry.IsDir() {
				q.PushBack(qItem{Name: name})
			}
		}
		prevName = item.Name
	}
	return nil
}

func safeMode(mode fs.FileMode) bool {
	return !mode.IsDir() &&
		!(mode&(fs.ModeDevice|fs.ModeNamedPipe|fs.ModeSocket|fs.ModeIrregular) == 0)
}
