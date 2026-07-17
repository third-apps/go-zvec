package collection

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/third-apps/go-zvec/doc"
	"github.com/third-apps/go-zvec/fts"
	"github.com/third-apps/go-zvec/index/diskann"
	"github.com/third-apps/go-zvec/index/flat"
	"github.com/third-apps/go-zvec/index/hnsw"
	"github.com/third-apps/go-zvec/index/hnsw_rabitq"
	"github.com/third-apps/go-zvec/index/invert"
	"github.com/third-apps/go-zvec/index/ivf"
	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/index/vamana"
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
	path          string
	schema        *schema.CollectionSchema
	options       Options
	docs          []*doc.Doc
	docIndex      map[string]int
	docIDToPK     map[uint64]string
	nextDocID     uint64
	indexes       map[string]Index
	ftsIndexes    map[string]*fts.FTSIndex
	invertIndexes map[string]*invert.InvertIndex
	wal           *wal.WAL
	segManager    *segment.Manager
}

type Index interface {
	Search(query []float32, topK int) []flat.SearchResult
	SearchWithFilter(query []float32, topK int, filterFn func(pk string) bool) []flat.SearchResult
	Add(vector []float32, pk string) uint64
	Delete(pk string) bool
	Size() int
	Close() error
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
		docs:          make([]*doc.Doc, 0),
		docIndex:      make(map[string]int),
		docIDToPK:     make(map[uint64]string),
		indexes:       make(map[string]Index),
		ftsIndexes:    make(map[string]*fts.FTSIndex),
		invertIndexes: make(map[string]*invert.InvertIndex),
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
		tok := fts.NewStandardTokenizer()
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
		docs:          make([]*doc.Doc, 0),
		docIndex:      make(map[string]int),
		docIDToPK:     make(map[uint64]string),
		indexes:       make(map[string]Index),
		ftsIndexes:    make(map[string]*fts.FTSIndex),
		invertIndexes: make(map[string]*invert.InvertIndex),
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
		tok := fts.NewStandardTokenizer()
		c.ftsIndexes[field.Name] = fts.NewFTSIndex(tok)
	}

	for _, field := range s.InvertFields() {
		c.invertIndexes[field.Name] = invert.NewInvertIndex()
	}

	walF, err := wal.Open(filepath.Join(path, "wal.log"))
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
	f, err := os.Open(filepath.Join(c.path, "wal.log"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry wal.LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		c.replayEntry(entry)
	}
	return scanner.Err()

}

func (c *Collection) replayEntry(entry wal.LogEntry) {
	switch entry.Op {
	case wal.OpInsert:
		if entry.Doc == nil {
			return
		}
		if c.segManager != nil {
			c.segManager.Insert(entry.Doc)
		} else {
			c.docs = append(c.docs, entry.Doc)
			c.docIndex[entry.Doc.ID] = len(c.docs) - 1
			c.docIDToPK[entry.Doc.DocID] = entry.Doc.ID
		}
		if entry.Doc.DocID >= c.nextDocID {
			c.nextDocID = entry.Doc.DocID + 1
		}
		for _, field := range c.schema.VectorFields() {
			if idx, ok := c.indexes[field.Name]; ok {
				if v, ok2 := entry.Doc.Vector(field.Name); ok2 && v.Float32s != nil {
					idx.Add(v.Float32s, entry.Doc.ID)
				}
			}
		}
		for _, field := range c.schema.FTSFields() {
			if ftsIdx, ok := c.ftsIndexes[field.Name]; ok {
				if fv, ok2 := entry.Doc.Field(field.Name); ok2 && !fv.Null {
					ftsIdx.Index(entry.Doc.DocID, fv.StringVal)
				}
			}
		}
	case wal.OpUpsert:
		if entry.Doc == nil {
			return
		}
		if c.segManager != nil {
			if existing := c.segManager.GetDoc(entry.Doc.ID); existing != nil {
				entry.Doc.DocID = existing.DocID
				c.deleteFromIndexes(existing.ID)
				c.deleteFromFTSIndexes(existing.ID)
				c.deleteFromInvertIndexes(existing.ID)
				c.segManager.Upsert(entry.Doc)
				c.addToIndexes(entry.Doc)
				c.addToFTSIndexes(entry.Doc)
				c.addToInvertIndexes(entry.Doc)
			} else {
				c.segManager.Insert(entry.Doc)
				c.addToIndexes(entry.Doc)
				c.addToFTSIndexes(entry.Doc)
				c.addToInvertIndexes(entry.Doc)
			}
		} else {
			if existingIdx, exists := c.docIndex[entry.Doc.ID]; exists {
				c.deleteFromIndexes(c.docs[existingIdx].ID)
				c.deleteFromFTSIndexes(c.docs[existingIdx].ID)
				c.deleteFromInvertIndexes(c.docs[existingIdx].ID)
				entry.Doc.DocID = c.docs[existingIdx].DocID
				c.docs[existingIdx] = entry.Doc
				c.addToIndexes(entry.Doc)
				c.addToFTSIndexes(entry.Doc)
				c.addToInvertIndexes(entry.Doc)
			} else {
				c.docs = append(c.docs, entry.Doc)
				c.docIndex[entry.Doc.ID] = len(c.docs) - 1
				c.docIDToPK[entry.Doc.DocID] = entry.Doc.ID
				if entry.Doc.DocID >= c.nextDocID {
					c.nextDocID = entry.Doc.DocID + 1
				}
				c.addToIndexes(entry.Doc)
				c.addToFTSIndexes(entry.Doc)
				c.addToInvertIndexes(entry.Doc)
			}
		}
	case wal.OpUpdate:
		if entry.Doc == nil {
			return
		}
		if c.segManager != nil {
			if existing := c.segManager.GetDoc(entry.Doc.ID); existing != nil {
				entry.Doc.DocID = existing.DocID
				c.deleteFromIndexes(existing.ID)
				c.deleteFromFTSIndexes(existing.ID)
				c.deleteFromInvertIndexes(existing.ID)
				c.segManager.Upsert(entry.Doc)
				c.addToIndexes(entry.Doc)
				c.addToFTSIndexes(entry.Doc)
				c.addToInvertIndexes(entry.Doc)
			}
		} else {
			existingIdx, exists := c.docIndex[entry.Doc.ID]
			if !exists {
				return
			}
			c.deleteFromIndexes(c.docs[existingIdx].ID)
			c.deleteFromFTSIndexes(c.docs[existingIdx].ID)
			c.deleteFromInvertIndexes(c.docs[existingIdx].ID)
			entry.Doc.DocID = c.docs[existingIdx].DocID
			c.docs[existingIdx] = entry.Doc
			c.addToIndexes(entry.Doc)
			c.addToFTSIndexes(entry.Doc)
			c.addToInvertIndexes(entry.Doc)
		}
	case wal.OpDelete:
		for _, id := range entry.IDs {
			c.deleteFromIndexes(id)
			c.deleteFromFTSIndexes(id)
			c.deleteFromInvertIndexes(id)
			if c.segManager != nil {
				c.segManager.Delete(id)
			} else {
				if idx, exists := c.docIndex[id]; exists {
					removed := c.docs[idx]
					delete(c.docIDToPK, removed.DocID)
					c.docs = append(c.docs[:idx], c.docs[idx+1:]...)
					delete(c.docIndex, id)
					for pk, pos := range c.docIndex {
						if pos > idx {
							c.docIndex[pk] = pos - 1
						}
					}
				}
			}
		}
	}
}

func (c *Collection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, idx := range c.indexes {
		idx.Close()
	}
	c.indexes = nil
	c.docs = nil
	c.docIndex = nil
	c.docIDToPK = nil
	c.ftsIndexes = nil
	c.invertIndexes = nil
	if c.segManager != nil {
		c.segManager.Close()
		c.segManager = nil
	}
	if c.wal != nil {
		return c.wal.Close()
	}
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

func (c *Collection) Insert(docs []*doc.Doc) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, d := range docs {
		if err := d.Validate(c.schema); err != nil {
			return status.NewInvalidArgument(err.Error())
		}
		if c.pkExists(d.ID) {
			return status.NewInvalidArgument(fmt.Sprintf("doc '%s' already exists, use Upsert instead", d.ID))
		}
	}

	for _, d := range docs {

		docID := c.nextDocID
		c.nextDocID++
		d.DocID = docID
		if c.segManager != nil {
			c.segManager.Insert(d)
		} else {
			c.docs = append(c.docs, d)
			c.docIndex[d.ID] = len(c.docs) - 1
			c.docIDToPK[docID] = d.ID
		}

		c.addToIndexes(d)
		c.addToFTSIndexes(d)
		c.addToInvertIndexes(d)

		if c.wal != nil {
			if err := c.wal.AppendInsert(d.ID, d); err != nil {
				return status.NewInternalError(fmt.Sprintf("WAL write failed: %v", err))
			}
		}
	}

	return status.OKStatus()
}

func (c *Collection) Upsert(docs []*doc.Doc) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, d := range docs {
		if err := d.Validate(c.schema); err != nil {
			return status.NewInvalidArgument(err.Error())
		}
	}

	var newDocs []*doc.Doc
	for _, d := range docs {
		if c.segManager != nil {
			existing := c.segManager.GetDoc(d.ID)
			if existing != nil {
				c.deleteFromIndexes(existing.ID)
				c.deleteFromFTSIndexes(existing.ID)
				c.deleteFromInvertIndexes(existing.ID)
				d.DocID = existing.DocID
				c.segManager.Upsert(d)
				c.addToIndexes(d)
				c.addToFTSIndexes(d)
				c.addToInvertIndexes(d)
			} else {
				newDocs = append(newDocs, d)
			}
		} else if existingIdx, exists := c.docIndex[d.ID]; exists {
			existing := c.docs[existingIdx]
			c.deleteFromIndexes(existing.ID)
			c.deleteFromFTSIndexes(existing.ID)
			c.deleteFromInvertIndexes(existing.ID)
			d.DocID = existing.DocID
			c.docs[existingIdx] = d
			c.addToIndexes(d)
			c.addToFTSIndexes(d)
			c.addToInvertIndexes(d)
		} else {
			newDocs = append(newDocs, d)
		}
	}

	if len(newDocs) > 0 {
		for _, d := range newDocs {
			docID := c.nextDocID
			c.nextDocID++
			d.DocID = docID
			if c.segManager != nil {
				c.segManager.Insert(d)
			} else {
				c.docs = append(c.docs, d)
				c.docIndex[d.ID] = len(c.docs) - 1
				c.docIDToPK[docID] = d.ID
			}
			c.addToIndexes(d)
			c.addToFTSIndexes(d)
			c.addToInvertIndexes(d)
		}
	}

	if c.wal != nil {
		for _, d := range docs {
			if err := c.wal.AppendUpsert(d.ID, d); err != nil {
				return status.NewInternalError(fmt.Sprintf("WAL write failed: %v", err))
			}
		}
	}

	return status.OKStatus()
}

func (c *Collection) Update(docs []*doc.Doc) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, d := range docs {
		if err := d.Validate(c.schema); err != nil {
			return status.NewInvalidArgument(err.Error())
		}

		var existing *doc.Doc
		if c.segManager != nil {
			existing = c.segManager.GetDoc(d.ID)
		} else {
			if existingIdx, exists := c.docIndex[d.ID]; exists {
				existing = c.docs[existingIdx]
			}
		}
		if existing == nil {
			return status.NewNotFound(fmt.Sprintf("doc '%s' not found", d.ID))
		}

		d.DocID = existing.DocID
		if c.segManager != nil {
			c.segManager.Upsert(d)
		} else {
			c.docs[c.docIndex[d.ID]] = d
		}

		c.deleteFromIndexes(existing.ID)
		c.deleteFromFTSIndexes(existing.ID)
		c.deleteFromInvertIndexes(existing.ID)
		c.addToIndexes(d)
		c.addToFTSIndexes(d)
		c.addToInvertIndexes(d)
	}

	if c.wal != nil {
		for _, d := range docs {
			if err := c.wal.AppendUpdate(d.ID, d); err != nil {
				return status.NewInternalError(fmt.Sprintf("WAL write failed: %v", err))
			}
		}
	}

	return status.OKStatus()
}

func (c *Collection) Delete(ids []string) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, id := range ids {
		c.deleteDoc(id)
	}

	if c.wal != nil {
		if err := c.wal.AppendDeletes(ids); err != nil {
			return status.NewInternalError(fmt.Sprintf("WAL write failed: %v", err))
		}
	}

	return status.OKStatus()
}

func (c *Collection) deleteDoc(id string) {
	c.deleteFromIndexes(id)
	c.deleteFromFTSIndexes(id)
	c.deleteFromInvertIndexes(id)
	if c.segManager != nil {
		c.segManager.Delete(id)
	} else if idx, exists := c.docIndex[id]; exists {
		removed := c.docs[idx]
		delete(c.docIDToPK, removed.DocID)
		c.docs = append(c.docs[:idx], c.docs[idx+1:]...)
		delete(c.docIndex, id)
		for pk, pos := range c.docIndex {
			if pos > idx {
				c.docIndex[pk] = pos - 1
			}
		}
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
		for _, d := range c.docs {
			if fn(d) {
				toDelete = append(toDelete, d.ID)
			}
		}
	}

	for _, id := range toDelete {
		c.deleteDoc(id)
	}

	if c.wal != nil && len(toDelete) > 0 {
		if err := c.wal.AppendDeletes(toDelete); err != nil {
			return status.NewInternalError(fmt.Sprintf("WAL write failed: %v", err))
		}
	}

	return status.OKStatus()
}

func (c *Collection) getDocByPK(pk string) *doc.Doc {
	if c.segManager != nil {
		return c.segManager.GetDoc(pk)
	}
	if d, exists := c.docIndex[pk]; exists && d < len(c.docs) {
		return c.docs[d]
	}
	return nil
}

func (c *Collection) resolveDocIDToPK(docID uint64) string {
	if c.segManager != nil {
		for _, seg := range c.segManager.Segments() {
			if pk, ok := seg.DocIDToPK()[docID]; ok {
				return pk
			}
		}
		return ""
	}
	return c.docIDToPK[docID]
}

func (c *Collection) addToIndexes(d *doc.Doc) {
	for _, field := range c.schema.VectorFields() {
		if idx, ok := c.indexes[field.Name]; ok {
			v, _ := d.Vector(field.Name)
			if v.Float32s != nil {
				idx.Add(v.Float32s, d.ID)
			}
		}
	}
}

func (c *Collection) deleteFromIndexes(pk string) {
	for _, field := range c.schema.VectorFields() {
		if idx, ok := c.indexes[field.Name]; ok {
			idx.Delete(pk)
		}
	}
}

func (c *Collection) addToFTSIndexes(d *doc.Doc) {
	for _, field := range c.schema.FTSFields() {
		if ftsIdx, ok := c.ftsIndexes[field.Name]; ok {
			if fv, ok := d.Field(field.Name); ok && !fv.Null {
				ftsIdx.Index(d.DocID, fv.StringVal)
			}
		}
	}
}

func (c *Collection) deleteFromFTSIndexes(pk string) {
	d := c.getDocByPK(pk)
	if d == nil {
		return
	}
	for _, field := range c.schema.FTSFields() {
		if ftsIdx, ok := c.ftsIndexes[field.Name]; ok {
			ftsIdx.Delete(d.DocID)
		}
	}
}

func (c *Collection) addToInvertIndexes(d *doc.Doc) {
	for _, field := range c.schema.InvertFields() {
		if invIdx, ok := c.invertIndexes[field.Name]; ok {
			if fv, ok := d.Field(field.Name); ok && !fv.Null && fv.StringVal != "" {
				invIdx.Add(d.DocID, fv.StringVal)
			}
		}
	}
}

func (c *Collection) deleteFromInvertIndexes(pk string) {
	d := c.getDocByPK(pk)
	if d == nil {
		return
	}
	for _, field := range c.schema.InvertFields() {
		if invIdx, ok := c.invertIndexes[field.Name]; ok {
			if fv, ok := d.Field(field.Name); ok && !fv.Null && fv.StringVal != "" {
				invIdx.Delete(d.DocID, fv.StringVal)
			}
		}
	}
}

func (c *Collection) allDocs() []*doc.Doc {
	if c.segManager != nil {
		return c.segManager.AllDocs()
	}
	return c.docs
}

func (c *Collection) pkExists(pk string) bool {
	if c.segManager != nil {
		return c.segManager.DocExists(pk)
	}
	_, exists := c.docIndex[pk]
	return exists
}

func (c *Collection) Query(q *query.SearchQuery) ([]map[string]interface{}, status.Status) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []flat.SearchResult
	if q.Target.FTS != nil {
		ftsIdx, ok := c.ftsIndexes[q.Target.FieldName]
		if !ok {
			return nil, status.NewNotFound(
				fmt.Sprintf("no FTS index on field '%s'", q.Target.FieldName))
		}
		ftsResults := ftsIdx.Search(q.Target.FTS.QueryString, q.TopK)
		for _, fr := range ftsResults {
			pk := c.resolveDocIDToPK(fr.DocID)
			results = append(results, flat.SearchResult{
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

	results := make([][]map[string]interface{}, len(queries))
	var wg sync.WaitGroup
	wg.Add(len(queries))

	for i, q := range queries {
		go func(idx int, q *query.SearchQuery) {
			defer wg.Done()
			var searchResults []flat.SearchResult

			if q.Target.FTS != nil {
				ftsIdx, ok := c.ftsIndexes[q.Target.FieldName]
				if !ok {
					return
				}
				ftsResults := ftsIdx.Search(q.Target.FTS.QueryString, q.TopK)
				for _, fr := range ftsResults {
					pk := c.resolveDocIDToPK(fr.DocID)
					searchResults = append(searchResults, flat.SearchResult{
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

	var allResults [][]flat.SearchResult
	for _, sq := range mq.SubQueries {
		idx, ok := c.indexes[sq.Target.FieldName]
		if !ok {
			continue
		}
		topK := sq.NumCandidates
		if topK <= 0 {
			topK = mq.TopK * 2
		}
		var results []flat.SearchResult
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

func (c *Collection) GroupBy(gq *query.GroupByVectorQuery) ([]query.GroupResult, status.Status) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	idx, ok := c.indexes[gq.Target.FieldName]
	if !ok {
		return nil, status.NewNotFound(
			fmt.Sprintf("no index on field '%s'", gq.Target.FieldName))
	}

	docCount := len(c.docs)
	if c.segManager != nil {
		docCount = c.segManager.DocCount()
	}

	var searchResults []flat.SearchResult
	if gq.Filter != "" {
		filterFn := compileFilter(gq.Filter)
		searchResults = idx.SearchWithFilter(
			gq.Target.Vector.QueryVector, docCount,
			func(pk string) bool {
				d := c.getDocByPK(pk)
				if d == nil {
					return false
				}
				return filterFn(d)
			})
	} else {
		searchResults = idx.Search(gq.Target.Vector.QueryVector, docCount)
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
	count := 0
	for key, docs := range groups {
		if count >= groupCount {
			break
		}
		results = append(results, query.GroupResult{
			GroupByValue: key,
			Docs:         docs,
		})
		count++
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

func (c *Collection) CreateIndex(fieldName string, params *param.IndexParams) status.Status {
	c.mu.Lock()
	defer c.mu.Unlock()

	field := c.schema.GetField(fieldName)
	if field == nil {
		return status.NewNotFound(fmt.Sprintf("field '%s' not found", fieldName))
	}

	if params != nil {
		field.IndexParam = params
	}

	if params != nil && params.Type == types.IndexTypeInvert {
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

	c.indexes = make(map[string]Index)
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

	c.ftsIndexes = make(map[string]*fts.FTSIndex)
	for _, field := range c.schema.FTSFields() {
		tokenizer := fts.NewStandardTokenizer()
		ftsIdx := fts.NewFTSIndex(tokenizer)
		c.ftsIndexes[field.Name] = ftsIdx

		for _, d := range allDocs {
			if fv, ok := d.Field(field.Name); ok && !fv.Null {
				ftsIdx.Index(d.DocID, fv.StringVal)
			}
		}
	}

	c.invertIndexes = make(map[string]*invert.InvertIndex)
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

func (c *Collection) Stats() *schema.CollectionStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	docCount := len(c.docs)
	if c.segManager != nil {
		docCount = c.segManager.DocCount()
	}
	return &schema.CollectionStats{
		DocCount: uint64(docCount),
	}
}

func createIndex(field *schema.FieldSchema) (Index, error) {
	if field.IndexParam == nil {
		return flat.NewFlatIndex(field.Dimension, types.MetricTypeCosine), nil
	}

	p := field.IndexParam
	switch p.Type {
	case types.IndexTypeFlat:
		return flat.NewFlatIndex(field.Dimension, p.MetricType), nil
	case types.IndexTypeHNSW:
		m := p.M
		if m <= 0 {
			m = 50
		}
		ef := p.EFConstruction
		if ef <= 0 {
			ef = 500
		}
		return hnsw.NewHNSWIndex(field.Dimension, p.MetricType, m, ef), nil
	case types.IndexTypeIVF:
		nList := p.NList
		if nList <= 0 {
			nList = 10
		}
		nIters := p.NIters
		if nIters <= 0 {
			nIters = 20
		}
		return ivf.NewIVFIndex(field.Dimension, p.MetricType, nList, nIters), nil
	case types.IndexTypeVamana:
		maxDeg := p.VamanaMaxDegree
		if maxDeg <= 0 {
			maxDeg = 64
		}
		searchList := p.VamanaSearchListSize
		if searchList <= 0 {
			searchList = 100
		}
		alpha := p.VamanaAlpha
		if alpha <= 0 {
			alpha = 1.2
		}
		return vamana.NewVamanaIndex(field.Dimension, p.MetricType,
			maxDeg, searchList, alpha, p.SaturateGraph), nil
	case types.IndexTypeDiskAnn:
		maxDeg := p.MaxDegree
		if maxDeg <= 0 {
			maxDeg = 64
		}
		searchList := p.ListSize
		if searchList <= 0 {
			searchList = 100
		}
		alpha := p.Alpha
		if alpha <= 0 {
			alpha = 1.2
		}
		return diskann.NewDiskAnnIndex(field.Dimension, p.MetricType,
			maxDeg, searchList, alpha, false), nil
	case types.IndexTypeHNSWRabitq:
		m := p.M
		if m <= 0 {
			m = 50
		}
		ef := p.EFConstruction
		if ef <= 0 {
			ef = 500
		}
		return hnsw_rabitq.NewHNSWRabitqIndex(field.Dimension, p.MetricType, m, ef), nil
	case types.IndexTypeInvert:
		return nil, errors.New("Invert index not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported index type: %v", p.Type)
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
