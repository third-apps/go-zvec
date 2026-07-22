# WAL Protobuf 生成说明

本目录包含 WAL（Write-Ahead Log）的 Protobuf 定义和生成的 Go 代码。

## 文件说明

- `wal.proto` — Protobuf 消息定义（LogEntry、Doc、Value、VectorValue、SparseVectorValue）
- `wal.pb.go` — 由 protoc 自动生成的 Go 代码，**请勿手动编辑**

## 生成命令

前置依赖：

```bash
# 安装 protoc 编译器（需要 v3+ 版本）
# macOS: brew install protobuf
# Ubuntu: apt install protobuf-compiler
# Windows: 从 https://github.com/protocolbuffers/protobuf/releases 下载

# 安装 Go protoc 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

在项目根目录执行：

```bash
protoc --go_out=. --go_opt=paths=source_relative wal/proto/wal.proto
```

## 修改 Proto 定义后

1. 编辑 `wal.proto`
2. 在项目根目录执行上述生成命令
3. 提交 `wal.proto` 和 `wal.pb.go` 的变更