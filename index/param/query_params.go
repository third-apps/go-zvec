package param

import "github.com/third-apps/go-zvec/types"

type QueryParam interface {
	IndexType() types.IndexType
}

type HNSWQueryParam struct {
	EF             int
	Radius         float32
	IsLinear       bool
	IsUsingRefiner bool
	PrefetchOffset int
	PrefetchLines  int
}

func NewHNSWQueryParam(ef int, radius float32, isLinear bool) *HNSWQueryParam {
	return &HNSWQueryParam{EF: ef, Radius: radius, IsLinear: isLinear}
}

func (p *HNSWQueryParam) IndexType() types.IndexType { return types.IndexTypeHNSW }

type IVFQueryParam struct {
	NProbe         int
	ScaleFactor    float32
	Radius         float32
	IsLinear       bool
	IsUsingRefiner bool
}

func NewIVFQueryParam(nprobe int, scaleFactor float32) *IVFQueryParam {
	return &IVFQueryParam{NProbe: nprobe, ScaleFactor: scaleFactor}
}

func (p *IVFQueryParam) IndexType() types.IndexType { return types.IndexTypeIVF }

type FlatQueryParam struct {
	IsUsingRefiner bool
	ScaleFactor    float32
}

func NewFlatQueryParam() *FlatQueryParam { return &FlatQueryParam{} }

func (p *FlatQueryParam) IndexType() types.IndexType { return types.IndexTypeFlat }

type FTSQueryParam struct {
	DefaultOperator string
}

func NewFTSQueryParam(defaultOp string) *FTSQueryParam {
	return &FTSQueryParam{DefaultOperator: defaultOp}
}

func (p *FTSQueryParam) IndexType() types.IndexType { return types.IndexTypeFTS }

type DiskAnnQueryParam struct {
	ListSize int
}

func (p *DiskAnnQueryParam) IndexType() types.IndexType { return types.IndexTypeDiskAnn }

type VamanaQueryParam struct {
	EFSearch       int
	Radius         float32
	IsLinear       bool
	IsUsingRefiner bool
	PrefetchOffset int
	PrefetchLines  int
}

func (p *VamanaQueryParam) IndexType() types.IndexType { return types.IndexTypeVamana }

type HNSWRabitqQueryParam struct {
	EF             int
	Radius         float32
	IsLinear       bool
	IsUsingRefiner bool
}

func (p *HNSWRabitqQueryParam) IndexType() types.IndexType { return types.IndexTypeHNSWRabitq }
