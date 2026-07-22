package query

import (
	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/types"
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

type MetadataCondition struct {
	FieldName  string
	Op         MetadataOp
	ValueType  types.DataType
	StringVal  string
	Int64Val   int64
	BoolVal    bool
	StringVals []string
	Int64Vals  []int64
}

type MetadataOp uint8

const (
	MetadataOpEq     MetadataOp = 1
	MetadataOpIn     MetadataOp = 2
	MetadataOpNe     MetadataOp = 3
	MetadataOpGt     MetadataOp = 4
	MetadataOpLt     MetadataOp = 5
	MetadataOpGte    MetadataOp = 6
	MetadataOpLte    MetadataOp = 7
	MetadataOpExists MetadataOp = 8
)

type MetadataFilter struct {
	Conditions []MetadataCondition
}

func NewMetadataFilter() *MetadataFilter {
	return &MetadataFilter{}
}

func (f *MetadataFilter) WhereEq(fieldName string, value string) *MetadataFilter {
	f.Conditions = append(f.Conditions, MetadataCondition{
		FieldName: fieldName, Op: MetadataOpEq, ValueType: types.DataTypeString, StringVal: value,
	})
	return f
}

func (f *MetadataFilter) WhereIn(fieldName string, values []string) *MetadataFilter {
	f.Conditions = append(f.Conditions, MetadataCondition{
		FieldName: fieldName, Op: MetadataOpIn, ValueType: types.DataTypeString, StringVals: values,
	})
	return f
}

func (f *MetadataFilter) WhereIntEq(fieldName string, value int64) *MetadataFilter {
	f.Conditions = append(f.Conditions, MetadataCondition{
		FieldName: fieldName, Op: MetadataOpEq, ValueType: types.DataTypeInt64, Int64Val: value,
	})
	return f
}

func (f *MetadataFilter) WhereBoolEq(fieldName string, value bool) *MetadataFilter {
	f.Conditions = append(f.Conditions, MetadataCondition{
		FieldName: fieldName, Op: MetadataOpEq, ValueType: types.DataTypeBool, BoolVal: value,
	})
	return f
}

func (f *MetadataFilter) WhereIntGt(fieldName string, value int64) *MetadataFilter {
	f.Conditions = append(f.Conditions, MetadataCondition{
		FieldName: fieldName, Op: MetadataOpGt, ValueType: types.DataTypeInt64, Int64Val: value,
	})
	return f
}

func (f *MetadataFilter) WhereIntLt(fieldName string, value int64) *MetadataFilter {
	f.Conditions = append(f.Conditions, MetadataCondition{
		FieldName: fieldName, Op: MetadataOpLt, ValueType: types.DataTypeInt64, Int64Val: value,
	})
	return f
}

func (f *MetadataFilter) WhereIntIn(fieldName string, values []int64) *MetadataFilter {
	f.Conditions = append(f.Conditions, MetadataCondition{
		FieldName: fieldName, Op: MetadataOpIn, ValueType: types.DataTypeInt64, Int64Vals: values,
	})
	return f
}

type SearchQuery struct {
	Target        QueryTarget
	TopK          int
	Filter        string
	MetaFilter    *MetadataFilter
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

type RerankCallbackFunc func(results [][]types.SearchResult, topN int) []types.SearchResult

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
