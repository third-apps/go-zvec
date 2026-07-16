package segment

import (
	"sync"

	"github.com/third-apps/go-zvec/doc"
)

type Segment struct {
	mu          sync.RWMutex
	ID          int
	MaxDocCount int
	docs        []*doc.Doc
	docIndex    map[string]int
	docIDToPK   map[uint64]string
	closed      bool
}

func NewSegment(id int, maxDocCount int) *Segment {
	return &Segment{
		ID:          id,
		MaxDocCount: maxDocCount,
		docs:        make([]*doc.Doc, 0),
		docIndex:    make(map[string]int),
		docIDToPK:   make(map[uint64]string),
	}
}

func (s *Segment) IsFull() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.docs) >= s.MaxDocCount
}

func (s *Segment) DocCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.docs)
}

func (s *Segment) Insert(d *doc.Doc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs = append(s.docs, d)
	s.docIndex[d.ID] = len(s.docs) - 1
	s.docIDToPK[d.DocID] = d.ID
}

func (s *Segment) Update(d *doc.Doc) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, exists := s.docIndex[d.ID]
	if !exists {
		return false
	}
	d.DocID = s.docs[idx].DocID
	s.docs[idx] = d
	return true
}

func (s *Segment) Delete(pk string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx, exists := s.docIndex[pk]
	if !exists {
		return false
	}
	s.docs = append(s.docs[:idx], s.docs[idx+1:]...)
	delete(s.docIndex, pk)
	for pk2, pos := range s.docIndex {
		if pos > idx {
			s.docIndex[pk2] = pos - 1
		}
	}
	return true
}

func (s *Segment) GetDoc(pk string) *doc.Doc {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx, exists := s.docIndex[pk]
	if !exists {
		return nil
	}
	return s.docs[idx]
}

func (s *Segment) HasDoc(pk string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.docIndex[pk]
	return exists
}

func (s *Segment) DocIDToPK() map[uint64]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[uint64]string, len(s.docIDToPK))
	for k, v := range s.docIDToPK {
		result[k] = v
	}
	return result
}

func (s *Segment) AllDocs() []*doc.Doc {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*doc.Doc, len(s.docs))
	copy(result, s.docs)
	return result
}

func (s *Segment) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.docs = nil
	s.docIndex = nil
	s.docIDToPK = nil
}
