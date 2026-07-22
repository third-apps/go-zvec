package metadata

import (
	"sync"
)

type MetadataIndex struct {
	mu     sync.RWMutex
	fields map[string]*fieldIndex
}

type fieldIndex struct {
	stringIdx map[string]map[uint64]struct{}
	int64Idx  map[int64]map[uint64]struct{}
	boolIdx   map[bool]map[uint64]struct{}
}

func NewMetadataIndex() *MetadataIndex {
	return &MetadataIndex{
		fields: make(map[string]*fieldIndex),
	}
}

func (m *MetadataIndex) AddString(fieldName string, docID uint64, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fi := m.getOrCreateField(fieldName)
	if fi.stringIdx == nil {
		fi.stringIdx = make(map[string]map[uint64]struct{})
	}
	set, ok := fi.stringIdx[value]
	if !ok {
		set = make(map[uint64]struct{})
		fi.stringIdx[value] = set
	}
	set[docID] = struct{}{}
}

func (m *MetadataIndex) AddInt64(fieldName string, docID uint64, value int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fi := m.getOrCreateField(fieldName)
	if fi.int64Idx == nil {
		fi.int64Idx = make(map[int64]map[uint64]struct{})
	}
	set, ok := fi.int64Idx[value]
	if !ok {
		set = make(map[uint64]struct{})
		fi.int64Idx[value] = set
	}
	set[docID] = struct{}{}
}

func (m *MetadataIndex) AddBool(fieldName string, docID uint64, value bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fi := m.getOrCreateField(fieldName)
	if fi.boolIdx == nil {
		fi.boolIdx = make(map[bool]map[uint64]struct{})
	}
	set, ok := fi.boolIdx[value]
	if !ok {
		set = make(map[uint64]struct{})
		fi.boolIdx[value] = set
	}
	set[docID] = struct{}{}
}

func (m *MetadataIndex) DeleteDoc(docID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, fi := range m.fields {
		if fi.stringIdx != nil {
			for val, set := range fi.stringIdx {
				delete(set, docID)
				if len(set) == 0 {
					delete(fi.stringIdx, val)
				}
			}
			if len(fi.stringIdx) == 0 {
				fi.stringIdx = nil
			}
		}
		if fi.int64Idx != nil {
			for val, set := range fi.int64Idx {
				delete(set, docID)
				if len(set) == 0 {
					delete(fi.int64Idx, val)
				}
			}
			if len(fi.int64Idx) == 0 {
				fi.int64Idx = nil
			}
		}
		if fi.boolIdx != nil {
			for val, set := range fi.boolIdx {
				delete(set, docID)
				if len(set) == 0 {
					delete(fi.boolIdx, val)
				}
			}
			if len(fi.boolIdx) == 0 {
				fi.boolIdx = nil
			}
		}
		if fi.stringIdx == nil && fi.int64Idx == nil && fi.boolIdx == nil {
			delete(m.fields, name)
		}
	}
}

func (m *MetadataIndex) MatchString(fieldName, value string) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok {
		return nil
	}
	if fi.stringIdx == nil {
		return []uint64{}
	}
	set, ok := fi.stringIdx[value]
	if !ok {
		return []uint64{}
	}
	result := make([]uint64, 0, len(set))
	for docID := range set {
		result = append(result, docID)
	}
	return result
}

func (m *MetadataIndex) MatchInt64(fieldName string, value int64) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok {
		return nil
	}
	if fi.int64Idx == nil {
		return []uint64{}
	}
	set, ok := fi.int64Idx[value]
	if !ok {
		return []uint64{}
	}
	result := make([]uint64, 0, len(set))
	for docID := range set {
		result = append(result, docID)
	}
	return result
}

func (m *MetadataIndex) MatchBool(fieldName string, value bool) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok {
		return nil
	}
	if fi.boolIdx == nil {
		return []uint64{}
	}
	set, ok := fi.boolIdx[value]
	if !ok {
		return []uint64{}
	}
	result := make([]uint64, 0, len(set))
	for docID := range set {
		result = append(result, docID)
	}
	return result
}

func (m *MetadataIndex) MatchStrings(fieldName string, values []string) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok {
		return nil
	}
	if fi.stringIdx == nil {
		return []uint64{}
	}
	seen := make(map[uint64]struct{})
	for _, value := range values {
		set, ok := fi.stringIdx[value]
		if !ok {
			continue
		}
		for docID := range set {
			seen[docID] = struct{}{}
		}
	}
	result := make([]uint64, 0, len(seen))
	for docID := range seen {
		result = append(result, docID)
	}
	return result
}

func (m *MetadataIndex) HasField(fieldName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.fields[fieldName]
	return ok
}

func (m *MetadataIndex) FieldValues(fieldName string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok || fi.stringIdx == nil {
		return nil
	}
	values := make([]string, 0, len(fi.stringIdx))
	for v := range fi.stringIdx {
		values = append(values, v)
	}
	return values
}

func (m *MetadataIndex) DocCount(fieldName, value string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok || fi.stringIdx == nil {
		return 0
	}
	set, ok := fi.stringIdx[value]
	if !ok {
		return 0
	}
	return len(set)
}

func (m *MetadataIndex) MatchInt64Gt(fieldName string, value int64) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok || fi.int64Idx == nil {
		return nil
	}
	var result []uint64
	for k, set := range fi.int64Idx {
		if k > value {
			for docID := range set {
				result = append(result, docID)
			}
		}
	}
	return result
}

func (m *MetadataIndex) MatchInt64Lt(fieldName string, value int64) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok || fi.int64Idx == nil {
		return nil
	}
	var result []uint64
	for k, set := range fi.int64Idx {
		if k < value {
			for docID := range set {
				result = append(result, docID)
			}
		}
	}
	return result
}

func (m *MetadataIndex) MatchInt64Gte(fieldName string, value int64) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok || fi.int64Idx == nil {
		return nil
	}
	var result []uint64
	for k, set := range fi.int64Idx {
		if k >= value {
			for docID := range set {
				result = append(result, docID)
			}
		}
	}
	return result
}

func (m *MetadataIndex) MatchInt64Lte(fieldName string, value int64) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok || fi.int64Idx == nil {
		return nil
	}
	var result []uint64
	for k, set := range fi.int64Idx {
		if k <= value {
			for docID := range set {
				result = append(result, docID)
			}
		}
	}
	return result
}

func (m *MetadataIndex) MatchInt64Ne(fieldName string, value int64) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok || fi.int64Idx == nil {
		return nil
	}
	var result []uint64
	for k, set := range fi.int64Idx {
		if k != value {
			for docID := range set {
				result = append(result, docID)
			}
		}
	}
	return result
}

func (m *MetadataIndex) MatchExists(fieldName string) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fi, ok := m.fields[fieldName]
	if !ok {
		return nil
	}
	seen := make(map[uint64]struct{})
	if fi.stringIdx != nil {
		for _, set := range fi.stringIdx {
			for docID := range set {
				seen[docID] = struct{}{}
			}
		}
	}
	if fi.int64Idx != nil {
		for _, set := range fi.int64Idx {
			for docID := range set {
				seen[docID] = struct{}{}
			}
		}
	}
	if fi.boolIdx != nil {
		for _, set := range fi.boolIdx {
			for docID := range set {
				seen[docID] = struct{}{}
			}
		}
	}
	result := make([]uint64, 0, len(seen))
	for docID := range seen {
		result = append(result, docID)
	}
	return result
}

func (m *MetadataIndex) getOrCreateField(fieldName string) *fieldIndex {
	fi, ok := m.fields[fieldName]
	if !ok {
		fi = &fieldIndex{}
		m.fields[fieldName] = fi
	}
	return fi
}
