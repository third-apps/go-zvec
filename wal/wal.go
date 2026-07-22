package wal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/third-apps/go-zvec/doc"
	"github.com/third-apps/go-zvec/types"
	pb "github.com/third-apps/go-zvec/wal/proto"
	"google.golang.org/protobuf/proto"
)

type OpType string

const (
	OpInsert OpType = "INSERT"
	OpUpsert OpType = "UPSERT"
	OpUpdate OpType = "UPDATE"
	OpDelete OpType = "DELETE"
)

type LogEntry struct {
	LSN   uint64
	Op    OpType
	ID    string
	Doc   *doc.Doc
	IDs   []string
	CRC32 uint32
}

type WAL struct {
	mu           sync.Mutex
	file         *os.File
	writer       *bufio.Writer
	path         string
	readOnly     bool
	nextLSN      uint64
	syncInterval time.Duration
	syncDone     chan struct{}
	syncOnce     sync.Once
}

func Open(path string) (*WAL, error) {
	return OpenWithSyncInterval(path, 0)
}

func OpenWithSyncInterval(path string, syncInterval time.Duration) (*WAL, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	w := bufio.NewWriterSize(f, 64*1024)
	wal := &WAL{
		file:         f,
		path:         path,
		writer:       w,
		syncInterval: syncInterval,
		syncDone:     make(chan struct{}),
	}

	if syncInterval > 0 {
		wal.startSyncLoop()
	}

	return wal, nil
}

func OpenReadOnly(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return &WAL{path: path, readOnly: true}, nil
		}
		return nil, err
	}
	return &WAL{
		file:     f,
		path:     path,
		readOnly: true,
	}, nil
}

func (w *WAL) startSyncLoop() {
	w.syncOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(w.syncInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					w.doSync()
				case <-w.syncDone:
					return
				}
			}
		}()
	})
}

func (w *WAL) doSync() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer != nil {
		w.writer.Flush()
	}
	if w.file != nil {
		w.file.Sync()
	}
}

func (w *WAL) AppendInsert(id string, d *doc.Doc) error {
	return w.appendEntry(LogEntry{Op: OpInsert, ID: id, Doc: d})
}

func (w *WAL) AppendInserts(docs []*doc.Doc) error {
	if w.readOnly {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, d := range docs {
		w.nextLSN++
		entry := LogEntry{Op: OpInsert, ID: d.ID, Doc: d, LSN: w.nextLSN}
		if err := w.writeEntryLocked(entry); err != nil {
			return err
		}
	}
	if err := w.writer.Flush(); err != nil {
		return err
	}
	if w.syncInterval == 0 {
		return w.file.Sync()
	}
	return nil
}

func (w *WAL) AppendUpsert(id string, d *doc.Doc) error {
	return w.appendEntry(LogEntry{Op: OpUpsert, ID: id, Doc: d})
}

func (w *WAL) AppendUpserts(docs []*doc.Doc) error {
	if w.readOnly {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, d := range docs {
		w.nextLSN++
		entry := LogEntry{Op: OpUpsert, ID: d.ID, Doc: d, LSN: w.nextLSN}
		if err := w.writeEntryLocked(entry); err != nil {
			return err
		}
	}
	if err := w.writer.Flush(); err != nil {
		return err
	}
	if w.syncInterval == 0 {
		return w.file.Sync()
	}
	return nil
}

func (w *WAL) AppendUpdate(id string, d *doc.Doc) error {
	return w.appendEntry(LogEntry{Op: OpUpdate, ID: id, Doc: d})
}

func (w *WAL) AppendUpdates(docs []*doc.Doc) error {
	if w.readOnly {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, d := range docs {
		w.nextLSN++
		entry := LogEntry{Op: OpUpdate, ID: d.ID, Doc: d, LSN: w.nextLSN}
		if err := w.writeEntryLocked(entry); err != nil {
			return err
		}
	}
	if err := w.writer.Flush(); err != nil {
		return err
	}
	if w.syncInterval == 0 {
		return w.file.Sync()
	}
	return nil
}

func (w *WAL) AppendDeletes(ids []string) error {
	return w.appendEntry(LogEntry{Op: OpDelete, IDs: ids})
}

func (w *WAL) appendEntry(entry LogEntry) error {
	if w.readOnly {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.nextLSN++
	entry.LSN = w.nextLSN

	if err := w.writeEntryLocked(entry); err != nil {
		return err
	}
	if err := w.writer.Flush(); err != nil {
		return err
	}
	if w.syncInterval == 0 {
		return w.file.Sync()
	}
	return nil
}

func (w *WAL) writeEntryLocked(entry LogEntry) error {
	pbEntry := logEntryToProto(entry)
	pbEntry.Crc32 = 0
	payload, err := proto.Marshal(pbEntry)
	if err != nil {
		return err
	}

	crc := crc32.ChecksumIEEE(payload)

	var lenBuf [4]byte
	var crcBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	binary.LittleEndian.PutUint32(crcBuf[:], crc)

	if _, err := w.writer.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := w.writer.Write(crcBuf[:]); err != nil {
		return err
	}
	if _, err := w.writer.Write(payload); err != nil {
		return err
	}
	return nil
}

func (w *WAL) Sync() error {
	if w.readOnly {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			return err
		}
	}
	if w.file != nil {
		return w.file.Sync()
	}
	return nil
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.syncInterval > 0 && w.syncDone != nil {
		close(w.syncDone)
		w.syncDone = nil
	}

	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			return err
		}
	}
	if w.file != nil {
		if !w.readOnly {
			if err := w.file.Sync(); err != nil {
				return err
			}
		}
		return w.file.Close()
	}
	return nil
}

type ReplayFunc func(entry LogEntry) error

func (w *WAL) Replay(fn ReplayFunc) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			return err
		}
	}

	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}

	var maxLSN uint64
	reader := bufio.NewReader(w.file)
	for {
		var lenBuf [4]byte
		if _, err := io.ReadFull(reader, lenBuf[:]); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("replay: failed to read entry length: %w", err)
		}
		payloadLen := binary.LittleEndian.Uint32(lenBuf[:])
		if payloadLen == 0 || payloadLen > 64*1024*1024 {
			slog.Warn("replay: invalid payload length", "len", payloadLen)
			break
		}

		var crcBuf [4]byte
		if _, err := io.ReadFull(reader, crcBuf[:]); err != nil {
			return fmt.Errorf("replay: failed to read CRC: %w", err)
		}
		savedCRC := binary.LittleEndian.Uint32(crcBuf[:])

		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return fmt.Errorf("replay: failed to read payload: %w", err)
		}

		if savedCRC != 0 && crc32.ChecksumIEEE(payload) != savedCRC {
			slog.Warn("replay: CRC mismatch, stopping replay")
			break
		}

		var pbEntry pb.LogEntry
		if err := proto.Unmarshal(payload, &pbEntry); err != nil {
			slog.Warn("replay: failed to unmarshal entry", "error", err)
			break
		}

		entry := protoToLogEntry(&pbEntry)
		if entry.LSN > maxLSN {
			maxLSN = entry.LSN
		}
		if err := fn(entry); err != nil {
			return err
		}
	}

	if maxLSN >= w.nextLSN {
		w.nextLSN = maxLSN + 1
	}

	if _, err := w.file.Seek(0, 2); err != nil {
		return err
	}
	return nil
}

func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}

	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			return err
		}
	}
	if err := w.file.Close(); err != nil {
		return err
	}

	f, err := os.OpenFile(w.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	bw := bufio.NewWriterSize(f, 64*1024)
	w.file = f
	w.writer = bw
	return nil
}

func (w *WAL) Path() string {
	return w.path
}

func logEntryToProto(e LogEntry) *pb.LogEntry {
	pbEntry := &pb.LogEntry{
		Lsn: e.LSN,
		Op:  string(e.Op),
		Id:  e.ID,
		Ids: e.IDs,
	}
	if e.Doc != nil {
		pbEntry.Doc = docToProto(e.Doc)
	}
	return pbEntry
}

func protoToLogEntry(pbEntry *pb.LogEntry) LogEntry {
	e := LogEntry{
		LSN:   pbEntry.Lsn,
		Op:    OpType(pbEntry.Op),
		ID:    pbEntry.Id,
		IDs:   pbEntry.Ids,
		CRC32: pbEntry.Crc32,
	}
	if pbEntry.Doc != nil {
		e.Doc = protoToDoc(pbEntry.Doc)
	}
	return e
}

func docToProto(d *doc.Doc) *pb.Doc {
	pbDoc := &pb.Doc{
		Id:    d.ID,
		Score: d.Score,
		DocId: d.DocID,
	}
	pbDoc.Fields = make(map[string]*pb.Value)
	d.ForEachField(func(name string, fv doc.Value) {
		pbDoc.Fields[name] = valueToProto(fv)
	})
	pbDoc.Vectors = make(map[string]*pb.VectorValue)
	d.ForEachVector(func(name string, vv doc.VectorValue) {
		pbDoc.Vectors[name] = vectorValueToProto(vv)
	})
	pbDoc.SparseVectors = make(map[string]*pb.SparseVectorValue)
	d.ForEachSparseVector(func(name string, sv doc.SparseVectorValue) {
		pbDoc.SparseVectors[name] = sparseVectorValueToProto(sv)
	})
	return pbDoc
}

func protoToDoc(pbDoc *pb.Doc) *doc.Doc {
	d := doc.NewDoc(pbDoc.Id)
	d.Score = pbDoc.Score
	d.DocID = pbDoc.DocId
	for k, pv := range pbDoc.Fields {
		d.SetField(k, protoToValue(pv))
	}
	for k, pv := range pbDoc.Vectors {
		d.SetVector(k, protoToVectorValue(pv))
	}
	for k, pv := range pbDoc.SparseVectors {
		d.SetSparseVector(k, protoToSparseVectorValue(pv))
	}
	return d
}

func valueToProto(v doc.Value) *pb.Value {
	return &pb.Value{
		Null:      v.Null,
		Type:      uint32(v.Type),
		BoolVal:   v.BoolVal,
		Int32Val:  v.Int32Val,
		Uint32Val: v.Uint32Val,
		Int64Val:  v.Int64Val,
		Uint64Val: v.Uint64Val,
		FloatVal:  v.FloatVal,
		DoubleVal: v.DoubleVal,
		StringVal: v.StringVal,
		BinaryVal: v.BinaryVal,
	}
}

func protoToValue(pv *pb.Value) doc.Value {
	return doc.Value{
		Null:      pv.Null,
		Type:      types.DataType(pv.Type),
		BoolVal:   pv.BoolVal,
		Int32Val:  pv.Int32Val,
		Uint32Val: pv.Uint32Val,
		Int64Val:  pv.Int64Val,
		Uint64Val: pv.Uint64Val,
		FloatVal:  pv.FloatVal,
		DoubleVal: pv.DoubleVal,
		StringVal: pv.StringVal,
		BinaryVal: pv.BinaryVal,
	}
}

func vectorValueToProto(v doc.VectorValue) *pb.VectorValue {
	pb := &pb.VectorValue{}
	if v.Float32s != nil {
		pb.Float32S = v.Float32s
	}
	if v.Float64s != nil {
		pb.Float64S = v.Float64s
	}
	if v.Int8s != nil {
		pb.Int8S = make([]int32, len(v.Int8s))
		for i, x := range v.Int8s {
			pb.Int8S[i] = int32(x)
		}
	}
	if v.Int16s != nil {
		pb.Int16S = make([]int32, len(v.Int16s))
		for i, x := range v.Int16s {
			pb.Int16S[i] = int32(x)
		}
	}
	if v.Int32s != nil {
		pb.Int32S = v.Int32s
	}
	if v.Int4s != nil {
		unpacked := v.Int4sUnpacked()
		pb.Int4S = make([]int32, len(unpacked))
		for i, x := range unpacked {
			pb.Int4S[i] = int32(x)
		}
	}
	if v.Float16s != nil {
		pb.Float16S = make([]uint32, len(v.Float16s))
		for i, x := range v.Float16s {
			pb.Float16S[i] = uint32(x)
		}
	}
	return pb
}

func protoToVectorValue(pv *pb.VectorValue) doc.VectorValue {
	v := doc.VectorValue{}
	if len(pv.Float32S) > 0 {
		v.Float32s = pv.Float32S
	}
	if len(pv.Float64S) > 0 {
		v.Float64s = pv.Float64S
	}
	if len(pv.Int8S) > 0 {
		v.Int8s = make([]int8, len(pv.Int8S))
		for i, x := range pv.Int8S {
			v.Int8s[i] = int8(x)
		}
	}
	if len(pv.Int16S) > 0 {
		v.Int16s = make([]int16, len(pv.Int16S))
		for i, x := range pv.Int16S {
			v.Int16s[i] = int16(x)
		}
	}
	if len(pv.Int32S) > 0 {
		v.Int32s = pv.Int32S
	}
	if len(pv.Int4S) > 0 {
		vals := make([]int8, len(pv.Int4S))
		for i, x := range pv.Int4S {
			vals[i] = int8(x)
		}
		v.SetInt4s(vals)
	}
	if len(pv.Float16S) > 0 {
		v.Float16s = make([]uint16, len(pv.Float16S))
		for i, x := range pv.Float16S {
			v.Float16s[i] = uint16(x)
		}
	}
	return v
}

func sparseVectorValueToProto(sv doc.SparseVectorValue) *pb.SparseVectorValue {
	return &pb.SparseVectorValue{
		Indices: sv.Indices,
		Values:  sv.Values,
	}
}

func protoToSparseVectorValue(pv *pb.SparseVectorValue) doc.SparseVectorValue {
	return doc.SparseVectorValue{
		Indices: pv.Indices,
		Values:  pv.Values,
	}
}
