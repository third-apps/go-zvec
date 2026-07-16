package invert

import "sync"

type InvertIndex struct {
	mu    sync.RWMutex
	index map[string]map[uint64]struct{}
}

func NewInvertIndex() *InvertIndex {
	return &InvertIndex{index: make(map[string]map[uint64]struct{})}
}

func (idx *InvertIndex) Add(docID uint64, value string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if value == "" {
		return
	}
	set, ok := idx.index[value]
	if !ok {
		set = make(map[uint64]struct{})
		idx.index[value] = set
	}
	set[docID] = struct{}{}
}

func (idx *InvertIndex) Search(value string) []uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	set, ok := idx.index[value]
	if !ok {
		return nil
	}
	result := make([]uint64, 0, len(set))
	for docID := range set {
		result = append(result, docID)
	}
	return result
}

func (idx *InvertIndex) Delete(docID uint64, value string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if value == "" {
		return
	}
	set, ok := idx.index[value]
	if ok {
		delete(set, docID)
		if len(set) == 0 {
			delete(idx.index, value)
		}
	}
}

func (idx *InvertIndex) DeleteDoc(docID uint64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for value, set := range idx.index {
		delete(set, docID)
		if len(set) == 0 {
			delete(idx.index, value)
		}
	}
}

func (idx *InvertIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.index)
}

func (idx *InvertIndex) SearchWithFilter(value string, filterFn func(uint64) bool) []uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	set, ok := idx.index[value]
	if !ok {
		return nil
	}
	result := make([]uint64, 0, len(set))
	for docID := range set {
		if filterFn == nil || filterFn(docID) {
			result = append(result, docID)
		}
	}
	return result
}

func (idx *InvertIndex) GetDocIDs(value string) []uint64 {
	return idx.Search(value)
}

func (idx *InvertIndex) HasValue(value string) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	_, ok := idx.index[value]
	return ok
}
