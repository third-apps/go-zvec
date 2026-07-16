package storage

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/third-apps/go-zvec/status"
	"github.com/third-apps/go-zvec/types"
)

type StorageType = types.StorageType

const (
	StorageNone       = types.StorageTypeNone
	StorageMMAP       = types.StorageTypeMMAP
	StorageMemory     = types.StorageTypeMemory
	StorageBufferPool = types.StorageTypeBufferPool
)

type StorageOptions struct {
	Type      StorageType
	CreateNew bool
	ReadOnly  bool
}

type Storage interface {
	Read(offset int64, data []byte) (int, error)
	Write(offset int64, data []byte) error
	Sync() error
	Size() (int64, error)
	Close() error
}

type FileStorage struct {
	f    *os.File
	path string
	opts StorageOptions
}

func OpenFileStorage(path string, opts StorageOptions) (*FileStorage, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	var flags int
	if opts.ReadOnly {
		flags = os.O_RDONLY
	} else if opts.CreateNew {
		flags = os.O_RDWR | os.O_CREATE | os.O_TRUNC
	} else {
		flags = os.O_RDWR | os.O_CREATE
	}

	f, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return nil, err
	}

	return &FileStorage{f: f, path: path, opts: opts}, nil
}

func (s *FileStorage) Read(offset int64, data []byte) (int, error) {
	return s.f.ReadAt(data, offset)
}

func (s *FileStorage) Write(offset int64, data []byte) error {
	_, err := s.f.WriteAt(data, offset)
	return err
}

func (s *FileStorage) Sync() error {
	return s.f.Sync()
}

func (s *FileStorage) Size() (int64, error) {
	info, err := s.f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (s *FileStorage) Close() error {
	return s.f.Close()
}

type MemoryStorage struct {
	data []byte
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{data: make([]byte, 0)}
}

func (s *MemoryStorage) Read(offset int64, data []byte) (int, error) {
	if offset >= int64(len(s.data)) {
		return 0, status.NewInternalError("read out of bounds").GoError()
	}
	n := copy(data, s.data[offset:])
	return n, nil
}

func (s *MemoryStorage) Write(offset int64, data []byte) error {
	end := offset + int64(len(data))
	if end > int64(len(s.data)) {
		newCap := max(int64(len(s.data))*2, end)
		newData := make([]byte, newCap)
		copy(newData, s.data)
		s.data = newData[:end]
	}
	copy(s.data[offset:], data)
	return nil
}

func (s *MemoryStorage) Sync() error {
	return nil
}

func (s *MemoryStorage) Size() (int64, error) {
	return int64(len(s.data)), nil
}

func (s *MemoryStorage) Close() error {
	s.data = nil
	return nil
}

type MMAPStorage struct {
	data      []byte
	file      *os.File
	mapHandle syscall.Handle
	basePtr   uintptr
	size      int64
	path      string
	opts      StorageOptions
}

func NewMMAPStorage(path string, size int64, opts StorageOptions) (*MMAPStorage, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	var access uint32 = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	var create uint32 = syscall.OPEN_ALWAYS
	if opts.ReadOnly {
		access = syscall.GENERIC_READ
	}
	if opts.CreateNew {
		create = syscall.CREATE_ALWAYS
	}

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	handle, err := syscall.CreateFile(pathPtr, access, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil, create, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}

	f := os.NewFile(uintptr(handle), path)

	if !opts.ReadOnly && size > 0 {
		if err := f.Truncate(size); err != nil {
			f.Close()
			return nil, err
		}
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	actualSize := info.Size()

	var maxSizeHigh uint32
	var maxSizeLow uint32
	maxSizeLow = uint32(uint64(actualSize) & 0xFFFFFFFF)
	maxSizeHigh = uint32(uint64(actualSize) >> 32)

	var prot uint32 = syscall.PAGE_READWRITE
	var mapAccess uint32 = syscall.FILE_MAP_WRITE
	if opts.ReadOnly {
		prot = syscall.PAGE_READONLY
		mapAccess = syscall.FILE_MAP_READ
	}

	mapHandle, err := syscall.CreateFileMapping(handle, nil, prot, maxSizeHigh, maxSizeLow, nil)
	if err != nil {
		f.Close()
		return nil, err
	}

	ptr, err := syscall.MapViewOfFile(mapHandle, mapAccess, 0, 0, uintptr(actualSize))
	if err != nil {
		syscall.CloseHandle(mapHandle)
		f.Close()
		return nil, err
	}

	var p *byte
	*(*uintptr)(unsafe.Pointer(&p)) = ptr
	data := unsafe.Slice(p, int(actualSize))

	return &MMAPStorage{
		data:      data,
		file:      f,
		mapHandle: mapHandle,
		basePtr:   ptr,
		size:      actualSize,
		path:      path,
		opts:      opts,
	}, nil
}

func (s *MMAPStorage) Read(offset int64, data []byte) (int, error) {
	if offset >= s.size {
		return 0, status.NewInternalError("read out of bounds").GoError()
	}
	n := copy(data, s.data[offset:])
	return n, nil
}

func (s *MMAPStorage) Write(offset int64, data []byte) error {
	if offset+int64(len(data)) > s.size {
		return status.NewInternalError("write out of bounds").GoError()
	}
	copy(s.data[offset:], data)
	return nil
}

func (s *MMAPStorage) Sync() error {
	if err := syscall.FlushViewOfFile(s.basePtr, uintptr(len(s.data))); err != nil {
		return err
	}
	return s.file.Sync()
}

func (s *MMAPStorage) Size() (int64, error) {
	return s.size, nil
}

func (s *MMAPStorage) Close() error {
	if s.data != nil {
		syscall.UnmapViewOfFile(s.basePtr)
		s.data = nil
	}
	if s.mapHandle != 0 {
		syscall.CloseHandle(s.mapHandle)
		s.mapHandle = 0
	}
	return s.file.Close()
}

func (s *MMAPStorage) Data() []byte {
	return s.data
}

var _ Storage = (*FileStorage)(nil)
var _ Storage = (*MemoryStorage)(nil)
var _ Storage = (*MMAPStorage)(nil)
