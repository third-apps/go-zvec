package param

import "github.com/third-apps/go-zvec/types"

type IndexConfig interface {
	GetIndexType() types.IndexType
	GetMetricType() types.MetricType
}

type CommonParams struct {
	MetricType   types.MetricType
	QuantizeType types.QuantizeType
	EnableRotate bool
}

type HNSWParams struct {
	CommonParams
	M                   int
	EFConstruction      int
	UseContiguousMemory bool
}

func (p *HNSWParams) GetIndexType() types.IndexType   { return types.IndexTypeHNSW }
func (p *HNSWParams) GetMetricType() types.MetricType { return p.MetricType }

func NewHNSWParams(metric types.MetricType, m, efConstruction int) *HNSWParams {
	return &HNSWParams{
		CommonParams:   CommonParams{MetricType: metric},
		M:              m,
		EFConstruction: efConstruction,
	}
}

func NewHNSWParamsFull(metric types.MetricType, m, efConstruction int,
	quantize types.QuantizeType, useContiguousMemory bool) *HNSWParams {
	return &HNSWParams{
		CommonParams:        CommonParams{MetricType: metric, QuantizeType: quantize},
		M:                   m,
		EFConstruction:      efConstruction,
		UseContiguousMemory: useContiguousMemory,
	}
}

type IVFParams struct {
	CommonParams
	NList   int
	NIters  int
	UseSOAR bool
}

func (p *IVFParams) GetIndexType() types.IndexType   { return types.IndexTypeIVF }
func (p *IVFParams) GetMetricType() types.MetricType { return p.MetricType }

func NewIVFParams(metric types.MetricType, nList, nIters int, useSOAR bool) *IVFParams {
	return &IVFParams{
		CommonParams: CommonParams{MetricType: metric},
		NList:        nList,
		NIters:       nIters,
		UseSOAR:      useSOAR,
	}
}

type FlatParams struct {
	CommonParams
	MajorOrder string
}

func (p *FlatParams) GetIndexType() types.IndexType   { return types.IndexTypeFlat }
func (p *FlatParams) GetMetricType() types.MetricType { return p.MetricType }

func NewFlatParams(metric types.MetricType) *FlatParams {
	return &FlatParams{
		CommonParams: CommonParams{MetricType: metric},
	}
}

func NewFlatParamsFull(metric types.MetricType, quantize types.QuantizeType) *FlatParams {
	return &FlatParams{
		CommonParams: CommonParams{MetricType: metric, QuantizeType: quantize},
	}
}

type DiskAnnParams struct {
	CommonParams
	MaxDegree  int
	ListSize   int
	PQChunkNum int
	Alpha      float64
}

func (p *DiskAnnParams) GetIndexType() types.IndexType   { return types.IndexTypeDiskAnn }
func (p *DiskAnnParams) GetMetricType() types.MetricType { return p.MetricType }

func NewDiskAnnParams(metric types.MetricType, maxDegree, listSize, pqChunkNum int) *DiskAnnParams {
	return &DiskAnnParams{
		CommonParams: CommonParams{MetricType: metric},
		MaxDegree:    maxDegree,
		ListSize:     listSize,
		PQChunkNum:   pqChunkNum,
	}
}

func NewDiskAnnParamsFull(metric types.MetricType, maxDegree, listSize, pqChunkNum int, alpha float64) *DiskAnnParams {
	return &DiskAnnParams{
		CommonParams: CommonParams{MetricType: metric},
		MaxDegree:    maxDegree,
		ListSize:     listSize,
		PQChunkNum:   pqChunkNum,
		Alpha:        alpha,
	}
}

type VamanaParams struct {
	CommonParams
	MaxDegree           int
	SearchListSize      int
	Alpha               float32
	SaturateGraph       bool
	UseContiguousMemory bool
}

func (p *VamanaParams) GetIndexType() types.IndexType   { return types.IndexTypeVamana }
func (p *VamanaParams) GetMetricType() types.MetricType { return p.MetricType }

func NewVamanaParams(metric types.MetricType, maxDegree, searchListSize int,
	alpha float32, saturateGraph, useContiguousMemory bool) *VamanaParams {
	return &VamanaParams{
		CommonParams:        CommonParams{MetricType: metric},
		MaxDegree:           maxDegree,
		SearchListSize:      searchListSize,
		Alpha:               alpha,
		SaturateGraph:       saturateGraph,
		UseContiguousMemory: useContiguousMemory,
	}
}

type HNSWRabitqParams struct {
	CommonParams
	M              int
	EFConstruction int
	TotalBits      int
	NumClusters    int
	SampleCount    int
}

func (p *HNSWRabitqParams) GetIndexType() types.IndexType   { return types.IndexTypeHNSWRabitq }
func (p *HNSWRabitqParams) GetMetricType() types.MetricType { return p.MetricType }

func NewHNSWRabitqParams(metric types.MetricType, totalBits, numClusters,
	m, efConstruction, sampleCount int) *HNSWRabitqParams {
	return &HNSWRabitqParams{
		CommonParams:   CommonParams{MetricType: metric},
		M:              m,
		EFConstruction: efConstruction,
		TotalBits:      totalBits,
		NumClusters:    numClusters,
		SampleCount:    sampleCount,
	}
}

type FTSParams struct {
	Tokenizer   string
	Filters     []string
	ExtraParams string
}

func (p *FTSParams) GetIndexType() types.IndexType   { return types.IndexTypeFTS }
func (p *FTSParams) GetMetricType() types.MetricType { return types.MetricTypeUndefined }

func NewFTSParams(tokenizer string, filters []string, extraParams string) *FTSParams {
	return &FTSParams{
		Tokenizer:   tokenizer,
		Filters:     filters,
		ExtraParams: extraParams,
	}
}

type InvertParams struct {
	EnableRangeOptimization bool
	EnableExtendedWildcard  bool
}

func (p *InvertParams) GetIndexType() types.IndexType   { return types.IndexTypeInvert }
func (p *InvertParams) GetMetricType() types.MetricType { return types.MetricTypeUndefined }

func NewInvertParams(enableRangeOpt, enableWildcard bool) *InvertParams {
	return &InvertParams{
		EnableRangeOptimization: enableRangeOpt,
		EnableExtendedWildcard:  enableWildcard,
	}
}

func IndexConfigFromLegacy(p *IndexParams) IndexConfig {
	if p == nil {
		return nil
	}
	switch p.Type {
	case types.IndexTypeHNSW:
		return &HNSWParams{
			CommonParams:        CommonParams{MetricType: p.MetricType, QuantizeType: p.QuantizeType, EnableRotate: p.EnableRotate},
			M:                   p.M,
			EFConstruction:      p.EFConstruction,
			UseContiguousMemory: p.UseContiguousMemory,
		}
	case types.IndexTypeIVF:
		return &IVFParams{
			CommonParams: CommonParams{MetricType: p.MetricType, QuantizeType: p.QuantizeType, EnableRotate: p.EnableRotate},
			NList:        p.NList,
			NIters:       p.NIters,
			UseSOAR:      p.UseSOAR,
		}
	case types.IndexTypeFlat:
		return &FlatParams{
			CommonParams: CommonParams{MetricType: p.MetricType, QuantizeType: p.QuantizeType, EnableRotate: p.EnableRotate},
			MajorOrder:   p.MajorOrder,
		}
	case types.IndexTypeDiskAnn:
		return &DiskAnnParams{
			CommonParams: CommonParams{MetricType: p.MetricType, QuantizeType: p.QuantizeType, EnableRotate: p.EnableRotate},
			MaxDegree:    p.MaxDegree,
			ListSize:     p.ListSize,
			PQChunkNum:   p.PQChunkNum,
			Alpha:        p.Alpha,
		}
	case types.IndexTypeVamana:
		return &VamanaParams{
			CommonParams:        CommonParams{MetricType: p.MetricType, QuantizeType: p.QuantizeType, EnableRotate: p.EnableRotate},
			MaxDegree:           p.VamanaMaxDegree,
			SearchListSize:      p.VamanaSearchListSize,
			Alpha:               p.VamanaAlpha,
			SaturateGraph:       p.SaturateGraph,
			UseContiguousMemory: p.UseContiguousMemory,
		}
	case types.IndexTypeHNSWRabitq:
		return &HNSWRabitqParams{
			CommonParams:   CommonParams{MetricType: p.MetricType, QuantizeType: p.QuantizeType, EnableRotate: p.EnableRotate},
			M:              p.M,
			EFConstruction: p.EFConstruction,
			TotalBits:      p.TotalBits,
			NumClusters:    p.NumClusters,
			SampleCount:    p.SampleCount,
		}
	case types.IndexTypeFTS:
		return &FTSParams{
			Tokenizer:   p.FtsTokenizer,
			Filters:     p.FtsFilters,
			ExtraParams: p.FtsExtraParams,
		}
	case types.IndexTypeInvert:
		return &InvertParams{
			EnableRangeOptimization: p.EnableRangeOptimization,
			EnableExtendedWildcard:  p.EnableExtendedWildcard,
		}
	default:
		return &FlatParams{
			CommonParams: CommonParams{MetricType: p.MetricType, QuantizeType: p.QuantizeType},
		}
	}
}

func LegacyFromIndexConfig(cfg IndexConfig) *IndexParams {
	if cfg == nil {
		return nil
	}
	p := &IndexParams{
		Type:       cfg.GetIndexType(),
		MetricType: cfg.GetMetricType(),
	}
	switch c := cfg.(type) {
	case *HNSWParams:
		p.M = c.M
		p.EFConstruction = c.EFConstruction
		p.UseContiguousMemory = c.UseContiguousMemory
		p.QuantizeType = c.QuantizeType
		p.EnableRotate = c.EnableRotate
	case *IVFParams:
		p.NList = c.NList
		p.NIters = c.NIters
		p.UseSOAR = c.UseSOAR
		p.QuantizeType = c.QuantizeType
		p.EnableRotate = c.EnableRotate
	case *FlatParams:
		p.MajorOrder = c.MajorOrder
		p.QuantizeType = c.QuantizeType
		p.EnableRotate = c.EnableRotate
	case *DiskAnnParams:
		p.MaxDegree = c.MaxDegree
		p.ListSize = c.ListSize
		p.PQChunkNum = c.PQChunkNum
		p.Alpha = c.Alpha
		p.QuantizeType = c.QuantizeType
		p.EnableRotate = c.EnableRotate
	case *VamanaParams:
		p.VamanaMaxDegree = c.MaxDegree
		p.VamanaSearchListSize = c.SearchListSize
		p.VamanaAlpha = c.Alpha
		p.SaturateGraph = c.SaturateGraph
		p.UseContiguousMemory = c.UseContiguousMemory
		p.QuantizeType = c.QuantizeType
		p.EnableRotate = c.EnableRotate
	case *HNSWRabitqParams:
		p.M = c.M
		p.EFConstruction = c.EFConstruction
		p.TotalBits = c.TotalBits
		p.NumClusters = c.NumClusters
		p.SampleCount = c.SampleCount
		p.QuantizeType = c.QuantizeType
		p.EnableRotate = c.EnableRotate
	case *FTSParams:
		p.FtsTokenizer = c.Tokenizer
		p.FtsFilters = c.Filters
		p.FtsExtraParams = c.ExtraParams
	case *InvertParams:
		p.EnableRangeOptimization = c.EnableRangeOptimization
		p.EnableExtendedWildcard = c.EnableExtendedWildcard
	}
	return p
}
