package types

type SearchResult struct {
	DocID uint64
	Score float32
	PK    string
}
