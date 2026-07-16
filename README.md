# Go-Zvec

**纯 Go 向量数据库 / Pure Go Vector Database** — Alibaba Zvec C++ 引擎的纯 Go 移植版，零 CGo 依赖。

Zero CGo, pure Go re-implementation of the Alibaba Zvec vector database engine.

---

## 功能特性 / Features

### 索引类型 / Index Types

| 索引 | 说明 | 状态 |
|---|---|---|
| **Flat** | 暴力搜索，精确结果 | ✅ |
| **HNSW** | 分层可导航小世界图，近似最近邻 | ✅ |
| **HNSW RaBitQ** | RaBitQ 量化版 HNSW，1-bit 编码加速搜索 | ✅ |
| **IVF** | K-Means++ 聚类倒排文件，可配置 nprobe | ✅ |
| **Vamana** | DiskANN 核心图算法 | ✅ |
| **DiskAnn** | 磁盘友好图索引，MMAP + LRU 缓存 | ✅ |
| **Flat Sparse** | 稀疏向量暴力搜索 | ✅ |
| **Invert** | 标量值倒排索引，精确过滤 | ✅ |
| **FTS** | 全文搜索，BM25 评分 | ✅ |

### 量化器 / Quantizers

| 量化器 | 说明 | 状态 |
|---|---|---|
| **FP16** | 半精度浮点量化 | ✅ |
| **Int8** | 8-bit 整数量化 + 随机旋转 | ✅ |
| **Int4** | 4-bit 整数量化 + 随机旋转 | ✅ |
| **RaBitQ** | 随机位量化，1-bit + 范数 | ✅ |
| **PQ** | 乘积量化，K-Means 子空间聚类 | ✅ |

### 距离度量 / Distance Metrics

| 度量 | 说明 | 状态 |
|---|---|---|
| **L2** | 欧几里得距离 | ✅ |
| **IP** | 内积 | ✅ |
| **Cosine** | 余弦距离 | ✅ |
| **MIPSL2** | 最大内积搜索 + L2 | ✅ |
| **SparseIP** | 稀疏向量内积 | ✅ |

### 全文搜索 / Full-Text Search

| 功能 | 说明 | 状态 |
|---|---|---|
| **StandardTokenizer** | Unicode 字母/数字分词 | ✅ |
| **WhitespaceTokenizer** | 空格分词 | ✅ |
| **JiebaTokenizer** | 中文正向最大匹配分词（内置词典 + 自定义词典） | ✅ |
| **BM25 评分** | 经典 BM25 相关性评分 | ✅ |
| **查询语法** | AND / OR / NOT / 短语查询 / 括号分组 | ✅ |
| **SearchAdvanced** | 基于语法树的进阶搜索 | ✅ |

### 集合操作 / Collection Operations

| 操作 | 说明 | 状态 |
|---|---|---|
| **CRUD** | Insert / Upsert / Update / Delete / Fetch | ✅ |
| **DeleteByFilter** | 按过滤条件批量删除 | ✅ |
| **Optimize** | 批量重建索引（支持 Concurrency 参数） | ✅ |
| **CreateIndex / DropIndex** | 动态创建/删除索引 | ✅ |
| **AddColumn / DropColumn / AlterColumn** | 动态列管理 | ✅ |
| **Flush** | WAL 同步刷盘 | ✅ |
| **GroupBy** | 分组聚合搜索 | ✅ |
| **MultiQuery** | 多子查询 + RRF/Weighted/Callback 重排序 | ✅ |

### 过滤表达式 / Filter Expressions

| 操作符 | 说明 | 状态 |
|---|---|---|
| `>=` `<=` `>` `<` `==` `!=` | 数值/字符串比较 | ✅ |
| `LIKE` | 通配符模式匹配（`*` 和 `?`） | ✅ |
| `IS_NULL` / `IS_NOT_NULL` | 空值检查 | ✅ |
| `HAS_PREFIX` / `HAS_SUFFIX` | 前缀/后缀匹配 | ✅ |
| `CONTAIN_ALL` / `CONTAIN_ANY` | 包含全部/任一值 | ✅ |
| `NOT_CONTAIN_ALL` / `NOT_CONTAIN_ANY` | 不包含全部/任一值 | ✅ |

### 存储 / Storage

| 类型 | 说明 | 状态 |
|---|---|---|
| **FileStorage** | 文件存储 | ✅ |
| **MemoryStorage** | 内存存储 | ✅ |
| **MMAPStorage** | Windows 内存映射文件 | ✅ |
| **WAL** | JSON 预写日志，崩溃恢复 | ✅ |
| **Segment** | 分段文档管理 | ✅ |

---

## 快速开始 / Quick Start

```go
package main

import (
    "fmt"
    "github.com/third-apps/go-zvec"
    "github.com/third-apps/go-zvec/doc"
    "github.com/third-apps/go-zvec/index/param"
    "github.com/third-apps/go-zvec/query"
    "github.com/third-apps/go-zvec/schema"
    "github.com/third-apps/go-zvec/types"
)

func main() {
    s := schema.NewCollectionSchema("demo")
    vecF := schema.NewFieldSchema("vector", types.DataTypeVectorFP32, false, 4)
    vecF.SetIndexParams(param.NewHNSWIndexParams(types.MetricTypeCosine, 16, 200))
    s.AddField(vecF)

    c, _ := zvec.CreateAndOpen("./data", s, nil)
    defer c.Close()

    d := doc.NewDoc("doc1")
    d.SetVectorFP32Field("vector", []float32{0.1, 0.2, 0.3, 0.4})
    c.Insert([]*doc.Doc{d})

    results, _ := c.Query(&query.SearchQuery{
        Target: query.QueryTarget{
            FieldName: "vector",
            Vector:    &query.VectorClause{QueryVector: []float32{0.4, 0.3, 0.3, 0.1}},
        },
        TopK: 5,
    })
    fmt.Println(results[0]["id"], results[0]["score"])
}
```

---

## 包结构 / Package Structure

```
go-zvec/
├── zvec.go              # Public API (Init/CreateAndOpen/Open/Shutdown)
├── collection/          # Collection CRUD, index management, query, optimize
├── config/              # Global configuration
├── doc/                 # Document model (fields, vectors, validation)
├── fts/                 # Full-text search (tokenizer, BM25, query parser, Jieba)
├── index/
│   ├── diskann/         # Disk-based Vamana with MMAP storage + LRU cache
│   ├── flat/            # Brute-force flat index
│   ├── flat_sparse/     # Sparse vector brute-force index
│   ├── hnsw/            # HNSW graph index
│   ├── hnsw_rabitq/     # RaBitQ quantized HNSW index
│   ├── invert/          # Inverted index for scalar fields
│   ├── ivf/             # IVF cluster index (configurable nprobe)
│   ├── vamana/          # Vamana filtered graph index
│   └── param/           # Index & query parameter types
├── metric/              # Distance functions (L2, IP, Cosine, SparseIP)
├── quantizer/           # FP16 / Int8 / Int4 / RaBitQ / PQ quantizers
├── query/               # Query types (SearchQuery, MultiQuery, GroupBy)
├── reranker/            # RRF / Weighted / Callback reranking
├── schema/              # FieldSchema / CollectionSchema
├── segment/             # Multi-segment document management
├── status/              # Status & Result[T] error handling
├── storage/             # FileStorage / MemoryStorage / MMAPStorage
├── types/               # DataType, IndexType, MetricType, QuantizeType, etc.
├── wal/                 # Write-ahead log
└── examples/            # Usage examples
```

---

## 多查询 + 重排序 / MultiQuery + Rerank

```go
results, _ := c.MultiQuery(&query.MultiQuery{
    SubQueries: []query.SubQuery{
        {Target: query.QueryTarget{FieldName: "v1", Vector: &query.VectorClause{QueryVector: q1}}},
        {Target: query.QueryTarget{FieldName: "v2", Vector: &query.VectorClause{QueryVector: q2}}},
    },
    TopK: 10,
    Rerank: query.RerankParams{Type: query.RerankTypeRRF, RRFConstant: 60},
})
```

## 全文搜索 / Full-Text Search

```go
// 创建 FTS 索引
ftsField := schema.NewFieldSchema("text", types.DataTypeString, false, 0)
ftsField.SetIndexParams(param.NewFTSIndexParams("standard", nil, ""))
s.AddField(ftsField)

// 使用查询语法搜索
results, _ := c.FTSQuery("text", `"hello world" AND go`, 10)
```

## 中文分词 / Chinese Tokenization

```go
import "github.com/third-apps/go-zvec/fts"

// 使用内置 Jieba 分词器
tokenizer := fts.NewJiebaTokenizer()

// 或自定义词典
tokenizer := fts.NewJiebaTokenizerWithDict([]string{"向量", "数据库", "搜索引擎"})
```

## 批量优化 / Optimize

```go
// 批量插入后重建索引以提升性能
c.Optimize(nil)
```

---

## 许可 / License

MIT
