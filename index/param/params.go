package param

import "github.com/third-apps/go-zvec/types"

type IndexParams struct {
	Type types.IndexType
	// Common
	MetricType   types.MetricType
	QuantizeType types.QuantizeType
	EnableRotate bool
	// HNSW
	M                   int
	EFConstruction      int
	UseContiguousMemory bool
	// IVF
	NList   int
	NIters  int
	UseSOAR bool
	// DiskANN
	MaxDegree  int
	ListSize   int
	PQChunkNum int
	Alpha      float64
	// Vamana
	VamanaMaxDegree      int
	VamanaSearchListSize int
	VamanaAlpha          float32
	SaturateGraph        bool
	// HNSW RaBitQ
	TotalBits   int
	NumClusters int
	SampleCount int
	// FTS
	FtsTokenizer   string
	FtsFilters     []string
	FtsExtraParams string
	// Invert
	EnableRangeOptimization bool
	EnableExtendedWildcard  bool
	// Flat
	MajorOrder string
}

func NewHNSWIndexParams(metric types.MetricType, m, efConstruction int) *IndexParams {
	return &IndexParams{
		Type: types.IndexTypeHNSW, MetricType: metric,
		M: m, EFConstruction: efConstruction,
	}
}

func NewHNSWIndexParamsFull(metric types.MetricType, m, efConstruction int,
	quantize types.QuantizeType, useContiguousMemory bool) *IndexParams {
	return &IndexParams{
		Type: types.IndexTypeHNSW, MetricType: metric,
		M: m, EFConstruction: efConstruction,
		QuantizeType: quantize, UseContiguousMemory: useContiguousMemory,
	}
}

func NewHNSWRabitqIndexParams(metric types.MetricType, totalBits, numClusters,
	m, efConstruction, sampleCount int) *IndexParams {
	return &IndexParams{
		Type: types.IndexTypeHNSWRabitq, MetricType: metric,
		M: m, EFConstruction: efConstruction,
		TotalBits: totalBits, NumClusters: numClusters, SampleCount: sampleCount,
	}
}

func NewIVFIndexParams(metric types.MetricType, nList, nIters int, useSOAR bool) *IndexParams {
	return &IndexParams{
		Type: types.IndexTypeIVF, MetricType: metric,
		NList: nList, NIters: nIters, UseSOAR: useSOAR,
	}
}

func NewFlatIndexParams(metric types.MetricType) *IndexParams {
	return &IndexParams{
		Type: types.IndexTypeFlat, MetricType: metric,
	}
}

func NewFlatIndexParamsFull(metric types.MetricType, quantize types.QuantizeType) *IndexParams {
	return &IndexParams{
		Type: types.IndexTypeFlat, MetricType: metric,
		QuantizeType: quantize,
	}
}

func NewDiskAnnIndexParams(metric types.MetricType, maxDegree, listSize, pqChunkNum int) *IndexParams {
	return &IndexParams{
		Type: types.IndexTypeDiskAnn, MetricType: metric,
		MaxDegree: maxDegree, ListSize: listSize, PQChunkNum: pqChunkNum,
	}
}

func NewDiskAnnIndexParamsFull(metric types.MetricType, maxDegree, listSize, pqChunkNum int, alpha float64) *IndexParams {
	return &IndexParams{
		Type: types.IndexTypeDiskAnn, MetricType: metric,
		MaxDegree: maxDegree, ListSize: listSize, PQChunkNum: pqChunkNum,
		Alpha: alpha,
	}
}

func NewVamanaIndexParams(metric types.MetricType, maxDegree, searchListSize int,
	alpha float32, saturateGraph, useContiguousMemory bool) *IndexParams {
	return &IndexParams{
		Type: types.IndexTypeVamana, MetricType: metric,
		VamanaMaxDegree: maxDegree, VamanaSearchListSize: searchListSize,
		VamanaAlpha: alpha, SaturateGraph: saturateGraph,
		UseContiguousMemory: useContiguousMemory,
	}
}

func NewInvertIndexParams(enableRangeOpt, enableWildcard bool) *IndexParams {
	return &IndexParams{
		Type:                    types.IndexTypeInvert,
		EnableRangeOptimization: enableRangeOpt,
		EnableExtendedWildcard:  enableWildcard,
	}
}

func NewFTSIndexParams(tokenizer string, filters []string, extraParams string) *IndexParams {
	return &IndexParams{
		Type:         types.IndexTypeFTS,
		FtsTokenizer: tokenizer, FtsFilters: filters,
		FtsExtraParams: extraParams,
	}
}
