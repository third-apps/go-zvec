package ivf

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/third-apps/go-zvec/metric"
	"github.com/third-apps/go-zvec/types"
)

// TestIVFIndexBasic 验证 IVF 聚类索引基本添加和搜索功能
func TestIVFIndexBasic(t *testing.T) {
	idx := NewIVFIndex(4, types.MetricTypeCosine, 2, 10)
	idx.Add([]float32{0.1, 0.2, 0.3, 0.4}, "doc_1")
	idx.Add([]float32{0.2, 0.3, 0.4, 0.1}, "doc_2")
	idx.Add([]float32{0.9, 0.8, 0.7, 0.6}, "doc_3")
	idx.Add([]float32{0.8, 0.9, 0.6, 0.7}, "doc_4")

	results := idx.Search([]float32{0.4, 0.3, 0.3, 0.1}, 3)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Score <= 0 {
		t.Fatalf("expected positive score, got %f", results[0].Score)
	}
	_ = results[0].PK
}

// TestIVFIndexEmpty 验证空 IVF 索引搜索返回 nil
func TestIVFIndexEmpty(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeL2, 2, 10)
	results := idx.Search([]float32{1, 2}, 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

// TestIVFIndexDelete 验证 IVF 索引删除文档功能
func TestIVFIndexDelete(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeCosine, 2, 10)
	idx.Add([]float32{1, 0}, "doc_1")
	idx.Add([]float32{0, 1}, "doc_2")

	if !idx.Delete("doc_1") {
		t.Fatal("expected delete to succeed")
	}
	if idx.Size() != 1 {
		t.Fatalf("expected size 1, got %d", idx.Size())
	}
}

// TestIVFIndexWithFilter 验证 IVF 索引带过滤条件搜索功能
func TestIVFIndexWithFilter(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeCosine, 3, 10)
	for i := 0; i < 30; i++ {
		v := []float32{float32(i) / 30, float32(30-i) / 30}
		idx.Add(v, fmt.Sprintf("doc_%d", i))
	}

	results := idx.SearchWithFilter([]float32{1, 0}, 5, func(pk string) bool {
		return pk != "doc_0"
	})
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	for _, r := range results {
		if r.PK == "doc_0" {
			t.Fatal("filter should have excluded doc_0")
		}
	}
}

// TestIVFIndexLarge 验证 IVF 索引大规模数据搜索功能
func TestIVFIndexLarge(t *testing.T) {
	idx := NewIVFIndex(8, types.MetricTypeCosine, 3, 10)
	for i := 0; i < 100; i++ {
		v := make([]float32, 8)
		for j := range v {
			v[j] = float32(i*10 + j)
		}
		idx.Add(v, "")
	}

	q := make([]float32, 8)
	for j := range q {
		q[j] = float32(500 + j)
	}

	results := idx.Search(q, 10)
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
}

// TestIVFExplicitTrain 验证 IVF 索引显式训练后质心数量正确
func TestIVFExplicitTrain(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeCosine, 2, 10)
	idx.Add([]float32{1, 0}, "a")
	idx.Add([]float32{0, 1}, "b")
	idx.Add([]float32{1, 1}, "c")
	idx.Train()

	if !idx.trained {
		t.Fatal("expected trained after Train()")
	}
	if len(idx.centroids) != 2 {
		t.Fatalf("expected 2 centroids, got %d", len(idx.centroids))
	}
}

// TestIVFTrainAfterAdd 验证 IVF 索引训练后新增文档的聚类分配
func TestIVFTrainAfterAdd(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeL2, 2, 5)
	idx.Add([]float32{1, 0}, "a")
	idx.Add([]float32{0, 1}, "b")
	idx.Train()

	docID := idx.Add([]float32{1, 1}, "c")
	if idx.assignments[docID] < 0 {
		t.Fatal("expected assignment >= 0 after train")
	}
}

// TestIVFKMeansPPEdgeCases 验证 KMeans++ 初始化单数据点边界情况
func TestIVFKMeansPPEdgeCases(t *testing.T) {
	data := [][]float32{{1, 2, 3}}
	centroids := kmeansPP(data, 1, metric.CosineDistance, 5)
	if len(centroids) != 1 {
		t.Fatalf("expected 1 centroid for 1 data point, got %d", len(centroids))
	}
}

// TestIVFKMeansPPMoreCentroids 验证 KMeans++ 初始化多质心情况
func TestIVFKMeansPPMoreCentroids(t *testing.T) {
	data := [][]float32{{1, 0}, {0, 1}, {1, 1}, {0, 0}}
	centroids := kmeansPP(data, 4, metric.CosineDistance, 10)
	if len(centroids) != 4 {
		t.Fatalf("expected 4 centroids, got %d", len(centroids))
	}
}

// TestIVFEmptyData 验证 IVF 索引无数据时训练不标记为已训练
func TestIVFEmptyData(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeL2, 2, 5)
	idx.Train()
	if idx.trained {
		t.Fatal("expected not trained with no data")
	}
}

// TestIVFDeleteUpdatesInvertedIndex 验证 IVF 索引删除文档后倒排索引一致性
func TestIVFDeleteUpdatesInvertedIndex(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeCosine, 2, 10)
	idx.Add([]float32{1, 0}, "a")
	idx.Add([]float32{0, 1}, "b")
	idx.Add([]float32{1, 1}, "c")
	idx.Train()

	idx.Delete("a")
	for _, inv := range idx.inverted {
		for _, docIdx := range inv {
			if docIdx >= len(idx.docs) {
				t.Fatalf("inverted index %d out of range (len=%d)", docIdx, len(idx.docs))
			}
		}
	}
}

// TestIVFIndexDimension 验证 IVF 索引维度查询
func TestIVFIndexDimension(t *testing.T) {
	idx := NewIVFIndex(4, types.MetricTypeL2, 2, 5)
	if idx.Dimension() != 4 {
		t.Fatalf("expected dimension 4, got %d", idx.Dimension())
	}
}

// TestIVFIndexCosineNormalize 验证 IVF 索引余弦度量下向量归一化搜索正确性
func TestIVFIndexCosineNormalize(t *testing.T) {
	idx := NewIVFIndex(2, types.MetricTypeCosine, 1, 5)
	idx.Add([]float32{3, 4}, "a")
	idx.Add([]float32{4, 3}, "b")
	results := idx.Search([]float32{3, 4}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].PK != "a" {
		t.Fatalf("expected 'a' as top result, got %s", results[0].PK)
	}
}

// TestIVFIndexSaveLoad 验证 IVF 索引序列化保存与反序列化加载
func TestIVFIndexSaveLoad(t *testing.T) {
	idx := NewIVFIndex(4, types.MetricTypeCosine, 4, 10)
	idx.Add([]float32{0.1, 0.2, 0.3, 0.4}, "doc_1")
	idx.Add([]float32{0.2, 0.3, 0.4, 0.1}, "doc_2")
	idx.Add([]float32{0.9, 0.8, 0.7, 0.6}, "doc_3")
	idx.Add([]float32{0.5, 0.5, 0.5, 0.5}, "doc_4")
	idx.Train()
	idx.Delete("doc_2")

	var buf bytes.Buffer
	if err := idx.Save(&buf); err != nil {
		t.Fatal(err)
	}

	idx2 := NewIVFIndex(4, types.MetricTypeCosine, 4, 10)
	if err := idx2.Load(&buf); err != nil {
		t.Fatal(err)
	}

	if idx2.Size() != 3 {
		t.Fatalf("expected size 3, got %d", idx2.Size())
	}

	results := idx2.Search([]float32{0.4, 0.3, 0.3, 0.1}, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}
