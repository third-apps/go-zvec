package collection

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/third-apps/go-zvec/doc"
	"github.com/third-apps/go-zvec/fts"
	"github.com/third-apps/go-zvec/index"
	"github.com/third-apps/go-zvec/index/diskann"
	"github.com/third-apps/go-zvec/index/flat"
	"github.com/third-apps/go-zvec/index/hnsw"
	"github.com/third-apps/go-zvec/index/hnsw_rabitq"
	"github.com/third-apps/go-zvec/index/invert"
	"github.com/third-apps/go-zvec/index/ivf"
	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/index/vamana"
	"github.com/third-apps/go-zvec/metadata"
	"github.com/third-apps/go-zvec/query"
	"github.com/third-apps/go-zvec/reranker"
	"github.com/third-apps/go-zvec/schema"
	"github.com/third-apps/go-zvec/segment"
	"github.com/third-apps/go-zvec/status"
	"github.com/third-apps/go-zvec/types"
	"github.com/third-apps/go-zvec/wal"
)

type Options struct {
	ReadOnly      bool
	EnableMMAP    bool
	MaxBufferSize int64
	SegmentSize   int
}

type Collection struct {
	mu            sync.RWMutex
	indexMu       sync.RWMutex
	ftsMu         sync.RWMutex
	path          string
	schema        *schema.CollectionSchema
	options       Options
	docs          *shardedDocs
	docIndex      *shardedDocIndex
	docIDToPK     *shardedDocIDToPK
	nextDocID     uint64
	indexes       map[string]index.Index
	ftsIndexes    map[string]FTSIndexer
	invertIndexes map[string]InvertIndexer
	metaIndex     MetaIndexer
	wal           WALWriter
	segManager    SegmentManager
}

func CreateAndOpen(path string, s *schema.CollectionSchema, opts *Options) (*Collection, error) {
	if path == "" {
		return nil, errors.New("collection path cannot be empty")
	}
	if s == nil {
		return nil, errors.New("schema cannot be nil")
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	actualOpts := Options{}
	if opts != nil {
		actualOpts = *opts
	}

	c := &Collection{
		path:          path,
		schema:        s,
		options:       actualOpts,
		docs:          newShardedDocs(),
		docIndex:      newShardedDocIndex(),
		docIDToPK:     newShardedDocIDToPK(),
		indexes:       make(map[string]index.Index),
		ftsIndexes:    make(map[string]FTSIndexer),
		invertIndexes: make(map[string]InvertIndexer),
		metaIndex:     metadata.NewMetadataIndex(),
	}
	if actualOpts.SegmentSize > 0 {
		c.segManager = segment.NewManager(segment.Option{MaxSegmentSize: actualOpts.SegmentSize})
	}

	walF, err := wal.Open(filepath.Join(path, "wal.log"))
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL: %w", err)
	}
	c.wal = walF

	for _, field := range s.VectorFields() {
		idx, err := createIndex(field)
		if err != nil {
			c.Close()
			return nil, err
		}
		c.indexes[field.Name] = idx
	}

	for _, field := range s.FTSFields() {
		tok := createTokenizer(field)
		c.ftsIndexes[field.Name] = fts.NewFTSIndex(tok)
	}

	for _, field := range s.InvertFields() {
		c.invertIndexes[field.Name] = invert.NewInvertIndex()
	}

	if err := c.saveSchema(); err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to save schema: %w", err)
	}

	return c, nil
}

func Open(path string, opts *Options) (*Collection, error) {
	s, err := loadSchema(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}

	actualOpts := Options{}
	if opts != nil {
		actualOpts = *opts
	}

	c := &Collection{
		path:          path,
		schema:        s,
		options:       actualOpts,
		docs:          newShardedDocs(),
		docIndex:      newShardedDocIndex(),
		docIDToPK:     newShardedDocIDToPK(),
		indexes:       make(map[string]index.Index),
		ftsIndexes:    make(map[string]FTSIndexer),
		invertIndexes: make(map[string]InvertIndexer),
		metaIndex:     metadata.NewMetadataIndex(),
	}
	if actualOpts.SegmentSize > 0 {
		c.segManager = segment.NewManager(segment.Option{MaxSegmentSize: actualOpts.SegmentSize})
	}

	for _, field := range s.VectorFields() {
		idx, err := createIndex(field)
		if err != nil {
			c.Close()
			return nil, err
		}
		c.indexes[field.Name] = idx
	}

	for _, field := range s.FTSFields() {
		tok := createTokenizer(field)
		c.ftsIndexes[field.Name] = fts.NewFTSIndex(tok)
	}

	for _, field := range s.InvertFields() {
		c.invertIndexes[field.Name] = invert.NewInvertIndex()
	}

	var walF *wal.WAL
	if actualOpts.ReadOnly {
		walF, err = wal.OpenReadOnly(filepath.Join(path, "wal.log"))
	} else {
		walF, err = wal.Open(filepath.Join(path, "wal.log"))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL: %w", err)
	}
	c.wal = walF

	if err := c.replayWAL(); err != nil {
		return nil, fmt.Errorf("failed to replay WAL: %w", err)
	}

	return c, nil
}

func loadSchema(path string) (*schema.CollectionSchema, error) {
	data, err := os.ReadFile(filepath.Join(path, "schema.json"))
	if err != nil {
		return nil, err
	}
	var s schema.CollectionSchema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Collection) saveSchema() error {
	data, err := json.Marshal(c.schema)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.path, "schema.json"), data, 0644)
}

func (c *Collection) replayWAL() error {
	if c.wal == nil {
		return nil
	}
	return c.wal.Replay(func(entry wal.LogEntry) error {
		c.replayEntry(entry)
		return nil
	})
}

func (c *Collection) replayEntry(entry wal.LogEntry) {
	switch entry.Op {
	case wal.OpInsert:
		if entry.Doc == nil {
			return
		}
		if c.pkExists(entry.Doc.ID) {
			return
		}
		if c.segManager != nil {
			c.segManager.Insert(entry.Doc)
		} else {
			c.docs.Append(entry.Doc.DocID, entry.Doc)
			c.docIndex.Set(entry.Doc.ID, int(entry.Doc.DocID))
			c.docIDToPK.Set(entry.Doc.DocID, entry.Doc.ID)
		}
		if entry.Doc.DocID >= c.nextDocID {
			c.nextDocID = entry.Doc.DocID + 1
		}
		c.addAllIndexesConcurrent(entry.Doc)
	case wal.OpUpsert:
		if entry.Doc == nil {
			return
		}
		if c.segManager != nil {
			if existing := c.segManager.GetDoc(entry.Doc.ID); existing != nil {
				entry.Doc.DocID = existing.DocID
				c.deleteAllIndexesConcurrent(existing.ID)
				c.segManager.Upsert(entry.Doc)
				c.addAllIndexesConcurrent(entry.Doc)
			} else {
				c.segManager.Insert(entry.Doc)
				c.addAllIndexesConcurrent(entry.Doc)
			}
		} else {
			if existingIdx, exists := c.docIndex.Get(entry.Doc.ID); exists {
				existing, ok := c.docs.Get(uint64(existingIdx))
				if !ok {
					return
				}
				c.deleteAllIndexesConcurrent(existing.ID)
				entry.Doc.DocID = existing.DocID
				c.docs.Set(uint64(existingIdx), entry.Doc)
				c.addAllIndexesConcurrent(entry.Doc)
			} else {
				c.docs.Append(entry.Doc.DocID, entry.Doc)
				c.docIndex.Set(entry.Doc.ID, int(entry.Doc.DocID))
				c.docIDToPK.Set(entry.Doc.DocID, entry.Doc.ID)
				if entry.Doc.DocID >= c.nextDocID {
					c.nextDocID = entry.Doc.DocID + 1
				}
				c.addAllIndexesConcurrent(entry.Doc)
			}
		}
	case wal.OpUpdate:
		if entry.Doc == nil {
			return
		}
		if c.segManager != nil {
			if existing := c.segManager.GetDoc(entry.Doc.ID); existing != nil {
				entry.Doc.DocID = existing.DocID
				c.deleteAllIndexesConcurrent(existing.ID)
				c.segManager.Upsert(entry.Doc)
				c.addAllIndexesConcurrent(entry.Doc)
			}
		} else {
			existingIdx, exists := c.docIndex.Get(entry.Doc.ID)
			if !exists {
				return
			}
			existing, ok := c.docs.Get(uint64(existingIdx))
			if !ok {
				return
			}
			c.deleteAllIndexesConcurrent(existing.ID)
			entry.Doc.DocID = existing.DocID
			c.docs.Set(uint64(existingIdx), entry.Doc)
			c.addAllIndexesConcurrent(entry.Doc)
		}
	case wal.OpDelete:
		for _, id := range entry.IDs {
			if !c.pkExists(id) {
				continue
			}
			c.deleteAllIndexesConcurrent(id)
			if c.segManager != nil {
				c.segManager.Delete(id)
			} else {
				if idx, exists := c.docIndex.Get(id); exists {
					removed, ok := c.docs.Get(uint64(idx))
					if !ok {
						continue
					}
					c.docIDToPK.Delete(removed.DocID)
					c.docs.Delete(uint64(idx))
					c.docIndex.Delete(id)
				}
			}
		}
	}
}

func (c *Collection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.indexMu.Lock()
	defer c.indexMu.Unlock()

	c.ftsMu.Lock()
	defer c.ftsMu.Unlock()

	for _, idx := range c.indexes {
		idx.Close()
	}
	c.indexes = nil
	c.docs = nil
	c.docIndex = nil
	c.docIDToPK = nil
	c.ftsIndexes = nil
	c.invertIndexes = nil
	c.metaIndex = nil
	if c.segManager != nil {
		c.segManager.Close()
		c.segManager = nil
	}
	if c.wal != nil {
		return c.wal.Close()
	}
	return nil
}

func (c *Collection) Compact() error {
	c.mu.Lock()
	var snapshot []*doc.Doc
	c.docs.ForEach(func(d *doc.Doc) bool {
		snapshot = append(snapshot, d)
		return true
	})
	nextDocID := c.nextDocID
	c.mu.Unlock()

	newDocIndex := newShardedDocIndex()
	newDocIDToPK := newShardedDocIDToPK()
	newSD := newShardedDocs()
	for _, d := range snapshot {
		newDocIndex.Set(d.ID, int(d.DocID))
		newDocIDToPK.Set(d.DocID, d.ID)
		newSD.Append(d.DocID, d)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.nextDocID != nextDocID {
		// State changed during rebuild, redo with fresh snapshot
		var curDocs []*doc.Doc
		c.docs.ForEach(func(d *doc.Doc) bool {
			curDocs = append(curDocs, d)
			return true
		})
		newDocIndex = newShardedDocIndex()
		newDocIDToPK = newShardedDocIDToPK()
		newSD = newShardedDocs()
		for _, d := range curDocs {
			newDocIndex.Set(d.ID, int(d.DocID))
			newDocIDToPK.Set(d.DocID, d.ID)
			newSD.Append(d.DocID, d)
		}
	}

	c.docs = newSD
	c.docIndex = newDocIndex
	c.docIDToPK = newDocIDToPK
	return nil
}

func (c *Collection) Path() string {
	return c.path
}

func (c *Collection) Schema() *schema.CollectionSchema {
	return c.schema
}

func (c *Collection) Options() Options {
	return c.options
}

func (c *Collection) Destroy() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, idx := range c.indexes {
		idx.Close()
	}
	c.docs = nil
	c.docIndex = nil
	c.indexes = nil
	c.ftsIndexes = nil
	c.invertIndexes = nil
	c.docIDToPK = nil
	if c.segManager != nil {
		c.segManager.Close()
		c.segManager = nil
	}
	if c.wal != nil {
		if err := c.wal.Close(); err != nil {
			return err
		}
		c.wal = nil
	}
	if c.path != "" {
		return os.RemoveAll(c.path)
	}
	return nil
}

func (c *Collection) Flush() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.wal != nil {
		return c.wal.Sync()
	}
	return nil
}

func (c *Collection) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.indexMu.RLock()
	defer c.indexMu.RUnlock()
	return c.saveLocked()
}

func LoadCollection(path string, opts *Options) (*Collection, error) {
	if path == "" {
		return nil, errors.New("collection path cannot be empty")
	}

	schemaPath := filepath.Join(path, "schema.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}

	var s schema.CollectionSchema
	if err := json.Unmarshal(schemaData, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}

	c, err := CreateAndOpen(path, &s, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	for _, field := range s.VectorFields() {
		if field.IndexParam == nil {
			continue
		}
		idxPath := filepath.Join(path, field.Name+".idx")
		if _, err := os.Stat(idxPath); os.IsNotExist(err) {
			continue
		}

		idx, ok := c.indexes[field.Name]
		if !ok {
			continue
		}

		f, err := os.Open(idxPath)
		if err != nil {
			c.Close()
			return nil, fmt.Errorf("failed to open index file for %s: %w", field.Name, err)
		}
		if err := idx.Load(f); err != nil {
			f.Close()
			c.Close()
			return nil, fmt.Errorf("failed to load index %s: %w", field.Name, err)
		}
		f.Close()
	}

	docsPath := filepath.Join(path, "docs.json")
	if docsData, err := os.ReadFile(docsPath); err == nil {
		var docs []*doc.Doc
		if err := json.Unmarshal(docsData, &docs); err == nil {
			c.mu.Lock()
			c.docs = newShardedDocs()
			c.docIndex = newShardedDocIndex()
			c.docIDToPK = newShardedDocIDToPK()
			for _, d := range docs {
				if d != nil {
					c.docs.Append(d.DocID, d)
					c.docIndex.Set(d.ID, int(d.DocID))
					c.docIDToPK.Set(d.DocID, d.ID)
				}
			}
			if len(docs) > 0 {
				maxDocID := uint64(0)
				c.docs.ForEach(func(d *doc.Doc) bool {
					if d.DocID > maxDocID {
						maxDocID = d.DocID
					}
					return true
				})
				c.nextDocID = maxDocID + 1
			}
			c.mu.Unlock()
		}
	}

	metaPath := filepath.Join(path, "meta.json")
	if metaData, err := os.ReadFile(metaPath); err == nil {
		var meta map[string]uint64
		if err := json.Unmarshal(metaData, &meta); err == nil {
			if nid, ok := meta["nextDocID"]; ok && nid > c.nextDocID {
				c.mu.Lock()
				c.nextDocID = nid
				c.mu.Unlock()
			}
		}
	}

	return c, nil
}

func (c *Collection) Snapshot() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.indexMu.RLock()
	defer c.indexMu.RUnlock()

	if err := c.saveLocked(); err != nil {
		return err
	}

	if c.wal != nil {
		if err := c.wal.Truncate(); err != nil {
			return fmt.Errorf("failed to truncate WAL after snapshot: %w", err)
		}
	}

	return nil
}

func (c *Collection) Recover() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.wal == nil {
		return nil
	}

	return c.wal.Replay(func(entry wal.LogEntry) error {
		switch entry.Op {
		case wal.OpInsert:
			if entry.Doc != nil {
				c.insertDocLocked(entry.Doc)
			}
		case wal.OpUpsert:
			if entry.Doc != nil {
				c.upsertDocLocked(entry.Doc)
			}
		case wal.OpUpdate:
			if entry.Doc != nil {
				c.upsertDocLocked(entry.Doc)
			}
		case wal.OpDelete:
			if entry.IDs != nil {
				for _, id := range entry.IDs {
					c.deleteAllIndexesConcurrent(id)
					c.removeDocLocked(id)
				}
			} else if entry.ID != "" {
				c.deleteAllIndexesConcurrent(entry.ID)
				c.removeDocLocked(entry.ID)
			}
		}
		return nil
	})
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (c *Collection) saveLocked() error {
	if c.path == "" {
		return errors.New("collection path is empty, cannot save")
	}

	if err := os.MkdirAll(c.path, 0755); err != nil {
		return fmt.Errorf("failed to create collection directory: %w", err)
	}

	schemaPath := filepath.Join(c.path, "schema.json")
	schemaData, err := json.Marshal(c.schema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}
	if err := writeFileAtomic(schemaPath, schemaData, 0644); err != nil {
		return fmt.Errorf("failed to write schema: %w", err)
	}

	metaPath := filepath.Join(c.path, "meta.json")
	metaData, err := json.Marshal(map[string]uint64{"nextDocID": c.nextDocID})
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}
	if err := writeFileAtomic(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write meta: %w", err)
	}

	for fieldName, idx := range c.indexes {
		idxPath := filepath.Join(c.path, fieldName+".idx")
		tmpIdxPath := idxPath + ".tmp"
		f, err := os.Create(tmpIdxPath)
		if err != nil {
			return fmt.Errorf("failed to create index file for %s: %w", fieldName, err)
		}
		if err := idx.Save(f); err != nil {
			f.Close()
			return fmt.Errorf("failed to save index %s: %w", fieldName, err)
		}
		f.Close()
		if err := os.Rename(tmpIdxPath, idxPath); err != nil {
			return fmt.Errorf("failed to rename index file for %s: %w", fieldName, err)
		}
	}

	docsPath := filepath.Join(c.path, "docs.json")
	var allDocs []*doc.Doc
	if c.segManager != nil {
		allDocs = c.segManager.AllDocs()
	} else {
		allDocs = c.docs.AllDocs()
	}
	docsData, err := json.Marshal(allDocs)
	if err != nil {
		return fmt.Errorf("failed to marshal docs: %w", err)
	}
	if err := writeFileAtomic(docsPath, docsData, 0644); err != nil {
		return fmt.Errorf("failed to write docs: %w", err)
	}

	if c.wal != nil {
		if err := c.wal.Sync(); err != nil {
			return fmt.Errorf("failed to sync WAL: %w", err)
		}
	}

	return nil
}

func (c *Collection) insertDocLocked(d *doc.Doc) {
	if c.segManager != nil {
		c.segManager.Insert(d)
	} else {
		c.docs.Append(d.DocID, d)
		c.docIndex.Set(d.ID, int(d.DocID))
		c.docIDToPK.Set(d.DocID, d.ID)
	}
	c.addAllIndexesConcurrent(d)
}

func (c *Collection) upsertDocLocked(d *doc.Doc) {
	if c.segManager != nil {
		c.segManager.Upsert(d)
	} else if existingIdx, exists := c.docIndex.Get(d.ID); exists {
		existing, ok := c.docs.Get(uint64(existingIdx))
		if !ok {
			return
		}
		c.deleteAllIndexesConcurrent(existing.ID)
		d.DocID = existing.DocID
		c.docs.Set(uint64(existingIdx), d)
		c.addAllIndexesConcurrent(d)
	} else {
		c.docs.Append(d.DocID, d)
		c.docIndex.Set(d.ID, int(d.DocID))
		c.docIDToPK.Set(d.DocID, d.ID)
		c.addAllIndexesConcurrent(d)
	}
}

func (c *Collection) removeDocLocked(pk string) {
	if c.segManager != nil {
		c.segManager.Delete(pk)
		return
	}
	idx, exists := c.docIndex.Get(pk)
	if !exists {
		return
	}
	removed, ok := c.docs.Get(uint64(idx))
	if !ok {
		return
	}
	c.docIDToPK.Delete(removed.DocID)
	c.docs.Delete(uint64(idx))
	c.docIndex.Delete(pk)
}

func (c *Collection) Insert(docs []*doc.Doc) status.Status {
	for _, d := range docs {
		if err := d.Validate(c.schema); err != nil {
			return status.NewInvalidArgument(err.Error())
		}
	}

	c.mu.Lock()
	for _, d := range docs {
		if c.pkExists(d.ID) {
			c.mu.Unlock()
			return status.NewInvalidArgument(fmt.Sprintf("doc '%s' already exists, use Upsert instead", d.ID))
		}
	}

	for _, d := range docs {
		d.DocID = c.nextDocID
		c.nextDocID++
	}

	if c.wal != nil {
		if err := c.wal.AppendInserts(docs); err != nil {
			c.nextDocID -= uint64(len(docs))
			c.mu.Unlock()
			return status.NewInternalError(fmt.Sprintf("WAL write failed: %v", err))
		}
	}

	for _, d := range docs {
		if c.segManager != nil {
			c.segManager.Insert(d)
		} else {
			c.docs.Append(d.DocID, d)
			c.docIndex.Set(d.ID, int(d.DocID))
			c.docIDToPK.Set(d.DocID, d.ID)
		}
	}
	c.mu.Unlock()

	c.addAllIndexesConcurrentBatch(docs)

	return status.OKStatus()
}

func (c *Collection) Upsert(docs []*doc.Doc) status.Status {
	for _, d := range docs {
		if err := d.Validate(c.schema); err != nil {
			return status.NewInvalidArgument(err.Error())
		}
	}

	c.mu.Lock()
	var existingPKs []string
	startDocID := c.nextDocID
	for _, d := range docs {
		if c.segManager != nil {
			existing := c.segManager.GetDoc(d.ID)
			if existing != nil {
				existingPKs = append(existingPKs, existing.ID)
				d.DocID = existing.DocID
			} else {
				d.DocID = c.nextDocID
				c.nextDocID++
			}
		} else if existingIdx, exists := c.docIndex.Get(d.ID); exists {
			existing, ok := c.docs.Get(uint64(existingIdx))
			if !ok {
				continue
			}
			existingPKs = append(existingPKs, existing.ID)
			d.DocID = existing.DocID
		} else {
			d.DocID = c.nextDocID
			c.nextDocID++
		}
	}

	if c.wal != nil {
		if err := c.wal.AppendUpserts(docs); err != nil {
			c.nextDocID = startDocID
			c.mu.Unlock()
			return status.NewInternalError(fmt.Sprintf("WAL write failed: %v", err))
		}
	}

	for _, d := range docs {
		if c.segManager != nil {
			c.segManager.Upsert(d)
		} else if existingIdx, exists := c.docIndex.Get(d.ID); exists {
			c.docs.Set(uint64(existingIdx), d)
		} else {
			c.docs.Append(d.DocID, d)
			c.docIndex.Set(d.ID, int(d.DocID))
			c.docIDToPK.Set(d.DocID, d.ID)
		}
	}
	c.mu.Unlock()

	c.deleteAllIndexesConcurrentBatch(existingPKs)
	c.addAllIndexesConcurrentBatch(docs)

	return status.OKStatus()
}

func (c *Collection) Update(docs []*doc.Doc) status.Status {
	for _, d := range docs {
		if err := d.Validate(c.schema); err != nil {
			return status.NewInvalidArgument(err.Error())
		}
	}

	c.mu.Lock()
	var existingPKs []string
	for _, d := range docs {
		var existing *doc.Doc
		if c.segManager != nil {
			existing = c.segManager.GetDoc(d.ID)
		} else {
			if existingIdx, exists := c.docIndex.Get(d.ID); exists {
				existing, _ = c.docs.Get(uint64(existingIdx))
			}
		}
		if existing == nil {
			c.mu.Unlock()
			return status.NewNotFound(fmt.Sprintf("doc '%s' not found", d.ID))
		}
		d.DocID = existing.DocID
		existingPKs = append(existingPKs, existing.ID)
	}
	if c.wal != nil {
		if err := c.wal.AppendUpdates(docs); err != nil {
			c.mu.Unlock()
			return status.NewInternalError(fmt.Sprintf("WAL write failed: %v", err))
		}
	}
	for _, d := range docs {
		if c.segManager != nil {
			c.segManager.Upsert(d)
		} else {
			idx, exists := c.docIndex.Get(d.ID)
			if !exists {
				continue
			}
			c.docs.Set(uint64(idx), d)
		}
	}
	c.mu.Unlock()

	c.deleteAllIndexesConcurrentBatch(existingPKs)
	c.addAllIndexesConcurrentBatch(docs)

	return status.OKStatus()
}

func (c *Collection) Delete(ids []string) status.Status {
	c.mu.Lock()
	if c.wal != nil {
		if err := c.wal.AppendDeletes(ids); err != nil {
			c.mu.Unlock()
			return status.NewInternalError(fmt.Sprintf("WAL write failed: %v", err))
		}
	}
	for _, id := range ids {
		c.deleteAllIndexesConcurrent(id)
		if c.segManager != nil {
			c.segManager.Delete(id)
		} else if idx, exists := c.docIndex.Get(id); exists {
			removed, ok := c.docs.Get(uint64(idx))
			if !ok {
				continue
			}
			c.docIDToPK.Delete(removed.DocID)
			c.docs.Delete(uint64(idx))
			c.docIndex.Delete(id)
		}
	}
	c.mu.Unlock()
	return status.OKStatus()
}

func (c *Collection) deleteDoc(id string) {
	c.deleteAllIndexesConcurrent(id)
	if c.segManager != nil {
		c.segManager.Delete(id)
	} else if idx, exists := c.docIndex.Get(id); exists {
		removed, ok := c.docs.Get(uint64(idx))
		if !ok {
			return
		}
		c.docIDToPK.Delete(removed.DocID)
		c.docs.Delete(uint64(idx))
		c.docIndex.Delete(id)
	}
}

func (c *Collection) DeleteByFilter(filter string) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	var toDelete []string
	fn := compileFilter(filter)
	if c.segManager != nil {
		for _, d := range c.segManager.AllDocs() {
			if fn(d) {
				toDelete = append(toDelete, d.ID)
			}
		}
	} else {
		c.docs.ForEach(func(d *doc.Doc) bool {
			if fn(d) {
				toDelete = append(toDelete, d.ID)
			}
			return true
		})
	}

	if c.wal != nil && len(toDelete) > 0 {
		if err := c.wal.AppendDeletes(toDelete); err != nil {
			return status.NewInternalError(fmt.Sprintf("WAL write failed: %v", err))
		}
	}

	for _, id := range toDelete {
		c.deleteDoc(id)
	}

	return status.OKStatus()
}

func (c *Collection) getDocByPK(pk string) *doc.Doc {
	if c.segManager != nil {
		return c.segManager.GetDoc(pk)
	}
	if docID, exists := c.docIndex.Get(pk); exists {
		d, ok := c.docs.Get(uint64(docID))
		if !ok {
			return nil
		}
		return d
	}
	return nil
}

func (c *Collection) resolveDocIDToPK(docID uint64) string {
	if c.segManager != nil {
		return c.segManager.ResolveDocIDToPK(docID)
	}
	v, ok := c.docIDToPK.Get(docID)
	if !ok {
		return ""
	}
	return v
}

func (c *Collection) resolveMetaFilter(mf *query.MetadataFilter) []uint64 {
	var result []uint64
	for i, cond := range mf.Conditions {
		var docIDs []uint64
		switch cond.Op {
		case query.MetadataOpEq:
			switch cond.ValueType {
			case types.DataTypeString:
				docIDs = c.metaIndex.MatchString(cond.FieldName, cond.StringVal)
			case types.DataTypeInt64, types.DataTypeInt32:
				docIDs = c.metaIndex.MatchInt64(cond.FieldName, cond.Int64Val)
			case types.DataTypeBool:
				docIDs = c.metaIndex.MatchBool(cond.FieldName, cond.BoolVal)
			default:
				if cond.StringVal != "" {
					docIDs = c.metaIndex.MatchString(cond.FieldName, cond.StringVal)
				} else if cond.Int64Val != 0 {
					docIDs = c.metaIndex.MatchInt64(cond.FieldName, cond.Int64Val)
				} else {
					docIDs = c.metaIndex.MatchBool(cond.FieldName, cond.BoolVal)
				}
			}
		case query.MetadataOpIn:
			if len(cond.StringVals) > 0 {
				docIDs = c.metaIndex.MatchStrings(cond.FieldName, cond.StringVals)
			} else if len(cond.Int64Vals) > 0 {
				seen := make(map[uint64]struct{})
				for _, v := range cond.Int64Vals {
					for _, id := range c.metaIndex.MatchInt64(cond.FieldName, v) {
						seen[id] = struct{}{}
					}
				}
				docIDs = make([]uint64, 0, len(seen))
				for id := range seen {
					docIDs = append(docIDs, id)
				}
			}
		case query.MetadataOpNe:
			docIDs = c.metaIndex.MatchInt64Ne(cond.FieldName, cond.Int64Val)
		case query.MetadataOpGt:
			docIDs = c.metaIndex.MatchInt64Gt(cond.FieldName, cond.Int64Val)
		case query.MetadataOpLt:
			docIDs = c.metaIndex.MatchInt64Lt(cond.FieldName, cond.Int64Val)
		case query.MetadataOpGte:
			docIDs = c.metaIndex.MatchInt64Gte(cond.FieldName, cond.Int64Val)
		case query.MetadataOpLte:
			docIDs = c.metaIndex.MatchInt64Lte(cond.FieldName, cond.Int64Val)
		case query.MetadataOpExists:
			docIDs = c.metaIndex.MatchExists(cond.FieldName)
		default:
			continue
		}

		if i == 0 {
			result = docIDs
		} else {
			result = intersectUint64(result, docIDs)
		}
	}
	return result
}

func intersectUint64(a, b []uint64) []uint64 {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	setB := make(map[uint64]struct{}, len(b))
	for _, id := range b {
		setB[id] = struct{}{}
	}
	var result []uint64
	for _, id := range a {
		if _, ok := setB[id]; ok {
			result = append(result, id)
		}
	}
	return result
}

func (c *Collection) addToIndexes(d *doc.Doc) {
	var wg sync.WaitGroup
	for _, field := range c.schema.VectorFields() {
		if idx, ok := c.indexes[field.Name]; ok {
			v, _ := d.Vector(field.Name)
			if f32 := vectorToFloat32(v); f32 != nil {
				wg.Add(1)
				go func(idx index.Index, f32 []float32, pk string) {
					defer recoverDone(&wg, "addToIndexes", "pk", pk)
					idx.Add(f32, pk)
				}(idx, f32, d.ID)
			}
		}
	}
	wg.Wait()
}

func vectorToFloat32(v doc.VectorValue) []float32 {
	if v.Float32s != nil {
		return v.Float32s
	}
	if v.Float64s != nil {
		result := make([]float32, len(v.Float64s))
		for i, f := range v.Float64s {
			result[i] = float32(f)
		}
		return result
	}
	if v.Int8s != nil {
		result := make([]float32, len(v.Int8s))
		for i, x := range v.Int8s {
			result[i] = float32(x)
		}
		return result
	}
	if v.Int16s != nil {
		result := make([]float32, len(v.Int16s))
		for i, x := range v.Int16s {
			result[i] = float32(x)
		}
		return result
	}
	if v.Int32s != nil {
		result := make([]float32, len(v.Int32s))
		for i, x := range v.Int32s {
			result[i] = float32(x)
		}
		return result
	}
	if v.Int4s != nil {
		unpacked := v.Int4sUnpacked()
		result := make([]float32, len(unpacked))
		for i, x := range unpacked {
			result[i] = float32(x)
		}
		return result
	}
	if v.Float16s != nil {
		result := make([]float32, len(v.Float16s))
		for i, h := range v.Float16s {
			result[i] = float16toFloat32(h)
		}
		return result
	}
	return nil
}

func float16toFloat32(bits uint16) float32 {
	sign := uint32((bits >> 15) & 1)
	exp := uint32((bits >> 10) & 0x1F)
	mant := uint32(bits & 0x3FF)
	if exp == 0 {
		if mant == 0 {
			return float32fromBits(sign << 31)
		}
		for mant&0x400 == 0 {
			mant <<= 1
			exp--
		}
		exp++
		mant &= 0x3FF
	} else if exp == 0x1F {
		return float32fromBits((sign << 31) | (0xFF << 23) | (mant << 13))
	}
	exp += 127 - 15
	mant <<= 13
	return float32fromBits((sign << 31) | (exp << 23) | mant)
}

func float32fromBits(b uint32) float32 {
	return math.Float32frombits(b)
}

func (c *Collection) deleteFromIndexes(pk string) {
	var wg sync.WaitGroup
	for _, field := range c.schema.VectorFields() {
		if idx, ok := c.indexes[field.Name]; ok {
			wg.Add(1)
			go func(idx index.Index, pk string) {
				defer recoverDone(&wg, "deleteFromIndexes", "pk", pk)
				idx.Delete(pk)
			}(idx, pk)
		}
	}
	wg.Wait()
}

func recoverDone(wg *sync.WaitGroup, label string, fields ...any) {
	if r := recover(); r != nil {
		slog.Error("panic in "+label, append(fields, "error", r)...)
	}
	wg.Done()
}

func (c *Collection) addToFTSIndexes(d *doc.Doc) {
	var wg sync.WaitGroup
	for _, field := range c.schema.FTSFields() {
		if ftsIdx, ok := c.ftsIndexes[field.Name]; ok {
			if fv, ok := d.Field(field.Name); ok && !fv.Null {
				wg.Add(1)
				go func(ftsIdx FTSIndexer, docID uint64, text string) {
					defer recoverDone(&wg, "addToFTSIndexes", "docID", docID)
					ftsIdx.Index(docID, text)
				}(ftsIdx, d.DocID, fv.StringVal)
			}
		}
	}
	wg.Wait()
}

func (c *Collection) deleteFromFTSIndexes(pk string) {
	d := c.getDocByPK(pk)
	if d == nil {
		return
	}
	var wg sync.WaitGroup
	for _, field := range c.schema.FTSFields() {
		if ftsIdx, ok := c.ftsIndexes[field.Name]; ok {
			wg.Add(1)
			go func(ftsIdx FTSIndexer, docID uint64) {
				defer recoverDone(&wg, "deleteFromFTSIndexes", "docID", docID)
				ftsIdx.Delete(docID)
			}(ftsIdx, d.DocID)
		}
	}
	wg.Wait()
}

func (c *Collection) addToInvertIndexes(d *doc.Doc) {
	var wg sync.WaitGroup
	for _, field := range c.schema.InvertFields() {
		if invIdx, ok := c.invertIndexes[field.Name]; ok {
			if fv, ok := d.Field(field.Name); ok && !fv.Null && fv.StringVal != "" {
				wg.Add(1)
				go func(invIdx InvertIndexer, docID uint64, val string) {
					defer recoverDone(&wg, "addToInvertIndexes", "docID", docID)
					invIdx.Add(docID, val)
				}(invIdx, d.DocID, fv.StringVal)
			}
		}
	}
	wg.Wait()
}

func (c *Collection) deleteFromInvertIndexes(pk string) {
	d := c.getDocByPK(pk)
	if d == nil {
		return
	}
	var wg sync.WaitGroup
	for _, field := range c.schema.InvertFields() {
		if invIdx, ok := c.invertIndexes[field.Name]; ok {
			if fv, ok := d.Field(field.Name); ok && !fv.Null && fv.StringVal != "" {
				wg.Add(1)
				go func(invIdx InvertIndexer, docID uint64, val string) {
					defer recoverDone(&wg, "deleteFromInvertIndexes", "docID", docID)
					invIdx.Delete(docID, val)
				}(invIdx, d.DocID, fv.StringVal)
			}
		}
	}
	wg.Wait()
}

func (c *Collection) addToMetaIndex(d *doc.Doc) {
	for _, field := range c.schema.Fields() {
		if field.IsVectorField() {
			continue
		}
		fv, ok := d.Field(field.Name)
		if !ok || fv.Null {
			continue
		}
		switch {
		case fv.StringVal != "":
			c.metaIndex.AddString(field.Name, d.DocID, fv.StringVal)
		case fv.Type == types.DataTypeInt64:
			c.metaIndex.AddInt64(field.Name, d.DocID, fv.Int64Val)
		case fv.Type == types.DataTypeBool:
			c.metaIndex.AddBool(field.Name, d.DocID, fv.BoolVal)
		}
	}
}

func (c *Collection) deleteFromMetaIndex(pk string) {
	d := c.getDocByPK(pk)
	if d == nil {
		return
	}
	c.metaIndex.DeleteDoc(d.DocID)
}

func (c *Collection) addAllIndexesConcurrentBatch(docs []*doc.Doc) {
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer recoverDone(&wg, "addAllIndexesConcurrentBatch")
		for _, d := range docs {
			c.addToIndexes(d)
		}
	}()
	go func() {
		defer recoverDone(&wg, "addAllIndexesConcurrentBatch")
		for _, d := range docs {
			c.addToFTSIndexes(d)
		}
	}()
	go func() {
		defer recoverDone(&wg, "addAllIndexesConcurrentBatch")
		for _, d := range docs {
			c.addToInvertIndexes(d)
		}
	}()
	go func() {
		defer recoverDone(&wg, "addAllIndexesConcurrentBatch")
		for _, d := range docs {
			c.addToMetaIndex(d)
		}
	}()
	wg.Wait()
}

func (c *Collection) addAllIndexesConcurrent(d *doc.Doc) {
	var wg sync.WaitGroup
	wg.Add(4)
	go func() { defer recoverDone(&wg, "addAllIndexesConcurrent"); c.addToIndexes(d) }()
	go func() { defer recoverDone(&wg, "addAllIndexesConcurrent"); c.addToFTSIndexes(d) }()
	go func() { defer recoverDone(&wg, "addAllIndexesConcurrent"); c.addToInvertIndexes(d) }()
	go func() { defer recoverDone(&wg, "addAllIndexesConcurrent"); c.addToMetaIndex(d) }()
	wg.Wait()
}

func (c *Collection) deleteAllIndexesConcurrentBatch(pks []string) {
	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer recoverDone(&wg, "deleteAllIndexesConcurrentBatch")
		for _, pk := range pks {
			c.deleteFromIndexes(pk)
		}
	}()
	go func() {
		defer recoverDone(&wg, "deleteAllIndexesConcurrentBatch")
		for _, pk := range pks {
			c.deleteFromFTSIndexes(pk)
		}
	}()
	go func() {
		defer recoverDone(&wg, "deleteAllIndexesConcurrentBatch")
		for _, pk := range pks {
			c.deleteFromInvertIndexes(pk)
		}
	}()
	go func() {
		defer recoverDone(&wg, "deleteAllIndexesConcurrentBatch")
		for _, pk := range pks {
			c.deleteFromMetaIndex(pk)
		}
	}()
	wg.Wait()
}

func (c *Collection) deleteAllIndexesConcurrent(pk string) {
	var wg sync.WaitGroup
	wg.Add(4)
	go func() { defer recoverDone(&wg, "deleteAllIndexesConcurrent"); c.deleteFromIndexes(pk) }()
	go func() { defer recoverDone(&wg, "deleteAllIndexesConcurrent"); c.deleteFromFTSIndexes(pk) }()
	go func() { defer recoverDone(&wg, "deleteAllIndexesConcurrent"); c.deleteFromInvertIndexes(pk) }()
	go func() { defer recoverDone(&wg, "deleteAllIndexesConcurrent"); c.deleteFromMetaIndex(pk) }()
	wg.Wait()
}

func (c *Collection) allDocs() []*doc.Doc {
	if c.segManager != nil {
		return c.segManager.AllDocs()
	}
	return c.docs.AllDocs()
}

func (c *Collection) pkExists(pk string) bool {
	if c.segManager != nil {
		return c.segManager.DocExists(pk)
	}
	_, exists := c.docIndex.Get(pk)
	return exists
}

func (c *Collection) Query(q *query.SearchQuery) ([]map[string]interface{}, status.Status) {
	if q.TopK <= 0 {
		return nil, status.NewInvalidArgument("TopK must be positive")
	}
	if q.Target.FieldName == "" {
		return nil, status.NewInvalidArgument("field name cannot be empty")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	c.indexMu.RLock()
	defer c.indexMu.RUnlock()

	var results []types.SearchResult
	if q.Target.FTS != nil {
		ftsIdx, ok := c.ftsIndexes[q.Target.FieldName]
		if !ok {
			return nil, status.NewNotFound(
				fmt.Sprintf("no FTS index on field '%s'", q.Target.FieldName))
		}
		ftsResults := ftsIdx.Search(q.Target.FTS.QueryString, q.TopK)
		for _, fr := range ftsResults {
			pk := c.resolveDocIDToPK(fr.DocID)
			results = append(results, types.SearchResult{
				DocID: fr.DocID,
				Score: float32(fr.Score),
				PK:    pk,
			})
		}
	} else {
		idx, ok := c.indexes[q.Target.FieldName]
		if !ok {
			return nil, status.NewNotFound(
				fmt.Sprintf("no index on field '%s'", q.Target.FieldName))
		}

		if q.Filter != "" {
			results = idx.SearchWithFilter(
				q.Target.Vector.QueryVector, q.TopK,
				func(pk string) bool {
					d := c.getDocByPK(pk)
					if d == nil {
						return false
					}
					return matchFilter(d, q.Filter)
				})
		} else if q.MetaFilter != nil && len(q.MetaFilter.Conditions) > 0 {
			allowedDocIDs := c.resolveMetaFilter(q.MetaFilter)
			if len(allowedDocIDs) == 0 {
				results = nil
			} else {
				allowedSet := make(map[uint64]struct{}, len(allowedDocIDs))
				for _, id := range allowedDocIDs {
					allowedSet[id] = struct{}{}
				}
				results = idx.SearchWithFilter(
					q.Target.Vector.QueryVector, q.TopK,
					func(pk string) bool {
						d := c.getDocByPK(pk)
						if d == nil {
							return false
						}
						_, ok := allowedSet[d.DocID]
						return ok
					})
			}
		} else {
			results = idx.Search(q.Target.Vector.QueryVector, q.TopK)
		}
	}

	outputs := make([]map[string]interface{}, len(results))
	for i, r := range results {
		docObj := c.getDocByPK(r.PK)
		item := map[string]interface{}{
			"id":    r.PK,
			"score": r.Score,
		}

		if q.IncludeDocID && docObj != nil {
			item["doc_id"] = docObj.DocID
		}

		if docObj != nil && len(q.OutputFields) > 0 {
			for _, fn := range q.OutputFields {
				if fn == "id" || fn == "score" || fn == "doc_id" {
					continue
				}
				if fv, ok := docObj.Field(fn); ok {
					item[fn] = extractValue(fv)
				}
				if vv, ok := docObj.Vector(fn); ok {
					item[fn] = vv
				}
			}
		} else if docObj != nil && q.IncludeVector {
			for _, fn := range docObj.VectorNames() {
				v, _ := docObj.Vector(fn)
				item[fn] = v
			}
			for _, fn := range docObj.FieldNames() {
				fv, _ := docObj.Field(fn)
				item[fn] = extractValue(fv)
			}
		}

		outputs[i] = item
	}

	return outputs, status.OKStatus()
}

func (c *Collection) BatchQuery(queries []*query.SearchQuery) [][]map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.indexMu.RLock()
	defer c.indexMu.RUnlock()

	results := make([][]map[string]interface{}, len(queries))
	var wg sync.WaitGroup
	wg.Add(len(queries))

	for i, q := range queries {
		go func(idx int, q *query.SearchQuery) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("BatchQuery goroutine panic recovered", "index", idx, "recover", r)
					results[idx] = nil
				}
			}()

			var searchResults []types.SearchResult

			if q.Target.FTS != nil {
				ftsIdx, ok := c.ftsIndexes[q.Target.FieldName]
				if !ok {
					return
				}
				ftsResults := ftsIdx.Search(q.Target.FTS.QueryString, q.TopK)
				for _, fr := range ftsResults {
					pk := c.resolveDocIDToPK(fr.DocID)
					searchResults = append(searchResults, types.SearchResult{
						DocID: fr.DocID,
						Score: float32(fr.Score),
						PK:    pk,
					})
				}
			} else {
				idx2, ok := c.indexes[q.Target.FieldName]
				if !ok {
					return
				}
				if q.Filter != "" {
					searchResults = idx2.SearchWithFilter(
						q.Target.Vector.QueryVector, q.TopK,
						func(pk string) bool {
							d := c.getDocByPK(pk)
							if d == nil {
								return false
							}
							return matchFilter(d, q.Filter)
						})
				} else {
					searchResults = idx2.Search(q.Target.Vector.QueryVector, q.TopK)
				}
			}

			outputs := make([]map[string]interface{}, len(searchResults))
			for j, r := range searchResults {
				docObj := c.getDocByPK(r.PK)
				item := map[string]interface{}{
					"id":    r.PK,
					"score": r.Score,
				}
				if q.IncludeDocID && docObj != nil {
					item["doc_id"] = docObj.DocID
				}
				if docObj != nil && len(q.OutputFields) > 0 {
					for _, fn := range q.OutputFields {
						if fn == "id" || fn == "score" || fn == "doc_id" {
							continue
						}
						if fv, ok := docObj.Field(fn); ok {
							item[fn] = extractValue(fv)
						}
						if vv, ok := docObj.Vector(fn); ok {
							item[fn] = vv
						}
					}
				} else if docObj != nil && q.IncludeVector {
					for _, fn := range docObj.VectorNames() {
						v, _ := docObj.Vector(fn)
						item[fn] = v
					}
					for _, fn := range docObj.FieldNames() {
						fv, _ := docObj.Field(fn)
						item[fn] = extractValue(fv)
					}
				}
				outputs[j] = item
			}
			results[idx] = outputs
		}(i, q)
	}

	wg.Wait()
	return results
}

func (c *Collection) FTSQuery(fieldName string, queryStr string, topK int) ([]map[string]interface{}, status.Status) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.ftsMu.RLock()
	defer c.ftsMu.RUnlock()

	ftsIdx, ok := c.ftsIndexes[fieldName]
	if !ok {
		return nil, status.NewNotFound(fmt.Sprintf("no FTS index on field '%s'", fieldName))
	}

	results := ftsIdx.Search(queryStr, topK)
	outputs := make([]map[string]interface{}, len(results))
	for i, r := range results {
		pk := c.resolveDocIDToPK(r.DocID)
		outputs[i] = map[string]interface{}{
			"id":    pk,
			"score": r.Score,
			"text":  r.DocText,
		}
	}
	return outputs, status.OKStatus()
}

func (c *Collection) MultiQuery(mq *query.MultiQuery) ([]map[string]interface{}, status.Status) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.indexMu.RLock()
	defer c.indexMu.RUnlock()

	var allResults [][]types.SearchResult
	for _, sq := range mq.SubQueries {
		if sq.Target.FTS != nil {
			ftsIdx, ok := c.ftsIndexes[sq.Target.FieldName]
			if !ok {
				continue
			}
			topK := sq.NumCandidates
			if topK <= 0 {
				topK = mq.TopK * 2
			}
			ftsResults := ftsIdx.Search(sq.Target.FTS.QueryString, topK)
			results := make([]types.SearchResult, len(ftsResults))
			for i, r := range ftsResults {
				results[i] = types.SearchResult{
					DocID: r.DocID,
					Score: float32(r.Score),
					PK:    c.resolveDocIDToPK(r.DocID),
				}
			}
			allResults = append(allResults, results)
			continue
		}
		if sq.Target.Vector == nil {
			continue
		}
		idx, ok := c.indexes[sq.Target.FieldName]
		if !ok {
			continue
		}
		topK := sq.NumCandidates
		if topK <= 0 {
			topK = mq.TopK * 2
		}
		var results []types.SearchResult
		if mq.Filter != "" {
			filterFn := compileFilter(mq.Filter)
			results = idx.SearchWithFilter(sq.Target.Vector.QueryVector, topK,
				func(pk string) bool {
					d := c.getDocByPK(pk)
					if d == nil {
						return false
					}
					return filterFn(d)
				})
		} else {
			results = idx.Search(sq.Target.Vector.QueryVector, topK)
		}
		allResults = append(allResults, results)
	}

	if len(allResults) == 0 {
		return nil, status.OKStatus()
	}

	var rerankParams interface{}
	switch mq.Rerank.Type {
	case query.RerankTypeWeighted:
		rerankParams = reranker.NewWeightedParams(mq.Rerank.Weights)
	case query.RerankTypeCallback:
		rerankParams = &reranker.CallbackParams{Callback: mq.Rerank.Callback}
	default:
		rrfC := mq.Rerank.RRFConstant
		if rrfC <= 0 {
			rrfC = 60
		}
		rerankParams = reranker.NewRRFParams(rrfC)
	}

	final := reranker.Rerank(rerankParams, allResults, mq.TopK)
	outputs := make([]map[string]interface{}, len(final))
	for i, r := range final {
		docObj := c.getDocByPK(r.PK)
		item := map[string]interface{}{
			"id":    r.PK,
			"score": r.Score,
		}
		if mq.IncludeDocID && docObj != nil {
			item["doc_id"] = docObj.DocID
		}
		if docObj != nil {
			if len(mq.OutputFields) > 0 {
				for _, fn := range mq.OutputFields {
					if fn == "id" || fn == "score" {
						continue
					}
					if fv, ok := docObj.Field(fn); ok {
						item[fn] = extractValue(fv)
					}
				}
			} else if mq.IncludeVector {
				for _, fn := range docObj.VectorNames() {
					v, _ := docObj.Vector(fn)
					item[fn] = v
				}
			}
		}
		outputs[i] = item
	}
	return outputs, status.OKStatus()
}

func (c *Collection) HybridQuery(vectorFieldName string, queryVector []float32, ftsFieldName string, queryString string, topK int, rerankType query.RerankType, weights []float64) ([]map[string]interface{}, status.Status) {
	mq := &query.MultiQuery{
		TopK: topK,
		SubQueries: []query.SubQuery{
			{
				Target: query.QueryTarget{
					FieldName: vectorFieldName,
					Vector:    &query.VectorClause{QueryVector: queryVector},
				},
				NumCandidates: topK * 3,
			},
			{
				Target: query.QueryTarget{
					FieldName: ftsFieldName,
					FTS:       &query.FTSClause{QueryString: queryString},
				},
			},
		},
		Rerank: query.RerankParams{
			Type:    rerankType,
			Weights: weights,
		},
	}
	return c.MultiQuery(mq)
}

func (c *Collection) GroupBy(gq *query.GroupByVectorQuery) ([]query.GroupResult, status.Status) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.indexMu.RLock()
	defer c.indexMu.RUnlock()

	idx, ok := c.indexes[gq.Target.FieldName]
	if !ok {
		return nil, status.NewNotFound(
			fmt.Sprintf("no index on field '%s'", gq.Target.FieldName))
	}

	docCount := 0
	if c.segManager != nil {
		docCount = c.segManager.DocCount()
	} else {
		docCount = c.docs.Len()
	}

	topKSearch := docCount
	if bound := gq.TopKPerGroup * gq.GroupCount * 3; bound > 0 && bound < topKSearch {
		topKSearch = bound
	}

	var searchResults []types.SearchResult
	if gq.Filter != "" {
		filterFn := compileFilter(gq.Filter)
		searchResults = idx.SearchWithFilter(
			gq.Target.Vector.QueryVector, topKSearch,
			func(pk string) bool {
				d := c.getDocByPK(pk)
				if d == nil {
					return false
				}
				return filterFn(d)
			})
	} else {
		searchResults = idx.Search(gq.Target.Vector.QueryVector, topKSearch)
	}

	groups := make(map[string][]map[string]interface{})
	for _, r := range searchResults {
		d := c.getDocByPK(r.PK)
		if d == nil {
			continue
		}

		fv, ok := d.Field(gq.GroupByField)
		if !ok {
			continue
		}
		groupKey := fmt.Sprintf("%v", extractValue(fv))
		if len(groups[groupKey]) >= gq.TopKPerGroup {
			continue
		}

		item := map[string]interface{}{
			"id":    r.PK,
			"score": r.Score,
		}
		if len(gq.OutputFields) > 0 {
			for _, fn := range gq.OutputFields {
				if fn == "id" || fn == "score" {
					continue
				}
				if fv2, ok2 := d.Field(fn); ok2 {
					item[fn] = extractValue(fv2)
				}
			}
		} else if gq.IncludeVector {
			for _, fn := range d.VectorNames() {
				v, _ := d.Vector(fn)
				item[fn] = v
			}
		}
		groups[groupKey] = append(groups[groupKey], item)
	}

	results := make([]query.GroupResult, 0, len(groups))
	groupCount := gq.GroupCount
	if groupCount <= 0 || groupCount > len(groups) {
		groupCount = len(groups)
	}
	sortedKeys := make([]string, 0, len(groups))
	for key := range groups {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)
	for _, key := range sortedKeys {
		if len(results) >= groupCount {
			break
		}
		results = append(results, query.GroupResult{
			GroupByValue: key,
			Docs:         groups[key],
		})
	}
	return results, status.OKStatus()
}

func (c *Collection) Fetch(ids []string, outputFields []string,
	includeVector bool) (map[string]*doc.Doc, status.Status) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*doc.Doc)
	for _, id := range ids {
		d := c.getDocByPK(id)
		if d != nil {
			result[id] = d
		}
	}
	return result, status.OKStatus()
}

func (c *Collection) CreateIndex(fieldName string, params param.IndexConfig) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	field := c.schema.GetField(fieldName)
	if field == nil {
		return status.NewNotFound(fmt.Sprintf("field '%s' not found", fieldName))
	}

	if params != nil {
		field.IndexParam = params
	}

	if params != nil && params.GetIndexType() == types.IndexTypeInvert {
		invIdx := invert.NewInvertIndex()
		c.invertIndexes[fieldName] = invIdx
		allDocs := c.allDocs()
		for _, d := range allDocs {
			if fv, ok := d.Field(fieldName); ok && !fv.Null && fv.StringVal != "" {
				invIdx.Add(d.DocID, fv.StringVal)
			}
		}
		return status.OKStatus()
	}

	idx, err := createIndex(field)
	if err != nil {
		return status.NewInternalError(err.Error())
	}
	c.indexes[fieldName] = idx

	allDocs := c.allDocs()
	for _, d := range allDocs {
		v, ok := d.Vector(fieldName)
		if ok && v.Float32s != nil {
			idx.Add(v.Float32s, d.ID)
		}
	}

	return status.OKStatus()
}

func (c *Collection) DropIndex(fieldName string) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	if idx, ok := c.indexes[fieldName]; ok {
		idx.Close()
	}
	delete(c.indexes, fieldName)
	delete(c.invertIndexes, fieldName)
	delete(c.ftsIndexes, fieldName)
	return status.OKStatus()
}

type OptimizeOptions struct {
	Concurrency int
}

func (c *Collection) Optimize(opts *OptimizeOptions) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, idx := range c.indexes {
		idx.Close()
	}
	allDocs := c.allDocs()

	c.indexes = make(map[string]index.Index)
	for _, field := range c.schema.VectorFields() {
		idx, err := createIndex(field)
		if err != nil {
			return status.NewInternalError(fmt.Sprintf("failed to rebuild index for field '%s': %v", field.Name, err))
		}
		c.indexes[field.Name] = idx

		for _, d := range allDocs {
			v, ok := d.Vector(field.Name)
			if ok && v.Float32s != nil {
				idx.Add(v.Float32s, d.ID)
			}
		}
	}

	c.ftsIndexes = make(map[string]FTSIndexer)
	for _, field := range c.schema.FTSFields() {
		tokenizer := createTokenizer(field)
		ftsIdx := fts.NewFTSIndex(tokenizer)
		c.ftsIndexes[field.Name] = ftsIdx

		for _, d := range allDocs {
			if fv, ok := d.Field(field.Name); ok && !fv.Null {
				ftsIdx.Index(d.DocID, fv.StringVal)
			}
		}
	}

	c.invertIndexes = make(map[string]InvertIndexer)
	for _, field := range c.schema.InvertFields() {
		invIdx := invert.NewInvertIndex()
		c.invertIndexes[field.Name] = invIdx

		for _, d := range allDocs {
			if fv, ok := d.Field(field.Name); ok && !fv.Null && fv.StringVal != "" {
				invIdx.Add(d.DocID, fv.StringVal)
			}
		}
	}

	return status.OKStatus()
}

func (c *Collection) AddColumn(fieldSchema *schema.FieldSchema, defaultExpr string) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.schema.AddField(fieldSchema)
}

func (c *Collection) DropColumn(fieldName string) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.schema.DropField(fieldName)
}

func (c *Collection) AlterColumn(oldName, newName string, newSchema *schema.FieldSchema) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.schema.AlterField(oldName, newName, newSchema)
}

func (c *Collection) BatchBuild(fieldName string, vectors [][]float32, pks []string) status.Status {
	c.indexMu.Lock()
	defer c.indexMu.Unlock()

	idx, ok := c.indexes[fieldName]
	if !ok {
		return status.NewNotFound(fmt.Sprintf("index for field '%s' not found", fieldName))
	}

	builder, ok := idx.(index.BatchBuilder)
	if !ok {
		return status.NewInvalidArgument(fmt.Sprintf("index for field '%s' does not support BatchBuild", fieldName))
	}

	builder.BatchBuild(vectors, pks)
	return status.OKStatus()
}

func (c *Collection) Stats() *schema.CollectionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	docCount := 0
	if c.segManager != nil {
		docCount = c.segManager.DocCount()
	} else {
		docCount = c.docs.Len()
	}

	var totalMem uint64
	for _, idx := range c.indexes {
		totalMem += idx.MemoryBytes()
	}

	return &schema.CollectionStats{
		DocCount:         uint64(docCount),
		TotalMemoryBytes: totalMem,
	}
}

func createTokenizer(field *schema.FieldSchema) fts.Tokenizer {
	if field.IndexParam != nil {
		if ftsP, ok := field.IndexParam.(*param.FTSParams); ok && ftsP.Tokenizer == "jieba" {
			return fts.NewJiebaTokenizer()
		}
	}
	return fts.NewStandardTokenizer()
}

const (
	defaultHNSWM              = 50
	defaultHNSWEFConstruction = 500
	defaultIVFNList           = 10
	defaultIVFNIters          = 20
	defaultVamanaMaxDegree    = 64
	defaultVamanaSearchList   = 100
	defaultVamanaAlpha        = 1.2
)

func createIndex(field *schema.FieldSchema) (index.Index, error) {
	if field.IndexParam == nil {
		return flat.NewFlatIndex(field.Dimension, types.MetricTypeCosine), nil
	}

	switch p := field.IndexParam.(type) {
	case *param.HNSWParams:
		m := p.M
		if m <= 0 {
			m = defaultHNSWM
		}
		ef := p.EFConstruction
		if ef <= 0 {
			ef = defaultHNSWEFConstruction
		}
		return hnsw.NewShardedHNSWIndex(16, field.Dimension, p.MetricType, m, ef), nil
	case *param.IVFParams:
		nList := p.NList
		if nList <= 0 {
			nList = defaultIVFNList
		}
		nIters := p.NIters
		if nIters <= 0 {
			nIters = defaultIVFNIters
		}
		return ivf.NewIVFIndex(field.Dimension, p.MetricType, nList, nIters), nil
	case *param.VamanaParams:
		maxDeg := p.MaxDegree
		if maxDeg <= 0 {
			maxDeg = defaultVamanaMaxDegree
		}
		searchList := p.SearchListSize
		if searchList <= 0 {
			searchList = defaultVamanaSearchList
		}
		alpha := p.Alpha
		if alpha <= 0 {
			alpha = defaultVamanaAlpha
		}
		return vamana.NewVamanaIndex(field.Dimension, p.MetricType,
			maxDeg, searchList, alpha, p.SaturateGraph), nil
	case *param.DiskAnnParams:
		maxDeg := p.MaxDegree
		if maxDeg <= 0 {
			maxDeg = defaultVamanaMaxDegree
		}
		searchList := p.ListSize
		if searchList <= 0 {
			searchList = defaultVamanaSearchList
		}
		alpha := p.Alpha
		if alpha <= 0 {
			alpha = defaultVamanaAlpha
		}
		return diskann.NewShardedDiskAnnIndex(16, field.Dimension, p.MetricType,
			maxDeg, searchList, alpha, false), nil
	case *param.HNSWRabitqParams:
		m := p.M
		if m <= 0 {
			m = defaultHNSWM
		}
		ef := p.EFConstruction
		if ef <= 0 {
			ef = defaultHNSWEFConstruction
		}
		return hnsw_rabitq.NewHNSWRabitqIndex(field.Dimension, p.MetricType, m, ef), nil
	case *param.FlatParams:
		return flat.NewFlatIndex(field.Dimension, p.MetricType), nil
	case *param.IndexParams:
		cfg := param.IndexConfigFromLegacy(p)
		field.IndexParam = cfg
		return createIndex(field)
	case *param.InvertParams:
		return nil, errors.New("Invert index not yet implemented")
	case *param.FTSParams:
		return nil, errors.New("FTS index not created via createIndex")
	default:
		return nil, fmt.Errorf("unsupported index type: %T", field.IndexParam)
	}
}

func matchFilter(d *doc.Doc, filter string) bool {
	if filter == "" {
		return true
	}
	fn := compileFilter(filter)
	return fn(d)
}

func numericVal(v doc.Value) float64 {
	switch v.Type {
	case types.DataTypeDouble:
		return v.DoubleVal
	case types.DataTypeFloat:
		return float64(v.FloatVal)
	case types.DataTypeInt64:
		return float64(v.Int64Val)
	case types.DataTypeUint64:
		return float64(v.Uint64Val)
	case types.DataTypeInt32:
		return float64(v.Int32Val)
	case types.DataTypeUint32:
		return float64(v.Uint32Val)
	default:

		return 0
	}
}

func compileFilter(filter string) func(*doc.Doc) bool {
	f := strings.TrimSpace(filter)
	if f == "" {
		return func(d *doc.Doc) bool { return true }
	}

	if strings.HasSuffix(f, " IS_NULL") {
		fieldName := strings.TrimSpace(strings.TrimSuffix(f, " IS_NULL"))
		return func(d *doc.Doc) bool {
			v, ok := d.Field(fieldName)
			if !ok {
				return false
			}
			return v.Null
		}
	}
	if strings.HasSuffix(f, " IS_NOT_NULL") {
		fieldName := strings.TrimSpace(strings.TrimSuffix(f, " IS_NOT_NULL"))
		return func(d *doc.Doc) bool {
			v, ok := d.Field(fieldName)
			if !ok {
				return false
			}
			return !v.Null
		}
	}

	type keywordOp struct {
		keyword string
		opType  types.CompareOp
	}
	keywordOps := []keywordOp{
		{"NOT_CONTAIN_ALL", types.CompareOpNotContainAll},
		{"NOT_CONTAIN_ANY", types.CompareOpNotContainAny},
		{"CONTAIN_ALL", types.CompareOpContainAll},
		{"CONTAIN_ANY", types.CompareOpContainAny},
		{"HAS_PREFIX", types.CompareOpHasPrefix},
		{"HAS_SUFFIX", types.CompareOpHasSuffix},
		{"LIKE", types.CompareOpLike},
	}

	for _, ko := range keywordOps {
		idx := strings.Index(f, " "+ko.keyword+" ")
		if idx < 0 {
			continue
		}
		fieldName := strings.TrimSpace(f[:idx])
		valStr := strings.TrimSpace(f[idx+1+len(ko.keyword):])

		switch ko.opType {
		case types.CompareOpLike:
			pattern := valStr
			return func(d *doc.Doc) bool {
				v, ok := d.Field(fieldName)
				if !ok || v.Null {
					return false
				}
				return wildcardMatch(v.StringVal, pattern)
			}
		case types.CompareOpHasPrefix:
			return func(d *doc.Doc) bool {
				v, ok := d.Field(fieldName)
				if !ok || v.Null {
					return false
				}
				return strings.HasPrefix(v.StringVal, valStr)
			}
		case types.CompareOpHasSuffix:
			return func(d *doc.Doc) bool {
				v, ok := d.Field(fieldName)
				if !ok || v.Null {
					return false
				}
				return strings.HasSuffix(v.StringVal, valStr)
			}
		case types.CompareOpContainAll:
			vals := strings.Split(valStr, ",")
			return func(d *doc.Doc) bool {
				v, ok := d.Field(fieldName)
				if !ok || v.Null {
					return false
				}
				for _, val := range vals {
					if !strings.Contains(v.StringVal, strings.TrimSpace(val)) {
						return false
					}
				}
				return true
			}
		case types.CompareOpContainAny:
			vals := strings.Split(valStr, ",")
			return func(d *doc.Doc) bool {
				v, ok := d.Field(fieldName)
				if !ok || v.Null {
					return false
				}
				for _, val := range vals {
					if strings.Contains(v.StringVal, strings.TrimSpace(val)) {
						return true
					}
				}
				return false
			}
		case types.CompareOpNotContainAll:
			vals := strings.Split(valStr, ",")
			return func(d *doc.Doc) bool {
				v, ok := d.Field(fieldName)
				if !ok || v.Null {
					return false
				}
				for _, val := range vals {
					if !strings.Contains(v.StringVal, strings.TrimSpace(val)) {
						return true
					}
				}
				return false
			}
		case types.CompareOpNotContainAny:
			vals := strings.Split(valStr, ",")
			return func(d *doc.Doc) bool {
				v, ok := d.Field(fieldName)
				if !ok || v.Null {
					return false
				}
				for _, val := range vals {
					if strings.Contains(v.StringVal, strings.TrimSpace(val)) {
						return false
					}
				}
				return true
			}
		}
	}

	type opFunc func(float64, float64) bool
	ops := []struct {
		sep        string
		fn         opFunc
		isNotEqual bool
		isEqual    bool
	}{
		{">=", func(a, b float64) bool { return a >= b }, false, false},
		{"<=", func(a, b float64) bool { return a <= b }, false, false},
		{">", func(a, b float64) bool { return a > b }, false, false},
		{"<", func(a, b float64) bool { return a < b }, false, false},
		{"!=", nil, true, false},
		{"==", nil, false, true},
		{"=", nil, false, true},
	}

	return func(d *doc.Doc) bool {
		for _, op := range ops {
			if !strings.Contains(f, op.sep) {
				continue
			}
			parts := strings.SplitN(f, op.sep, 2)
			if len(parts) != 2 {
				continue
			}
			fieldName := strings.TrimSpace(parts[0])
			valStr := strings.TrimSpace(parts[1])
			v, ok := d.Field(fieldName)
			if !ok || v.Null {
				return false
			}
			if op.isNotEqual {
				if v.StringVal != "" {
					return v.StringVal != valStr
				}
				val, err := strconv.ParseFloat(valStr, 64)
				if err != nil {
					return v.StringVal != valStr
				}
				return numericVal(v) != val
			}
			if op.isEqual {
				if v.StringVal != "" {
					return v.StringVal == valStr
				}
				val, err := strconv.ParseFloat(valStr, 64)
				if err != nil {
					return v.StringVal == valStr
				}
				return numericVal(v) == val
			}
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil {
				return false
			}
			return op.fn(numericVal(v), val)
		}
		return true
	}
}

func wildcardMatch(s, pattern string) bool {
	sRunes := []rune(s)
	pRunes := []rune(pattern)
	si, pi := 0, 0
	starIdx := -1
	matchIdx := 0
	for si < len(sRunes) {
		if pi < len(pRunes) && (pRunes[pi] == '?' || pRunes[pi] == sRunes[si]) {
			si++
			pi++
		} else if pi < len(pRunes) && pRunes[pi] == '*' {
			starIdx = pi
			matchIdx = si
			pi++
		} else if starIdx != -1 {
			pi = starIdx + 1
			matchIdx++
			si = matchIdx
		} else {
			return false
		}
	}
	for pi < len(pRunes) && pRunes[pi] == '*' {
		pi++
	}
	return pi == len(pRunes)
}

func extractValue(v doc.Value) interface{} {
	if v.Null {
		return nil
	}
	switch v.Type {
	case types.DataTypeBool:
		return v.BoolVal
	case types.DataTypeInt32:
		return v.Int32Val
	case types.DataTypeUint32:
		return v.Uint32Val
	case types.DataTypeInt64:
		return v.Int64Val
	case types.DataTypeUint64:
		return v.Uint64Val
	case types.DataTypeFloat:
		return v.FloatVal
	case types.DataTypeDouble:
		return v.DoubleVal
	case types.DataTypeString:
		return v.StringVal
	default:
		return v.StringVal
	}
}
