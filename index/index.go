package index

import (
	"io"

	"github.com/third-apps/go-zvec/types"
)

type Index interface {
	Search(query []float32, topK int) []types.SearchResult
	SearchWithFilter(query []float32, topK int, filterFn func(pk string) bool) []types.SearchResult
	Add(vector []float32, pk string) uint64
	Delete(pk string) bool
	Size() int
	Close() error
	MemoryBytes() uint64
	Save(w io.Writer) error
	Load(r io.Reader) error
}

type BatchBuilder interface {
	BatchBuild(vectors [][]float32, pks []string)
}
