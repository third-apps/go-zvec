package param

import (
	"testing"

	"github.com/third-apps/go-zvec/types"
)

// TestNewHNSWParams 验证 HNSW 索引参数构造正确性
func TestNewHNSWParams(t *testing.T) {
	p := NewHNSWParams(types.MetricTypeCosine, 16, 200)
	if p.GetIndexType() != types.IndexTypeHNSW {
		t.Fatal("expected HNSW index type")
	}
	if p.GetMetricType() != types.MetricTypeCosine {
		t.Fatal("expected Cosine metric")
	}
	if p.M != 16 || p.EFConstruction != 200 {
		t.Fatalf("unexpected M/EF: %d/%d", p.M, p.EFConstruction)
	}
}

// TestNewVamanaParams 验证 Vamana 索引参数构造正确性
func TestNewVamanaParams(t *testing.T) {
	p := NewVamanaParams(types.MetricTypeL2, 32, 100, 1.2, false, false)
	if p.GetIndexType() != types.IndexTypeVamana {
		t.Fatal("expected Vamana index type")
	}
	if p.MaxDegree != 32 || p.SearchListSize != 100 {
		t.Fatalf("unexpected degree/searchList: %d/%d", p.MaxDegree, p.SearchListSize)
	}
}

// TestNewFlatParams 验证 Flat 索引参数构造正确性
func TestNewFlatParams(t *testing.T) {
	p := NewFlatIndexParams(types.MetricTypeIP)
	if p.GetIndexType() != types.IndexTypeFlat {
		t.Fatal("expected Flat index type")
	}
	if p.GetMetricType() != types.MetricTypeIP {
		t.Fatal("expected IP metric")
	}
}

// TestNewIVFParams 验证 IVF 索引参数构造正确性
func TestNewIVFParams(t *testing.T) {
	p := NewIVFIndexParams(types.MetricTypeCosine, 128, 20, false)
	if p.GetIndexType() != types.IndexTypeIVF {
		t.Fatal("expected IVF index type")
	}
}

// TestNewFTSParams 验证全文搜索索引参数构造正确性（standard 分词器）
func TestNewFTSParams(t *testing.T) {
	p := NewFTSParams("standard", nil, "")
	if p.GetIndexType() != types.IndexTypeFTS {
		t.Fatal("expected FTS index type")
	}
	if p.Tokenizer != "standard" {
		t.Fatalf("expected 'standard', got '%s'", p.Tokenizer)
	}
}

// TestNewFTSParamsJieba 验证全文搜索索引参数构造正确性（jieba 分词器）
func TestNewFTSParamsJieba(t *testing.T) {
	p := NewFTSParams("jieba", nil, "")
	if p.Tokenizer != "jieba" {
		t.Fatalf("expected 'jieba', got '%s'", p.Tokenizer)
	}
}

// TestNewInvertParams 验证倒排索引参数构造正确性
func TestNewInvertParams(t *testing.T) {
	p := NewInvertParams(true, true)
	if p.GetIndexType() != types.IndexTypeInvert {
		t.Fatal("expected Invert index type")
	}
}

// TestNewDiskAnnParams 验证 DiskAnn 索引参数构造正确性
func TestNewDiskAnnParams(t *testing.T) {
	p := NewDiskAnnParams(types.MetricTypeCosine, 32, 100, 10)
	if p.GetIndexType() != types.IndexTypeDiskAnn {
		t.Fatal("expected DiskAnn index type")
	}
}

// TestNewHNSWRabitqParams 验证 HNSW RaBitQ 量化索引参数构造正确性
func TestNewHNSWRabitqParams(t *testing.T) {
	p := NewHNSWRabitqIndexParams(types.MetricTypeCosine, 4, 256, 16, 200, 1000)
	if p.GetIndexType() != types.IndexTypeHNSWRabitq {
		t.Fatal("expected HNSWRabitq index type")
	}
}

// TestIndexConfigFromLegacy 验证旧版参数转换为 IndexConfig 接口
func TestIndexConfigFromLegacy(t *testing.T) {
	p := NewHNSWIndexParams(types.MetricTypeCosine, 16, 200)
	cfg := IndexConfigFromLegacy(p)
	if cfg.GetIndexType() != types.IndexTypeHNSW {
		t.Fatal("expected HNSW from legacy")
	}
}

// TestLegacyFromIndexConfig 验证 IndexConfig 接口转换为旧版参数
func TestLegacyFromIndexConfig(t *testing.T) {
	cfg := NewHNSWParams(types.MetricTypeL2, 32, 400)
	p := LegacyFromIndexConfig(cfg)
	if p.GetIndexType() != types.IndexTypeHNSW {
		t.Fatal("expected HNSW from config")
	}
	if p.MetricType != types.MetricTypeL2 {
		t.Fatal("expected L2 metric")
	}
}
