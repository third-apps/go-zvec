package persist

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const MagicNumber uint32 = 0x5A564543

type IndexType uint8

const (
	IndexTypeFlat       IndexType = 1
	IndexTypeHNSW       IndexType = 2
	IndexTypeVamana     IndexType = 3
	IndexTypeIVF        IndexType = 4
	IndexTypeDiskAnn    IndexType = 5
	IndexTypeHNSWRabitQ IndexType = 6
)

type FileHeader struct {
	Magic     uint32
	Version   uint32
	IndexType IndexType
}

func WriteHeader(w io.Writer, h FileHeader) error {
	if err := binary.Write(w, binary.LittleEndian, h.Magic); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, h.Version); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, h.IndexType)
}

func ReadHeader(r io.Reader) (FileHeader, error) {
	var h FileHeader
	if err := binary.Read(r, binary.LittleEndian, &h.Magic); err != nil {
		return h, err
	}
	if err := binary.Read(r, binary.LittleEndian, &h.Version); err != nil {
		return h, err
	}
	if err := binary.Read(r, binary.LittleEndian, &h.IndexType); err != nil {
		return h, err
	}
	if h.Magic != MagicNumber {
		return h, fmt.Errorf("invalid magic number: got 0x%08X, expected 0x%08X", h.Magic, MagicNumber)
	}
	return h, nil
}

func WriteUint32(w io.Writer, v uint32) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func ReadUint32(r io.Reader) (uint32, error) {
	var v uint32
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

func WriteUint64(w io.Writer, v uint64) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func ReadUint64(r io.Reader) (uint64, error) {
	var v uint64
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

func WriteFloat32(w io.Writer, v float32) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func ReadFloat32(r io.Reader) (float32, error) {
	var v float32
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

func WriteByte(w io.Writer, v byte) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func WriteInt(w io.Writer, v int) error {
	return binary.Write(w, binary.LittleEndian, int32(v))
}

func ReadInt(r io.Reader) (int, error) {
	var v int32
	err := binary.Read(r, binary.LittleEndian, &v)
	return int(v), err
}

func WriteInt64(w io.Writer, v int64) error {
	return binary.Write(w, binary.LittleEndian, v)
}

func ReadInt64(r io.Reader) (int64, error) {
	var v int64
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

func WriteString(w io.Writer, s string) error {
	if err := WriteUint32(w, uint32(len(s))); err != nil {
		return err
	}
	_, err := w.Write([]byte(s))
	return err
}

func ReadString(r io.Reader) (string, error) {
	n, err := ReadUint32(r)
	if err != nil {
		return "", err
	}
	if n > 1<<30 {
		return "", fmt.Errorf("string too long: %d bytes", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func WriteFloat32Slice(w io.Writer, s []float32) error {
	if err := WriteUint32(w, uint32(len(s))); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, s)
}

func ReadFloat32Slice(r io.Reader) ([]float32, error) {
	n, err := ReadUint32(r)
	if err != nil {
		return nil, err
	}
	if n > 1<<28 {
		return nil, fmt.Errorf("float32 slice too long: %d", n)
	}
	s := make([]float32, n)
	if err := binary.Read(r, binary.LittleEndian, s); err != nil {
		return nil, err
	}
	return s, nil
}

func WriteUint64Slice(w io.Writer, s []uint64) error {
	if err := WriteUint32(w, uint32(len(s))); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, s)
}

func ReadUint64Slice(r io.Reader) ([]uint64, error) {
	n, err := ReadUint32(r)
	if err != nil {
		return nil, err
	}
	if n > 1<<28 {
		return nil, fmt.Errorf("uint64 slice too long: %d", n)
	}
	s := make([]uint64, n)
	if err := binary.Read(r, binary.LittleEndian, s); err != nil {
		return nil, err
	}
	return s, nil
}

func WriteIntSlice(w io.Writer, s []int) error {
	if err := WriteUint32(w, uint32(len(s))); err != nil {
		return err
	}
	for _, v := range s {
		if err := WriteInt(w, v); err != nil {
			return err
		}
	}
	return nil
}

func ReadIntSlice(r io.Reader) ([]int, error) {
	n, err := ReadUint32(r)
	if err != nil {
		return nil, err
	}
	if n > 1<<28 {
		return nil, fmt.Errorf("int slice too long: %d", n)
	}
	s := make([]int, n)
	for i := range s {
		s[i], err = ReadInt(r)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func WriteByteSlice(w io.Writer, s []byte) error {
	if err := WriteUint32(w, uint32(len(s))); err != nil {
		return err
	}
	_, err := w.Write(s)
	return err
}

func ReadByteSlice(r io.Reader) ([]byte, error) {
	n, err := ReadUint32(r)
	if err != nil {
		return nil, err
	}
	if n > 1<<30 {
		return nil, fmt.Errorf("byte slice too long: %d", n)
	}
	s := make([]byte, n)
	if _, err := io.ReadFull(r, s); err != nil {
		return nil, err
	}
	return s, nil
}

func WriteBoolSlice(w io.Writer, s []bool) error {
	if err := WriteUint32(w, uint32(len(s))); err != nil {
		return err
	}
	buf := make([]byte, (len(s)+7)/8)
	for i, v := range s {
		if v {
			buf[i/8] |= 1 << uint(i%8)
		}
	}
	_, err := w.Write(buf)
	return err
}

func ReadBoolSlice(r io.Reader) ([]bool, error) {
	n, err := ReadUint32(r)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	if n > 1<<28 {
		return nil, fmt.Errorf("bool slice too long: %d", n)
	}
	bufLen := (int(n) + 7) / 8
	buf := make([]byte, bufLen)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	s := make([]bool, n)
	for i := range s {
		s[i] = buf[i/8]&(1<<uint(i%8)) != 0
	}
	return s, nil
}

func SaveToFile(path string, fn func(w *bufio.Writer) error) error {
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	if err := fn(bw); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := bw.Flush(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := f.Sync(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	f.Close()
	return os.Rename(tmpPath, path)
}

func LoadFromFile(path string, fn func(r *bufio.Reader) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return fn(bufio.NewReader(f))
}
