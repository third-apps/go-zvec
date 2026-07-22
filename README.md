<div align="center">

# Go-Zvec

**纯 Go 向量数据库 / Pure Go Vector Database**

Alibaba Zvec C++ 引擎的纯 Go 移植版，零 CGo 依赖

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg?style=flat-square)](./LICENSE)
[![Zero CGo](https://img.shields.io/badge/Zero-CGo-green?style=flat-square)](https://go.dev/)
[![CI](https://github.com/third-apps/go-zvec/actions/workflows/ci.yml/badge.svg)](https://github.com/third-apps/go-zvec/actions/workflows/ci.yml)

</div>

<p align="center">
  🚀 <a href="#-快速开始--quick-start"><strong>Quick Start</strong></a> |
  📊 <a href="#-性能测试--benchmark"><strong>Benchmark</strong></a> |
  📚 <a href="#-包结构--package-structure"><strong>Package Structure</strong></a> |
  🔍 <a href="#-全文搜索--full-text-search"><strong>FTS</strong></a> |
  🧮 <a href="#-多查询--重排序--multiquery--rerank"><strong>MultiQuery</strong></a>
</p>

---

**Go-Zvec** 是一个开源的进程内向量数据库 —— 轻量、极速，专为嵌入式场景设计。作为 Alibaba Zvec C++ 引擎的纯 Go 移植版，它继承了生产级的低延迟和可扩展相似性搜索能力，同时零 CGo 依赖，交叉编译无忧。

## 💫 特性

- **极速搜索**：毫秒级检索海量向量
- **零 CGo 依赖**：纯 Go 实现，交叉编译零配置
- **稠密 + 稀疏向量**：支持稠密/稀疏嵌入、多向量查询、从内存到磁盘的多种索引类型
- **全文搜索 (FTS)**：原生关键词全文搜索，支持自然语言和结构化查询语法
- **混合检索**：单次查询融合向量相似性、全文搜索和结构化过滤
- **元数据过滤**：结构化字段索引 + 过滤条件（Eq/In/Gt/Lt），搜索时预过滤
- **持久化存储**：WAL 预写日志（Protobuf 二进制格式 + CRC32 校验）+ 快照恢复，进程崩溃或断电数据不丢失
- **类型安全参数**：每种索引类型独立的参数结构，编译期类型检查
- **并发安全**：读写锁保护，支持并发读取；索引并发写入，文档分片存储
- **SIMD 优化**：距离计算自动检测 AVX2/NEON，8x 循环展开 + 多累加器
- **随处运行**：作为进程内库，Go-Zvec 随你的代码运行 —— 服务器、CLI 工具、甚至边缘设备

## 📦 安装

```bash
go get github.com/third-apps/go-zvec
```

### ✅ 支持平台

- Linux (x86_64, ARM64)
- macOS (ARM64, x86_64)
- Windows (x86_64)

## ⚡ 快速开始 / Quick Start

```go
package main

import (
    "fmt"
    "github.com/third-apps/go-zvec"
    "github.com/third-apps/go-zvec/doc"
    "github.com/third-apps/go-zvec/schema"
    "github.com/third-apps/go-zvec/types"
)

func main() {
    s := schema.NewCollectionSchema("demo")
    vecF := schema.NewFieldSchema("vector", types.DataTypeVectorFP32, false, 4)
    vecF.SetIndexParams(zvec.NewHNSWParams(types.MetricTypeCosine, 16, 200))
    s.AddField(vecF)

    c, _ := zvec.CreateAndOpen("./data", s, nil)
    defer c.Close()

    d := doc.NewDoc("doc1")
    d.SetVectorFP32Field("vector", []float32{0.1, 0.2, 0.3, 0.4})
    c.Insert([]*doc.Doc{d})

    results, _ := c.Query(&zvec.SearchQuery{
        Target: zvec.QueryTarget{
            FieldName: "vector",
            Vector:    &zvec.VectorClause{QueryVector: []float32{0.4, 0.3, 0.3, 0.1}},
        },
        TopK: 5,
    })
    fmt.Println(results[0]["id"], results[0]["score"])
}
```

---

## 📊 性能测试 / Benchmark

运行 `examples/benchmark/` 下的完整基准测试程序：

```bash
go run ./examples/benchmark/
```

测试环境：Go 1.26.1, Windows x86_64, 36 CPU cores, 128维归一化随机向量, Cosine 度量, topK=10, 100 queries。

### 参考结果 / Reference Results (50K vectors)

| 索引类型 | 插入速度 | 搜索 QPS | 平均延迟 |
|---|---|---|---|
| **Flat** | ~119K docs/s | ~210 | ~4.8ms |
| **IVF** (nList=16) | ~129K docs/s | ~31 | ~32ms |
| **HNSW** (M=16, efConstruction=200) | ~820 docs/s | ~545 | ~1.8ms |
| **Vamana** (maxDegree=16, alpha=1.2) | ~2.8K docs/s | ~6,153 | ~0.16ms |
| **Concurrent Vamana** (BatchQuery, 100 goroutines) | — | **~80K** | ~0.01ms |

### 参考结果 / Reference Results (10K vectors)

| 索引类型 | 插入速度 | 搜索 QPS | 平均延迟 |
|---|---|---|---|
| **Flat** | ~103K docs/s | ~705 | ~1.4ms |
| **HNSW RaBitQ** (M=16, totalBits=1) | ~466 docs/s | ~284 | ~3.5ms |

### 参考结果 / Reference Results (1M vectors)

运行 `examples/benchmark_1m/` 下的 1M 大规模基准测试：

```bash
go run ./examples/benchmark_1m/
```

| 索引类型 | 插入速度 | 搜索 QPS | 平均延迟 | 内存占用 | Recall@10 |
|---|---|---|---|---|---|
| **HNSW** (M=16, efConstruction=200) | — | — | — | — | — |
| **HNSW RaBitQ** (M=16, totalBits=1) | — | — | — | — | — |
| **Vamana** (maxDegree=16, alpha=1.2) | — | — | — | — | — |

> ⚠️ 以上 1M 数据待测试后填充。运行测试程序约需 20-30 分钟。

> ⚠️ 以上数据为真实测试结果，实际性能取决于硬件、向量分布和查询模式。

---

## 🗂️ 索引类型 / Index Types

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

## 🧮 量化器 / Quantizers

| 量化器 | 说明 | 状态 |
|---|---|---|
| **FP16** | 半精度浮点量化 | ✅ |
| **Int8** | 8-bit 整数量化 + 随机旋转 | ✅ |
| **Int4** | 4-bit 整数量化 + 随机旋转 | ✅ |
| **RaBitQ** | 随机位量化，1-bit + 范数 | ✅ |
| **PQ** | 乘积量化，K-Means 子空间聚类 | ✅ |

## 📏 距离度量 / Distance Metrics

| 度量 | 说明 | 状态 |
|---|---|---|
| **L2** | 欧几里得距离 | ✅ |
| **IP** | 内积 | ✅ |
| **Cosine** | 余弦距离 | ✅ |
| **MIPSL2** | 最大内积搜索 + L2 | ✅ |
| **SparseIP** | 稀疏向量内积 | ✅ |

## 🔍 全文搜索 / Full-Text Search

| 功能 | 说明 | 状态 |
|---|---|---|
| **StandardTokenizer** | Unicode 字母/数字分词 | ✅ |
| **WhitespaceTokenizer** | 空格分词 | ✅ |
| **JiebaTokenizer** | 中文正向最大匹配分词（内置词典 + 自定义词典） | ✅ |
| **BM25 评分** | 经典 BM25 相关性评分 | ✅ |
| **查询语法** | AND / OR / NOT / 短语查询 / 括号分组 | ✅ |
| **SearchAdvanced** | 基于语法树的进阶搜索 | ✅ |

```go
// 创建 FTS 索引
ftsField := schema.NewFieldSchema("text", types.DataTypeString, false, 0)
ftsField.SetIndexParams(zvec.NewFTSParams("standard", nil, ""))
s.AddField(ftsField)

// 使用查询语法搜索
results, _ := c.FTSQuery("text", `"hello world" AND go`, 10)
```

### 中文分词 / Chinese Tokenization

```go
// Schema 配置中文分词器
ftsField := schema.NewFieldSchema("content", types.DataTypeString, false, 0)
ftsField.SetIndexParams(zvec.NewFTSParams("jieba", nil, ""))

// 或直接使用 fts 包
import "github.com/third-apps/go-zvec/fts"

tokenizer := fts.NewJiebaTokenizer()

// 自定义词典
tokenizer := fts.NewJiebaTokenizerWithDict([]string{"向量", "数据库", "搜索引擎"})
```

## 🛠️ 集合操作 / Collection Operations

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
| **BatchBuild** | 批量构建索引（Vamana 并行构建） | ✅ |
| **BatchQuery** | 并发批量搜索 | ✅ |
| **HybridQuery** | 向量 + 全文混合检索 | ✅ |
| **Save / LoadCollection** | 持久化保存与加载 | ✅ |
| **Snapshot / Recover** | 全量快照 + WAL 回放恢复 | ✅ |
| **Compact** | 后台压缩，清理已删除文档 | ✅ |

## 🔎 过滤表达式 / Filter Expressions

| 操作符 | 说明 | 状态 |
|---|---|---|
| `>=` `<=` `>` `<` `==` `!=` | 数值/字符串比较 | ✅ |
| `LIKE` | 通配符模式匹配（`*` 和 `?`） | ✅ |
| `IS_NULL` / `IS_NOT_NULL` | 空值检查 | ✅ |
| `HAS_PREFIX` / `HAS_SUFFIX` | 前缀/后缀匹配 | ✅ |
| `CONTAIN_ALL` / `CONTAIN_ANY` | 包含全部/任一值 | ✅ |
| `NOT_CONTAIN_ALL` / `NOT_CONTAIN_ANY` | 不包含全部/任一值 | ✅ |

## 🏷️ 元数据过滤 / Metadata Filter

```go
// 创建带元数据字段的集合
langF := schema.NewFieldSchema("language", types.DataTypeString, false, 0)
langF.SetIndexParams(zvec.NewInvertParams(false, false))

// 搜索时过滤
mf := zvec.NewMetadataFilter().WhereEq("language", "go")
results, _ := c.Query(&zvec.SearchQuery{
    Target:    zvec.QueryTarget{FieldName: "vector", Vector: &zvec.VectorClause{QueryVector: q}},
    TopK:      10,
    MetaFilter: mf,
})
```

| 条件 | 说明 | 状态 |
|---|---|---|
| `WhereEq` | 字符串/整数/布尔等值匹配 | ✅ |
| `WhereIn` / `WhereIntIn` | 字符串/整数集合包含 | ✅ |
| `WhereIntGt` / `WhereIntLt` | 整数范围过滤 | ✅ |
| `WhereIntGte` / `WhereIntLte` | 整数范围过滤（含边界） | ✅ |
| `WhereIntNe` | 整数不等于 | ✅ |
| `WhereBoolEq` | 布尔值过滤 | ✅ |
| `WhereExists` | 字段存在性检查 | ✅ |

## 💾 存储 / Storage

| 类型 | 说明 | 状态 |
|---|---|---|
| **FileStorage** | 文件存储 | ✅ |
| **MemoryStorage** | 内存存储 | ✅ |
| **MMAPStorage** | Windows 内存映射文件 | ✅ |
| **WAL** | Protobuf 二进制预写日志，CRC32 校验，支持批量 fsync | ✅ |
| **Segment** | 分段文档管理 | ✅ |
| **Snapshot** | 全量快照保存/加载 | ✅ |
| **Recover** | WAL 回放增量恢复 | ✅ |

### 持久化示例 / Persistence Example

```go
// 保存到磁盘
c.Save()

// 从磁盘加载
c, _ = zvec.LoadCollection("./data", nil)

// 快照 + 清空 WAL
c.Snapshot()

// WAL 回放恢复
c.Recover()
```

---

## 🧮 多查询 + 重排序 / MultiQuery + Rerank

```go
results, _ := c.MultiQuery(&zvec.MultiQuery{
    SubQueries: []zvec.SubQuery{
        {Target: zvec.QueryTarget{FieldName: "v1", Vector: &zvec.VectorClause{QueryVector: q1}}},
        {Target: zvec.QueryTarget{FieldName: "v2", Vector: &zvec.VectorClause{QueryVector: q2}}},
    },
    TopK: 10,
    Rerank: zvec.RerankParams{Type: zvec.RerankTypeRRF, RRFConstant: 60},
})
```

## 🔀 混合检索 / Hybrid Query

```go
// 向量 + 全文混合搜索
results, _ := c.HybridQuery("vector", q, "text", "hello world", 10)
```

## 🔄 批量优化 / Optimize

```go
// 批量插入后重建索引以提升性能
c.Optimize(nil)
```

---

## 📚 包结构 / Package Structure

```
go-zvec/
├── zvec.go              # Public API (Init/CreateAndOpen/Open/Shutdown/LoadCollection)
├── collection/          # Collection CRUD, index management, query, optimize, persistence
│   ├── interfaces.go    # FTSIndexer, InvertIndexer, MetaIndexer, WALWriter, SegmentManager
│   └── sharded_map.go   # Sharded docIndex/docIDToPK (16 shards, per-shard RWMutex)
├── config/              # Global configuration
├── doc/                 # Document model (fields, vectors, validation)
├── fts/                 # Full-text search (tokenizer, BM25, query parser, Jieba)
├── index/
│   ├── index.go         # Index & BatchBuilder interfaces
│   ├── diskann/         # Disk-based Vamana with MMAP storage + LRU cache
│   ├── flat/            # Brute-force flat index
│   ├── flat_sparse/     # Sparse vector brute-force index
│   ├── hnsw/            # HNSW graph index
│   ├── hnsw_rabitq/     # RaBitQ quantized HNSW index
│   ├── invert/          # Inverted index for scalar fields
│   ├── ivf/             # IVF cluster index (configurable nprobe)
│   ├── vamana/          # Vamana filtered graph index
│   └── param/           # Type-safe index params (HNSWParams, IVFParams, ...) + IndexConfig
├── metadata/            # Metadata index (string/int64/bool field indexing + match)
├── metric/              # Distance functions (L2, IP, Cosine, SparseIP) + SIMD optimization
├── persist/             # Binary serialization (Save/Load index files)
├── quantizer/           # FP16 / Int8 / Int4 / RaBitQ / PQ quantizers
├── query/               # Query types (SearchQuery, MultiQuery, GroupBy, MetadataFilter)
├── reranker/            # RRF / Weighted / Callback reranking
├── schema/              # FieldSchema / CollectionSchema (IndexConfig interface)
├── segment/             # Multi-segment document management
├── status/              # Status & Result[T] error handling
├── storage/             # FileStorage / MemoryStorage / MMAPStorage
├── types/               # DataType, IndexType, MetricType, SearchResult, etc.
├── wal/                 # Write-ahead log (Append/Replay/Truncate, Protobuf binary + CRC32)
│   └── proto/           # Protobuf definitions (wal.proto + generated wal.pb.go)
└── examples/            # Usage examples
```

---

## 🏗️ 架构 / Architecture

```
                    ┌─────────────────────────┐
                    │       Public API         │
                    │   zvec.go (Init/Open)    │
                    └────────────┬────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │      Collection          │
                    │  CRUD / Query / Persist  │
                    │  Sharded DocIndex (16)   │
                    └──┬─────┬─────┬──────────┘
                       │     │     │
          ┌────────────▼┐  ┌─▼──────▼────────┐
          │  ANN Engine  │  │  Metadata Engine │
          │ HNSW/Vamana  │  │  Invert/FTS/Meta│
          │ IVF/DiskAnn  │  │  BM25/Filter    │
          └──────┬───────┘  └─────────────────┘
                 │
          ┌──────▼───────┐
          │ Storage Layer │
          │ WAL/Segment   │
          │ Persist/MMAP  │
          └──────────────┘
```

---

## ❤️ Contributing

We welcome and appreciate contributions from the community! Whether you're fixing a bug, adding a feature, or improving documentation, your help makes Go-Zvec better for everyone.

## 📄 许可 / License

MIT
