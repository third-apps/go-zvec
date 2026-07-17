# Go-Zvec 第一轮性能基准测试分析报告

## 1. 测试环境

### 基础环境

| 项目 | 参数 |
|---|---|
| Go版本 | go1.26.1 |
| CPU核心 | 36 Core |
| 操作系统 | Windows |
| 向量维度 | 128 |
| 文档数量 | 100000 |
| TopK | 10 |
| 查询次数 | 10000 |

---

# 2. Benchmark测试结果

| Index | 插入耗时 | 构建速度 | 搜索耗时 | QPS | 平均延迟 |
|-|-:|-:|-:|-:|-:|
| Flat | 914 ms | 109409 docs/s | 94151 ms | 106 | 9.415 ms |
| IVF | 1012 ms | 98814 docs/s | 178803 ms | 56 | 17.880 ms |
| HNSW | 135158 ms | 740 docs/s | 20423 ms | 490 | 2.042 ms |
| Vamana | 39750 ms | 2516 docs/s | 2017 ms | 4958 | 0.202 ms |
| HNSW RaBitQ | 322764 ms | 310 docs/s | 43741 ms | 229 | 4.374 ms |
| Concurrent Vamana BatchQuery | - | - | 116 ms | 85586 | 0.012 ms |

---

# 3. 总体评价
当前 go-zvec 的 ANN 搜索核心表现非常优秀。
尤其是 Vamana：
- 单线程搜索 QPS：4958
- 平均延迟：0.202ms
- 相比 HNSW：
  4958 / 490 ≈ 10倍

说明当前 Vamana 图索引结构具有：
- 较好的搜索路径优化
- 较低的图遍历成本
- 较好的 CPU Cache 利用率


当前版本已经满足：
- AI IDE代码搜索
- 本地RAG
- Agent Memory
- 文档语义搜索
等场景需求。


但是距离完整 Vector Database 还有一些关键问题。

# 4. 当前发现的问题

# 问题1：Vamana索引构建速度仍然偏慢
## 当前表现
100000 vectors
Build Time:39.75s
速度:2516 docs/s
相比：

|Index|Build速度|
|-|-:|
|Flat|109409 docs/s|
|IVF|98814 docs/s|
|HNSW|740 docs/s|
|Vamana|2516 docs/s|


Vamana：
虽然比 HNSW 快：
相比：

|Index|Build速度|
|-|-:|
|Flat|109409 docs/s|
|IVF|98814 docs/s|
|HNSW|740 docs/s|
|Vamana|2516 docs/s|

Vamana：
虽然比 HNSW 快：2516 / 740 ≈ 3.4倍
但是：距离 Flat / IVF 仍有巨大差距。

## 影响场景

AI IDE首次建立索引：
例如：300000 chunks：
预计：300000 / 2516≈119秒

约：2分钟。
对于： 首次扫描项目 可以接受。但是对于大量文件实时更新不适合。


## 原因分析

当前Vamana属于：
在线插入模式：
Add Vector  Search Neighbor  Prune Graph Add Edge

每插入一个向量：
都会执行： 图搜索  距离计算  邻居裁剪
导致：O(N logN)
---

## 优化方向
增加两种构建模式。

### 1. Realtime Insert模式

用于：IDE实时修改。
例如：文件修改  新增Embedding 快速插入HNSW


---

### 2. Batch Build模式

用于：首次建立索引。
流程：Load all vectors| Parallel Graph Construction|Graph Optimization|Finalize Index

目标：
提升：5~20倍构建速度。

# 问题2：Concurrent Vamana结果需要进一步验证
## 当前结果 
10000 queries  116ms  QPS:85586
平均： 0.012ms

即：12微秒。



## 存在问题
当前测试：
使用：
BatchQuery()
本质：
一次提交大量查询。类似数据库批处理。并不完全等同于真实用户并发。
建议增加测试指标
增加：
| 指标          | 说明   |
| ----------- | ---- |
| Concurrency | 并发数量 |
| QPS         | 吞吐   |
| P50         | 中位延迟 |
| P95         | 高延迟  |
| P99         | 极端延迟 |

问题3：IVF索引性能异常
当前结果
IVF
QPS:56
甚至低于：Flat:106 QPS
原因分析
当前参数：nList=16
对于：100000 vectors明显不足。正常：应该：nList ≈ sqrt(N)
例如： 100000   sqrt(100000)≈316
优化方向
测试：
nList=128
nList=256
nList=512
同时增加：nProbe
例如：
nList=256
nProbe=16

问题4：缺少Recall评估
当前测试问题
目前只测试：速度。
但是ANN核心指标：不是单纯QPS。
需要：速度 + 准确率
建议增加
使用：Flat作为Ground Truth。
例如：Flat搜索：
Top10:A B C D E

Vamana：A B C F G
计算：Recall@10= 命中数量 / 10
输出：Vamana Recall@10 = 98%

问题5：缺少内存占用测试
当前没有统计：Index占用内存。
对于Vector Database 非常重要。

需要增加
输出：
Vector Memory
Graph Memory
Metadata Memory
Total Memory

例如：
100万：
768维：
理论：1000000 * 768 * 4≈3GB

实际：还包括：Graph。

问题6：RaBitQ测试规模不足
当前结果
HNSW RaBitQ
Build:322s
QPS:229
表现较差。

原因
RaBitQ优势：不是速度。
而是：降低内存。
50K~100K规模：
内存压力不存在。
所以：量化反而增加计算。

建议
RaBitQ应该测试：
1M vectors
10M vectors
100M vectors
重点观察：
内存下降比例
Recall变化
查询速度

问题7：缺少真实AI IDE场景测试

当前：随机向量。不能完全代表：代码搜索。
建议增加数据模型
模拟：

代码Chunk:

Metadata:

{
"file":"payment/order.go",
"language":"go",
"function":"CreateOrder",
"line":120
}

测试：搜索支付订单逻辑

验证：
实际RAG效果。

5. 当前版本评分
   ANN Engine

评分：
9.2 / 10

原因：
Vamana搜索性能优秀
扩展性良好
延迟低
AI IDE Vector Engine

评分：8 / 10

优势：
搜索性能足够
不足：
构建速度
增量更新
持久化
Metadata完整

Vector Database

评分：6 / 10
缺少：
Collection管理
Segment架构
WAL
Storage Layer
Metadata Engine
Hybrid Search
6. 下一阶段优化优先级
   P0 必须解决
   项目	优先级
   Batch Build模式	★★★★★
   Recall测试	★★★★★
   Memory统计	★★★★★
   真实并发测试	★★★★★
   P1 建议增加
   项目	优先级
   Segment架构	★★★★
   Metadata系统	★★★★
   持久化格式	★★★★
   增量更新	★★★★
   P2 长期方向
   项目
   Hybrid Search
   BM25
   Rerank
   分布式部署
   Replication
7. 最终结论

第一轮测试证明：

go-zvec 的核心ANN搜索能力已经达到较高水平。

尤其：

Vamana:

100000 vectors

4958 QPS

0.202ms latency

已经足够支撑：

AI IDE
本地知识库
Agent Memory

当前最大的瓶颈不是搜索。

而是：

Index Build

+
Database Engineering

下一阶段重点应该从：

优化搜索速度

转向：

构建完整Vector Database架构

重点：

Batch Build
Segment
Persistence
Metadata
Hybrid Search