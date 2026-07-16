package query

import (
	"github.com/third-apps/go-zvec/index/flat"
	"github.com/third-apps/go-zvec/index/param"
)

type VectorClause struct {
	QueryVector   []float32
	SparseIndices []uint32
	SparseValues  []float32
}

type FTSClause struct {
	QueryString string
	MatchString string
}

type QueryTarget struct {
	FieldName   string
	Vector      *VectorClause
	FTS         *FTSClause
	QueryParams param.QueryParam
}

type SearchQuery struct {
	Target        QueryTarget
	TopK          int
	Filter        string
	IncludeVector bool
	IncludeDocID  bool
	OutputFields  []string
}

type SubQuery struct {
	Target        QueryTarget
	NumCandidates int
}

type MultiQuery struct {
	SubQueries    []SubQuery
	TopK          int
	Filter        string
	IncludeVector bool
	IncludeDocID  bool
	OutputFields  []string
	Rerank        RerankParams
}

type RerankCallbackFunc func(results [][]flat.SearchResult, topN int) []flat.SearchResult

type RerankParams struct {
	Type        RerankType
	RRFConstant int
	Weights     []float64
	Callback    RerankCallbackFunc
}

type RerankType uint32

const (
	RerankTypeRRF      RerankType = 0
	RerankTypeWeighted RerankType = 1
	RerankTypeCallback RerankType = 2
)

type GroupByVectorQuery struct {
	Target        QueryTarget
	Filter        string
	IncludeVector bool
	OutputFields  []string
	GroupByField  string
	GroupCount    int
	TopKPerGroup  int
}

type GroupResult struct {
	GroupByValue string
	Docs         []map[string]interface{}
}
