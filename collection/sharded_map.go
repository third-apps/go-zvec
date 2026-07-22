package collection

import (
	"hash/fnv"
	"sync"

	"github.com/third-apps/go-zvec/doc"
)

const shardCount = 16

type stringIntShard struct {
	mu   sync.RWMutex
	data map[string]int
}

type shardedDocIndex struct {
	shards [shardCount]*stringIntShard
}

func newShardedDocIndex() *shardedDocIndex {
	si := &shardedDocIndex{}
	for i := 0; i < shardCount; i++ {
		si.shards[i] = &stringIntShard{data: make(map[string]int)}
	}
	return si
}

func (s *shardedDocIndex) getShard(key string) *stringIntShard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return s.shards[h.Sum32()%shardCount]
}

func (s *shardedDocIndex) Get(key string) (int, bool) {
	sh := s.getShard(key)
	sh.mu.RLock()
	v, ok := sh.data[key]
	sh.mu.RUnlock()
	return v, ok
}

func (s *shardedDocIndex) Set(key string, val int) {
	sh := s.getShard(key)
	sh.mu.Lock()
	sh.data[key] = val
	sh.mu.Unlock()
}

func (s *shardedDocIndex) Delete(key string) {
	sh := s.getShard(key)
	sh.mu.Lock()
	delete(sh.data, key)
	sh.mu.Unlock()
}

func (s *shardedDocIndex) Len() int {
	total := 0
	for i := 0; i < shardCount; i++ {
		s.shards[i].mu.RLock()
		total += len(s.shards[i].data)
		s.shards[i].mu.RUnlock()
	}
	return total
}

type uint64StringShard struct {
	mu   sync.RWMutex
	data map[uint64]string
}

type shardedDocIDToPK struct {
	shards [shardCount]*uint64StringShard
}

func newShardedDocIDToPK() *shardedDocIDToPK {
	si := &shardedDocIDToPK{}
	for i := 0; i < shardCount; i++ {
		si.shards[i] = &uint64StringShard{data: make(map[uint64]string)}
	}
	return si
}

func (s *shardedDocIDToPK) getShard(key uint64) *uint64StringShard {
	return s.shards[key%shardCount]
}

func (s *shardedDocIDToPK) Get(key uint64) (string, bool) {
	sh := s.getShard(key)
	sh.mu.RLock()
	v, ok := sh.data[key]
	sh.mu.RUnlock()
	return v, ok
}

func (s *shardedDocIDToPK) Set(key uint64, val string) {
	sh := s.getShard(key)
	sh.mu.Lock()
	sh.data[key] = val
	sh.mu.Unlock()
}

func (s *shardedDocIDToPK) Delete(key uint64) {
	sh := s.getShard(key)
	sh.mu.Lock()
	delete(sh.data, key)
	sh.mu.Unlock()
}

type docShard struct {
	mu   sync.RWMutex
	docs []*doc.Doc
}

type shardedDocs struct {
	shards [shardCount]*docShard
}

func newShardedDocs() *shardedDocs {
	sd := &shardedDocs{}
	for i := 0; i < shardCount; i++ {
		sd.shards[i] = &docShard{docs: make([]*doc.Doc, 0)}
	}
	return sd
}

func (s *shardedDocs) getShard(docID uint64) *docShard {
	return s.shards[docID%shardCount]
}

func (s *shardedDocs) Append(docID uint64, d *doc.Doc) {
	sh := s.getShard(docID)
	sh.mu.Lock()
	sh.docs = append(sh.docs, d)
	sh.mu.Unlock()
}

func (s *shardedDocs) Get(docID uint64) (*doc.Doc, bool) {
	sh := s.getShard(docID)
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	for _, d := range sh.docs {
		if d.DocID == docID {
			return d, true
		}
	}
	return nil, false
}

func (s *shardedDocs) Set(docID uint64, d *doc.Doc) {
	sh := s.getShard(docID)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	for i, existing := range sh.docs {
		if existing.DocID == docID {
			sh.docs[i] = d
			return
		}
	}
	sh.docs = append(sh.docs, d)
}

func (s *shardedDocs) Delete(docID uint64) bool {
	sh := s.getShard(docID)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	for i, d := range sh.docs {
		if d.DocID == docID {
			sh.docs = append(sh.docs[:i], sh.docs[i+1:]...)
			return true
		}
	}
	return false
}

func (s *shardedDocs) Len() int {
	total := 0
	for i := 0; i < shardCount; i++ {
		s.shards[i].mu.RLock()
		total += len(s.shards[i].docs)
		s.shards[i].mu.RUnlock()
	}
	return total
}

func (s *shardedDocs) ForEach(fn func(d *doc.Doc) bool) {
	for i := 0; i < shardCount; i++ {
		s.shards[i].mu.RLock()
		for _, d := range s.shards[i].docs {
			if !fn(d) {
				s.shards[i].mu.RUnlock()
				return
			}
		}
		s.shards[i].mu.RUnlock()
	}
}

func (s *shardedDocs) AllDocs() []*doc.Doc {
	var all []*doc.Doc
	for i := 0; i < shardCount; i++ {
		s.shards[i].mu.RLock()
		all = append(all, s.shards[i].docs...)
		s.shards[i].mu.RUnlock()
	}
	return all
}
