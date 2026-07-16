package segment

import (
	"sync"

	"github.com/third-apps/go-zvec/doc"
)

type Option struct {
	MaxSegmentSize int
}

type Manager struct {
	mu         sync.RWMutex
	segments   []*Segment
	maxSegSize int
	nextSegID  int
}

func NewManager(opt Option) *Manager {
	if opt.MaxSegmentSize <= 0 {
		opt.MaxSegmentSize = 10000
	}
	m := &Manager{
		segments:   make([]*Segment, 0),
		maxSegSize: opt.MaxSegmentSize,
	}
	m.rotate()
	return m
}

func (m *Manager) rotate() *Segment {
	seg := NewSegment(m.nextSegID, m.maxSegSize)
	m.nextSegID++
	m.segments = append(m.segments, seg)
	return seg
}

func (m *Manager) Insert(d *doc.Doc) {
	m.mu.Lock()
	active := m.segments[len(m.segments)-1]
	if active.IsFull() {
		active = m.rotate()
	}
	active.Insert(d)
	m.mu.Unlock()
}

func (m *Manager) Upsert(d *doc.Doc) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, seg := range m.segments {
		if seg.HasDoc(d.ID) {
			seg.Update(d)
			return true
		}
	}
	active := m.segments[len(m.segments)-1]
	if active.IsFull() {
		active = m.rotate()
	}
	active.Insert(d)
	return true
}

func (m *Manager) Delete(pk string) bool {
	m.mu.RLock()
	segs := m.segments
	m.mu.RUnlock()
	for _, seg := range segs {
		if seg.Delete(pk) {
			return true
		}
	}
	return false
}

func (m *Manager) GetDoc(pk string) *doc.Doc {
	m.mu.RLock()
	segs := m.segments
	m.mu.RUnlock()
	for _, seg := range segs {
		if d := seg.GetDoc(pk); d != nil {
			return d
		}
	}
	return nil
}

func (m *Manager) DocExists(pk string) bool {
	m.mu.RLock()
	segs := m.segments
	m.mu.RUnlock()
	for _, seg := range segs {
		if seg.HasDoc(pk) {
			return true
		}
	}
	return false
}

func (m *Manager) AllDocPKs() []string {
	m.mu.RLock()
	segs := m.segments
	m.mu.RUnlock()
	var pks []string
	seen := make(map[string]bool)
	for _, seg := range segs {
		for _, d := range seg.AllDocs() {
			if !seen[d.ID] {
				seen[d.ID] = true
				pks = append(pks, d.ID)
			}
		}
	}
	return pks
}

func (m *Manager) AllDocs() []*doc.Doc {
	m.mu.RLock()
	segs := m.segments
	m.mu.RUnlock()
	var docs []*doc.Doc
	seen := make(map[string]bool)
	for _, seg := range segs {
		for _, d := range seg.AllDocs() {
			if !seen[d.ID] {
				seen[d.ID] = true
				docs = append(docs, d)
			}
		}
	}
	return docs
}

func (m *Manager) DocCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := 0
	for _, seg := range m.segments {
		total += seg.DocCount()
	}
	return total
}

func (m *Manager) SegmentCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.segments)
}

func (m *Manager) Segments() []*Segment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Segment, len(m.segments))
	copy(result, m.segments)
	return result
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, seg := range m.segments {
		seg.Close()
	}
	m.segments = nil
}
