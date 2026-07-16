package types

type DocOperator uint32

const (
	DocOpInsert DocOperator = 0
	DocOpUpsert DocOperator = 1
	DocOpUpdate DocOperator = 2
	DocOpDelete DocOperator = 3
)

func (o DocOperator) String() string {
	switch o {
	case DocOpInsert:
		return "insert"
	case DocOpUpsert:
		return "upsert"
	case DocOpUpdate:
		return "update"
	case DocOpDelete:
		return "delete"
	default:
		return "unknown"
	}
}

type CompareOp uint32

const (
	CompareOpEQ            CompareOp = 0
	CompareOpNE            CompareOp = 1
	CompareOpLT            CompareOp = 2
	CompareOpLE            CompareOp = 3
	CompareOpGT            CompareOp = 4
	CompareOpGE            CompareOp = 5
	CompareOpLike          CompareOp = 6
	CompareOpContainAll    CompareOp = 7
	CompareOpContainAny    CompareOp = 8
	CompareOpNotContainAll CompareOp = 9
	CompareOpNotContainAny CompareOp = 10
	CompareOpIsNull        CompareOp = 11
	CompareOpIsNotNull     CompareOp = 12
	CompareOpHasPrefix     CompareOp = 13
	CompareOpHasSuffix     CompareOp = 14
)

type RelationOp uint32

const (
	RelationOpAND RelationOp = 0
	RelationOpOR  RelationOp = 1
)

type FileFormat uint32

const (
	FileFormatIPC     FileFormat = 0
	FileFormatParquet FileFormat = 1
)

type ColumnOp uint32

const (
	ColumnOpAdd   ColumnOp = 0
	ColumnOpAlter ColumnOp = 1
	ColumnOpDrop  ColumnOp = 2
)

type StorageType uint32

const (
	StorageTypeNone       StorageType = 0
	StorageTypeMMAP       StorageType = 1
	StorageTypeMemory     StorageType = 2
	StorageTypeBufferPool StorageType = 3
)
