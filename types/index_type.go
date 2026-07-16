package types

type IndexType uint32

const (
	IndexTypeUndefined  IndexType = 0
	IndexTypeHNSW       IndexType = 1
	IndexTypeIVF        IndexType = 2
	IndexTypeFlat       IndexType = 3
	IndexTypeHNSWRabitq IndexType = 4
	IndexTypeDiskAnn    IndexType = 5
	IndexTypeVamana     IndexType = 6
	IndexTypeInvert     IndexType = 10
	IndexTypeFTS        IndexType = 11
)

func (t IndexType) String() string {
	switch t {
	case IndexTypeUndefined:
		return "undefined"
	case IndexTypeHNSW:
		return "hnsw"
	case IndexTypeIVF:
		return "ivf"
	case IndexTypeFlat:
		return "flat"
	case IndexTypeHNSWRabitq:
		return "hnsw_rabitq"
	case IndexTypeDiskAnn:
		return "diskann"
	case IndexTypeVamana:
		return "vamana"
	case IndexTypeInvert:
		return "invert"
	case IndexTypeFTS:
		return "fts"
	default:
		return "unknown"
	}
}

func (t IndexType) IsVectorIndex() bool {
	switch t {
	case IndexTypeHNSW, IndexTypeIVF, IndexTypeFlat,
		IndexTypeHNSWRabitq, IndexTypeDiskAnn, IndexTypeVamana:
		return true
	}
	return false
}
