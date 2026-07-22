package reranker

import (
	"testing"

	"github.com/third-apps/go-zvec/types"
)

// TestRRFReranker 验证 RRF 倒数排名融合重排序
func TestRRFReranker(t *testing.T) {
	r1 := []types.SearchResult{
		{PK: "doc_a", Score: 0.9},
		{PK: "doc_b", Score: 0.5},
		{PK: "doc_c", Score: 0.3},
	}
	r2 := []types.SearchResult{
		{PK: "doc_b", Score: 0.8},
		{PK: "doc_d", Score: 0.7},
		{PK: "doc_a", Score: 0.2},
	}

	params := NewRRFParams(60)
	results := params.Rerank([][]types.SearchResult{r1, r2}, 5)

	if len(results) != 4 {
		t.Fatalf("expected 4 unique results, got %d", len(results))
	}

	if results[0].PK != "doc_b" {
		t.Fatalf("expected doc_b as top result (ranked high in both), got %s", results[0].PK)
	}
}

// TestWeightedReranker 验证加权分数融合重排序
func TestWeightedReranker(t *testing.T) {
	r1 := []types.SearchResult{
		{PK: "doc_a", Score: 0.9},
		{PK: "doc_b", Score: 0.8},
	}
	r2 := []types.SearchResult{
		{PK: "doc_b", Score: 0.1},
		{PK: "doc_c", Score: 0.9},
	}

	params := NewWeightedParams([]float64{0.7, 0.3})
	results := params.Rerank([][]types.SearchResult{r1, r2}, 3)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

// TestNormalizeScores 验证搜索结果分数归一化
func TestNormalizeScores(t *testing.T) {
	results := []types.SearchResult{
		{PK: "a", Score: 1.0},
		{PK: "b", Score: 0.5},
		{PK: "c", Score: 0.0},
	}

	normalized := normalizeScores(results)
	if normalized[0].Score != 1.0 {
		t.Fatalf("expected max to normalize to 1.0, got %f", normalized[0].Score)
	}
	if normalized[1].Score != 0.5 {
		t.Fatalf("expected 0.5, got %f", normalized[1].Score)
	}
}

// TestEmptyResults 验证空搜索结果重排序返回空
func TestEmptyResults(t *testing.T) {
	params := NewRRFParams(60)
	results := params.Rerank([][]types.SearchResult{{}}, 10)
	if len(results) != 0 {
		t.Fatalf("expected empty results")
	}
}

// TestRerankWithInterface 验证通过 Rerank 接口函数执行重排序
func TestRerankWithInterface(t *testing.T) {
	r1 := []types.SearchResult{{PK: "a", Score: 1.0}, {PK: "b", Score: 0.5}}
	r2 := []types.SearchResult{{PK: "b", Score: 0.8}, {PK: "c", Score: 0.3}}

	results := Rerank(NewRRFParams(60), [][]types.SearchResult{r1, r2}, 3)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}
