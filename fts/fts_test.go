package fts

import (
	"strings"
	"testing"
)

// TestStandardTokenizer 验证标准分词器分词和转小写
func TestStandardTokenizer(t *testing.T) {
	tok := NewStandardTokenizer()
	tokens := tok.Tokenize("Hello World! This is a Test.")
	expected := []string{"hello", "world", "this", "is", "a", "test"}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Fatalf("token %d: expected '%s', got '%s'", i, expected[i], tok)
		}
	}
}

// TestWhitespaceTokenizer 验证空白分词器按空格分词
func TestWhitespaceTokenizer(t *testing.T) {
	tok := NewWhitespaceTokenizer()
	tokens := tok.Tokenize("hello   world foo")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
}

// TestInvertedIndexBasic 验证倒排索引添加文档、搜索和文档频率统计
func TestInvertedIndexBasic(t *testing.T) {
	idx := NewInvertedIndex()
	idx.AddDocument(0, strings.Fields("hello world"))
	idx.AddDocument(1, strings.Fields("hello foo"))
	idx.AddDocument(2, strings.Fields("world bar"))

	postings := idx.Search("hello")
	if len(postings) != 2 {
		t.Fatalf("expected 2 postings for 'hello', got %d", len(postings))
	}

	if idx.TotalDocs() != 3 {
		t.Fatalf("expected 3 total docs, got %d", idx.TotalDocs())
	}

	if idx.DocFreq("world") != 2 {
		t.Fatalf("expected doc freq 2 for 'world', got %d", idx.DocFreq("world"))
	}
}

// TestFTSIndexSearch 验证全文搜索索引 BM25 搜索排序
func TestFTSIndexSearch(t *testing.T) {
	idx := NewFTSIndex(NewStandardTokenizer())
	idx.Index(0, "the quick brown fox jumps over the lazy dog")
	idx.Index(1, "a quick brown dog jumps over the lazy fox")
	idx.Index(2, "the lazy cat sleeps all day")

	results := idx.Search("quick brown fox", 5)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	docIDs := make(map[uint64]bool)
	for _, r := range results {
		docIDs[r.DocID] = true
	}
	if !docIDs[0] || !docIDs[1] {
		t.Fatal("expected both doc 0 and doc 1 in top results for query 'quick brown fox'")
	}
}

// TestFTSIndexBooleanAND 验证全文搜索布尔 AND 查询
func TestFTSIndexBooleanAND(t *testing.T) {
	idx := NewFTSIndex(NewStandardTokenizer())
	idx.Index(0, "the quick brown fox")
	idx.Index(1, "the lazy dog")

	results := idx.SearchBoolean("quick fox", OpAND, 5)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for AND query, got %d", len(results))
	}
	if results[0].DocID != 0 {
		t.Fatalf("expected doc 0")
	}
}

// TestFTSIndexBooleanOR 验证全文搜索布尔 OR 查询
func TestFTSIndexBooleanOR(t *testing.T) {
	idx := NewFTSIndex(NewStandardTokenizer())
	idx.Index(0, "quick fox")
	idx.Index(1, "lazy dog")
	idx.Index(2, "brown bear")

	results := idx.SearchBoolean("quick bear", OpOR, 5)
	if len(results) != 2 {
		t.Fatalf("expected 2 results for OR query, got %d", len(results))
	}
}

// TestBM25Scorer 验证 BM25 评分器计算正分
func TestBM25Scorer(t *testing.T) {
	scorer := NewBM25Scorer()
	scorer.SetAvgDocLen(10.0)

	score := scorer.Score(0, 5, 2, 3, 100)
	if score <= 0 {
		t.Fatalf("expected positive BM25 score, got %f", score)
	}
}

// TestFTSIndexEmpty 验证空全文搜索索引搜索返回 nil
func TestFTSIndexEmpty(t *testing.T) {
	idx := NewFTSIndex(NewStandardTokenizer())
	results := idx.Search("anything", 5)
	if results != nil {
		t.Fatalf("expected nil for empty index")
	}
}

// TestFTSIndexNoMatch 验证全文搜索无匹配时返回空结果
func TestFTSIndexNoMatch(t *testing.T) {
	idx := NewFTSIndex(NewStandardTokenizer())
	idx.Index(0, "hello world")

	results := idx.Search("nonexistent", 5)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for no match")
	}
}
