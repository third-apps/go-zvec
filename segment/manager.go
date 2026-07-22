package segment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/third-apps/go-zvec/doc"
)

type Option struct {
	MaxSegmentSize int
}

type Manager struct {
	mu         sync.RWMutex
	segments   []*Segment
	docSegment map[string]*Segment
	maxSegSize int
	nextSegID  int
}

func NewManager(opt Option) *Manager {
	if opt.MaxSegmentSize <= 0 {
		opt.MaxSegmentSize = 10000
	}
	m := &Manager{
		segments:   make([]*Segment, 0),
		docSegment: make(map[string]*Segment),
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
	m.docSegment[d.ID] = active
	m.mu.Unlock()
}

func (m *Manager) Upsert(d *doc.Doc) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if seg, ok := m.docSegment[d.ID]; ok {
		seg.Update(d)
		return true
	}
	active := m.segments[len(m.segments)-1]
	if active.IsFull() {
		active = m.rotate()
	}
	active.Insert(d)
	m.docSegment[d.ID] = active
	return true
}

func (m *Manager) Delete(pk string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	seg, ok := m.docSegment[pk]
	if !ok {
		return false
	}
	delete(m.docSegment, pk)
	return seg.Delete(pk)
}

func (m *Manager) GetDoc(pk string) *doc.Doc {
	m.mu.RLock()
	seg, ok := m.docSegment[pk]
	if !ok {
		m.mu.RUnlock()
		return nil
	}
	d := seg.GetDoc(pk)
	m.mu.RUnlock()
	return d
}

func (m *Manager) DocExists(pk string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.docSegment[pk]
	return ok
}

func (m *Manager) AllDocPKs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var pks []string
	seen := make(map[string]bool)
	for _, seg := range m.segments {
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
	defer m.mu.RUnlock()
	var docs []*doc.Doc
	seen := make(map[string]bool)
	for _, seg := range m.segments {
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

func (m *Manager) ResolveDocIDToPK(docID uint64) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, seg := range m.segments {
		if pk, ok := seg.ResolveDocIDToPK(docID); ok {
			return pk
		}
	}
	return ""
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, seg := range m.segments {
		seg.Close()
	}
	m.segments = nil
	m.docSegment = nil
}

func (m *Manager) Save(dir string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	metaPath := filepath.Join(dir, "segments.json")
	meta := struct {
		MaxSegSize int   `json:"max_seg_size"`
		NextSegID  int   `json:"next_seg_id"`
		SegIDs     []int `json:"seg_ids"`
	}{
		MaxSegSize: m.maxSegSize,
		NextSegID:  m.nextSegID,
	}
	for _, seg := range m.segments {
		meta.SegIDs = append(meta.SegIDs, seg.ID)
	}
	metaData, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return err
	}

	for _, seg := range m.segments {
		segPath := filepath.Join(dir, fmt.Sprintf("seg_%d.json", seg.ID))
		if err := seg.Save(segPath); err != nil {
			return fmt.Errorf("failed to save segment %d: %w", seg.ID, err)
		}
	}

	return nil
}

func LoadManager(dir string) (*Manager, error) {
	metaPath := filepath.Join(dir, "segments.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta struct {
		MaxSegSize int   `json:"max_seg_size"`
		NextSegID  int   `json:"next_seg_id"`
		SegIDs     []int `json:"seg_ids"`
	}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, err
	}

	m := &Manager{
		segments:   make([]*Segment, 0, len(meta.SegIDs)),
		docSegment: make(map[string]*Segment),
		maxSegSize: meta.MaxSegSize,
		nextSegID:  meta.NextSegID,
	}

	for _, segID := range meta.SegIDs {
		segPath := filepath.Join(dir, fmt.Sprintf("seg_%d.json", segID))
		seg, err := LoadSegment(segPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load segment %d: %w", segID, err)
		}
		m.segments = append(m.segments, seg)
		for _, d := range seg.AllDocs() {
			m.docSegment[d.ID] = seg
		}
	}

	if len(m.segments) == 0 {
		m.rotate()
	}

	return m, nil
}
