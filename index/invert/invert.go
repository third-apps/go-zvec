package invert

import "sync"

type InvertIndex struct {
	mu        sync.RWMutex
	index     map[string]map[uint64]struct{}
	docValues map[uint64][]string
}

func NewInvertIndex() *InvertIndex {
	return &InvertIndex{
		index:     make(map[string]map[uint64]struct{}),
		docValues: make(map[uint64][]string),
	}
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
	if _, exists := set[docID]; !exists {
		set[docID] = struct{}{}
		idx.docValues[docID] = append(idx.docValues[docID], value)
	}
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
	values, ok := idx.docValues[docID]
	if !ok {
		return
	}
	for _, value := range values {
		set, ok := idx.index[value]
		if ok {
			delete(set, docID)
			if len(set) == 0 {
				delete(idx.index, value)
			}
		}
	}
	delete(idx.docValues, docID)
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
