package wire

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	"github.com/nyiyui/opt/hinomori/wire/pb"
	"google.golang.org/protobuf/proto"
)

// EncodeStep encodes pb.Step into "step" wire format, described in wire.md.
func EncodeStep(w io.Writer, wire *pb.Step) error {
	out, err := proto.Marshal(wire)
	if err != nil {
		return err
	}
	var out2 [8]byte
	binary.LittleEndian.PutUint64(out2[:], uint64(len(out)))
	_, err = w.Write(out2[:])
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	if err != nil {
		return err
	}
	return nil
}

// DecodeSteps decodes the "step" wire format into pb.Step, described in wire.md.
func DecodeSteps(r io.Reader, steps chan<- *pb.Step) error {
	defer close(steps)
	var magic [4]byte
	_, err := r.Read(magic[:])
	if err != nil {
		return err
	}
	if string(magic[:]) != "hino" {
		return errors.New("invalid magic")
	}
	for {
		step, err := decodeSingle(r)
		if err != nil {
			return err
		}
		steps <- step
	}
}

func decodeSingle(r io.Reader) (*pb.Step, error) {
	var lenBytes [8]byte
	_, err := io.ReadFull(r, lenBytes[:])
	if err != nil {
		return nil, fmt.Errorf("read size: %w", err)
	}
	size := binary.LittleEndian.Uint64(lenBytes[:])
	buf := make([]byte, size)
	n, err := io.ReadFull(r, buf)
	if err != nil {
		return nil, fmt.Errorf("read data of size %d: %w", size, err)
	}
	if n != int(size) {
		return nil, fmt.Errorf("underread: want %d got %d", size, n)
	}
	var step pb.Step
	err = proto.Unmarshal(buf, &step)
	if err != nil {
		return nil, err
	}
	return &step, nil
}

// ConvertSteps converts a channel of pb.Step into FileInfo2.
func ConvertSteps(in <-chan *pb.Step, out chan<- FileInfo2, errs chan<- error) {
	defer close(out)
	defer close(errs)
	currentPath := "/"
	for step := range in {
		switch stepIn := step.Step.(type) {
		case *pb.Step_File:
			f := stepIn.File
			fi := FileInfo2{
				Mode: fs.FileMode(f.Mode),
				Size: f.Size,
				Name: f.Name,
				Path: currentPath,
				Hash: f.Hash,
			}
			out <- fi
		case *pb.Step_Up:
			up := int(stepIn.Up.Up)
			for i := 0; i < up; i++ {
				currentPath = filepath.Join(currentPath, "..")
			}
		case *pb.Step_Down:
			currentPath = filepath.Join(currentPath, string(stepIn.Down.Down))
		default:
			errs <- errors.New("invalid Step")
		}
	}
}
