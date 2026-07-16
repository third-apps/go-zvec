package wal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/third-apps/go-zvec/doc"
)

type OpType string

const (
	OpInsert OpType = "INSERT"
	OpUpsert OpType = "UPSERT"
	OpUpdate OpType = "UPDATE"
	OpDelete OpType = "DELETE"
)

type LogEntry struct {
	Op  OpType   `json:"op"`
	ID  string   `json:"id,omitempty"`
	Doc *doc.Doc `json:"doc,omitempty"`
	IDs []string `json:"ids,omitempty"`
}

type WAL struct {
	mu      sync.Mutex
	file    *os.File
	path    string
	encoder *json.Encoder
}

func Open(path string) (*WAL, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &WAL{
		file: f,
		path: path,

		encoder: json.NewEncoder(f),
	}, nil
}

func (w *WAL) AppendInsert(id string, d *doc.Doc) error {
	return w.appendEntry(LogEntry{Op: OpInsert, ID: id, Doc: d})
}

func (w *WAL) AppendUpsert(id string, d *doc.Doc) error {
	return w.appendEntry(LogEntry{Op: OpUpsert, ID: id, Doc: d})
}

func (w *WAL) AppendUpdate(id string, d *doc.Doc) error {
	return w.appendEntry(LogEntry{Op: OpUpdate, ID: id, Doc: d})
}

func (w *WAL) AppendDeletes(ids []string) error {
	return w.appendEntry(LogEntry{Op: OpDelete, IDs: ids})
}

func (w *WAL) appendEntry(entry LogEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.encoder.Encode(entry)
}

func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.file.Sync(); err != nil {
		return err
	}
	return w.file.Close()
}
