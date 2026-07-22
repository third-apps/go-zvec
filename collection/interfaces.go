package collection

import (
	"github.com/third-apps/go-zvec/doc"
	"github.com/third-apps/go-zvec/fts"
	"github.com/third-apps/go-zvec/wal"
)

type FTSIndexer interface {
	Index(docID uint64, text string)
	Delete(docID uint64)
	Search(query string, topK int) []fts.SearchResult
}

type InvertIndexer interface {
	Add(docID uint64, value string)
	Delete(docID uint64, value string)
}

type MetaIndexer interface {
	AddString(fieldName string, docID uint64, value string)
	AddInt64(fieldName string, docID uint64, value int64)
	AddBool(fieldName string, docID uint64, value bool)
	DeleteDoc(docID uint64)
	MatchString(fieldName string, value string) []uint64
	MatchInt64(fieldName string, value int64) []uint64
	MatchBool(fieldName string, value bool) []uint64
	MatchStrings(fieldName string, values []string) []uint64
	MatchInt64Ne(fieldName string, value int64) []uint64
	MatchInt64Gt(fieldName string, value int64) []uint64
	MatchInt64Lt(fieldName string, value int64) []uint64
	MatchInt64Gte(fieldName string, value int64) []uint64
	MatchInt64Lte(fieldName string, value int64) []uint64
	MatchExists(fieldName string) []uint64
}

type WALWriter interface {
	Close() error
	Sync() error
	Truncate() error
	Replay(fn wal.ReplayFunc) error
	AppendInsert(id string, doc *doc.Doc) error
	AppendInserts(docs []*doc.Doc) error
	AppendUpsert(id string, doc *doc.Doc) error
	AppendUpserts(docs []*doc.Doc) error
	AppendUpdate(id string, doc *doc.Doc) error
	AppendUpdates(docs []*doc.Doc) error
	AppendDeletes(ids []string) error
}

type SegmentManager interface {
	Insert(d *doc.Doc)
	Upsert(d *doc.Doc) bool
	GetDoc(pk string) *doc.Doc
	Delete(pk string) bool
	Close()
	AllDocs() []*doc.Doc
	DocExists(pk string) bool
	DocCount() int
	ResolveDocIDToPK(docID uint64) string
}
