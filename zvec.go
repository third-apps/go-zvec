package zvec

import (
	"github.com/third-apps/go-zvec/collection"
	"github.com/third-apps/go-zvec/config"
	"github.com/third-apps/go-zvec/doc"
	"github.com/third-apps/go-zvec/index"
	"github.com/third-apps/go-zvec/index/param"
	"github.com/third-apps/go-zvec/query"
	"github.com/third-apps/go-zvec/schema"
	"github.com/third-apps/go-zvec/status"
	"github.com/third-apps/go-zvec/types"
)

func Init(opts ...InitOption) error {
	cfg := &config.ConfigData{}
	for _, opt := range opts {
		opt(cfg)
	}
	return config.Initialize(cfg)
}

type InitOption func(*config.ConfigData)

func WithLogConsole(level types.LogLevel) InitOption {
	return func(cfg *config.ConfigData) {
		cfg.LogConfig = config.NewConsoleLogConfig(level)
	}
}

func WithLogFile(level types.LogLevel, dir, basename string, fileSizeMB, overdueDays uint32) InitOption {
	return func(cfg *config.ConfigData) {
		cfg.LogConfig = config.NewFileLogConfig(level, dir, basename, fileSizeMB, overdueDays)
	}
}

func WithQueryThreads(n uint32) InitOption {
	return func(cfg *config.ConfigData) { cfg.QueryThreadCount = n }
}

func WithOptimizeThreads(n uint32) InitOption {
	return func(cfg *config.ConfigData) { cfg.OptimizeThreadCount = n }
}

func WithMemoryLimitMB(mb uint64) InitOption {
	return func(cfg *config.ConfigData) { cfg.MemoryLimitBytes = mb * 1024 * 1024 }
}

func WithJiebaDictDir(dir string) InitOption {
	return func(cfg *config.ConfigData) { cfg.JiebaDictDir = dir }
}

func CreateAndOpen(path string, s *schema.CollectionSchema, opts *collection.Options) (*collection.Collection, error) {
	return collection.CreateAndOpen(path, s, opts)
}

func Open(path string, opts *collection.Options) (*collection.Collection, error) {
	return collection.Open(path, opts)
}

func Shutdown() {
	config.Shutdown()
}

func IsInitialized() bool {
	return config.IsInitialized()
}

func GetVersion() string {
	return "0.1.0-pure-go"
}

// Roadmap:
// v0.5 - Multi Tenant (Collection-level isolation, resource quotas)
// v0.8 - HTTP/gRPC API layer, multi-Collection management
// v1.0 - Distributed sharding, replication, consensus (raft)
//        This transforms go-zvec from an embedded vector database
//        into a distributed vector database service (Qdrant/Milvus class).
//        Requires: cluster management, leader election, cross-node search,
//        data partitioning, fault tolerance. Only pursue when the embedded
//        version is mature and production-proven.

func SetDefaultJiebaDictDir(dir string) {
	config.SetDefaultJiebaDictDir(dir)
}

func GetDefaultJiebaDictDir() string {
	return config.GetDefaultJiebaDictDir()
}

// Convenience re-exports.
//
// These type aliases are provided for ergonomic API usage so callers can
// reference types via "zvec.Doc" instead of "doc.Doc". Each alias maps
// directly to its underlying package type; no additional abstraction is
// introduced. For fine-grained control, import the sub-packages directly.
type (
	Doc               = doc.Doc
	Value             = doc.Value
	VectorValue       = doc.VectorValue
	SparseVectorValue = doc.SparseVectorValue

	FieldSchema      = schema.FieldSchema
	CollectionSchema = schema.CollectionSchema
	CollectionStats  = schema.CollectionStats

	CollectionOptions = collection.Options
	OptimizeOptions   = collection.OptimizeOptions

	HNSWParams       = param.HNSWParams
	IVFParams        = param.IVFParams
	FlatParams       = param.FlatParams
	FTSParams        = param.FTSParams
	InvertParams     = param.InvertParams
	DiskAnnParams    = param.DiskAnnParams
	VamanaParams     = param.VamanaParams
	HNSWRabitqParams = param.HNSWRabitqParams
	IndexConfig      = param.IndexConfig

	SearchQuery        = query.SearchQuery
	SearchResult       = types.SearchResult
	Index              = index.Index
	BatchBuilder       = index.BatchBuilder
	MultiQuery         = query.MultiQuery
	GroupByVectorQuery = query.GroupByVectorQuery
	GroupResult        = query.GroupResult
	FTSClause          = query.FTSClause
	SubQuery           = query.SubQuery
	RerankParams       = query.RerankParams
	MetadataFilter     = query.MetadataFilter
	MetadataCondition  = query.MetadataCondition

	Status = status.Status
)

const (
	DataTypeUndefined        = types.DataTypeUndefined
	DataTypeBinary           = types.DataTypeBinary
	DataTypeString           = types.DataTypeString
	DataTypeBool             = types.DataTypeBool
	DataTypeInt32            = types.DataTypeInt32
	DataTypeInt64            = types.DataTypeInt64
	DataTypeUint32           = types.DataTypeUint32
	DataTypeUint64           = types.DataTypeUint64
	DataTypeFloat            = types.DataTypeFloat
	DataTypeDouble           = types.DataTypeDouble
	DataTypeVectorBinary32   = types.DataTypeVectorBinary32
	DataTypeVectorBinary64   = types.DataTypeVectorBinary64
	DataTypeVectorFP16       = types.DataTypeVectorFP16
	DataTypeVectorFP32       = types.DataTypeVectorFP32
	DataTypeVectorFP64       = types.DataTypeVectorFP64
	DataTypeVectorInt4       = types.DataTypeVectorInt4
	DataTypeVectorInt8       = types.DataTypeVectorInt8
	DataTypeVectorInt16      = types.DataTypeVectorInt16
	DataTypeSparseVectorFP16 = types.DataTypeSparseVectorFP16
	DataTypeSparseVectorFP32 = types.DataTypeSparseVectorFP32
	DataTypeArrayBinary      = types.DataTypeArrayBinary
	DataTypeArrayString      = types.DataTypeArrayString
	DataTypeArrayBool        = types.DataTypeArrayBool
	DataTypeArrayInt32       = types.DataTypeArrayInt32
	DataTypeArrayInt64       = types.DataTypeArrayInt64
	DataTypeArrayUint32      = types.DataTypeArrayUint32
	DataTypeArrayUint64      = types.DataTypeArrayUint64
	DataTypeArrayFloat       = types.DataTypeArrayFloat
	DataTypeArrayDouble      = types.DataTypeArrayDouble

	IndexTypeUndefined  = types.IndexTypeUndefined
	IndexTypeHNSW       = types.IndexTypeHNSW
	IndexTypeIVF        = types.IndexTypeIVF
	IndexTypeFlat       = types.IndexTypeFlat
	IndexTypeDiskAnn    = types.IndexTypeDiskAnn
	IndexTypeVamana     = types.IndexTypeVamana
	IndexTypeInvert     = types.IndexTypeInvert
	IndexTypeFTS        = types.IndexTypeFTS
	IndexTypeHNSWRabitq = types.IndexTypeHNSWRabitq

	MetricTypeUndefined = types.MetricTypeUndefined
	MetricTypeL2        = types.MetricTypeL2
	MetricTypeIP        = types.MetricTypeIP
	MetricTypeCosine    = types.MetricTypeCosine
	MetricTypeMIPSL2    = types.MetricTypeMIPSL2

	QuantizeTypeUndefined = types.QuantizeTypeUndefined
	QuantizeTypeFP16      = types.QuantizeTypeFP16
	QuantizeTypeInt8      = types.QuantizeTypeInt8
	QuantizeTypeInt4      = types.QuantizeTypeInt4
	QuantizeTypeRaBitQ    = types.QuantizeTypeRaBitQ
	QuantizeTypePQ        = types.QuantizeTypePQ

	LogLevelDebug = types.LogLevelDebug
	LogLevelInfo  = types.LogLevelInfo
	LogLevelWarn  = types.LogLevelWarn
	LogLevelError = types.LogLevelError
	LogLevelFatal = types.LogLevelFatal

	LogTypeConsole = types.LogTypeConsole
	LogTypeFile    = types.LogTypeFile
)

var (
	NewDoc              = doc.NewDoc
	NewFieldSchema      = schema.NewFieldSchema
	NewCollectionSchema = schema.NewCollectionSchema

	NewHNSWParams       = param.NewHNSWParams
	NewIVFParams        = param.NewIVFParams
	NewFlatParams       = param.NewFlatParams
	NewFTSParams        = param.NewFTSParams
	NewInvertParams     = param.NewInvertParams
	NewDiskAnnParams    = param.NewDiskAnnParams
	NewVamanaParams     = param.NewVamanaParams
	NewHNSWRabitqParams = param.NewHNSWRabitqParams

	NewHNSWIndexParam       = param.NewHNSWIndexParams
	NewIVFIndexParam        = param.NewIVFIndexParams
	NewFlatIndexParam       = param.NewFlatIndexParams
	NewFTSIndexParam        = param.NewFTSIndexParams
	NewInvertIndexParam     = param.NewInvertIndexParams
	NewDiskAnnIndexParam    = param.NewDiskAnnIndexParams
	NewVamanaIndexParam     = param.NewVamanaIndexParams
	NewHNSWRabitqIndexParam = param.NewHNSWRabitqIndexParams
	NewMetadataFilter       = query.NewMetadataFilter
)

func LoadCollection(path string, opts *collection.Options) (*collection.Collection, error) {
	return collection.LoadCollection(path, opts)
}
