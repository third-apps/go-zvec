package segment

import (
	"encoding/json"
	"os"
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
	liveCount   int
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
	return s.liveCount >= s.MaxDocCount
}

func (s *Segment) DocCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.liveCount
}

func (s *Segment) Insert(d *doc.Doc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.docIndex[d.ID]; exists {
		return
	}
	s.docs = append(s.docs, d)
	s.docIndex[d.ID] = len(s.docs) - 1
	s.docIDToPK[d.DocID] = d.ID
	s.liveCount++
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
	removed := s.docs[idx]
	delete(s.docIDToPK, removed.DocID)
	s.docs[idx] = nil
	delete(s.docIndex, pk)
	s.liveCount--
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

func (s *Segment) ResolveDocIDToPK(docID uint64) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pk, ok := s.docIDToPK[docID]
	return pk, ok
}

func (s *Segment) AllDocs() []*doc.Doc {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*doc.Doc, 0, len(s.docs))
	for _, d := range s.docs {
		if d != nil {
			result = append(result, d)
		}
	}
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

func (s *Segment) Save(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var saveDocs []*doc.Doc
	for _, d := range s.docs {
		if d != nil {
			saveDocs = append(saveDocs, d)
		}
	}
	data, err := json.Marshal(struct {
		ID          int
		MaxDocCount int
		Docs        []*doc.Doc
		Closed      bool
	}{
		ID:          s.ID,
		MaxDocCount: s.MaxDocCount,
		Docs:        saveDocs,
		Closed:      s.closed,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadSegment(path string) (*Segment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		ID          int
		MaxDocCount int
		Docs        []*doc.Doc
		Closed      bool
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	s := &Segment{
		ID:          raw.ID,
		MaxDocCount: raw.MaxDocCount,
		docs:        raw.Docs,
		docIndex:    make(map[string]int, len(raw.Docs)),
		docIDToPK:   make(map[uint64]string, len(raw.Docs)),
		liveCount:   len(raw.Docs),
		closed:      raw.Closed,
	}
	for i, d := range raw.Docs {
		s.docIndex[d.ID] = i
		s.docIDToPK[d.DocID] = d.ID
	}
	return s, nil
}
