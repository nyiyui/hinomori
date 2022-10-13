package wire

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/cespare/xxhash"
)

func (w *Walker) Hash(hash bool) {
	w.hash = hash
}

func (w *Walker) makeHash(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer func() {
		err2 := f.Close()
		if err2 != nil {
			err = err2
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	digest := xxhash.New()
	_, err = io.Copy(digest, f)
	if err != nil {
		return nil, fmt.Errorf("copy: %w", err)
	}
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], digest.Sum64())
	return b[:], nil
}
