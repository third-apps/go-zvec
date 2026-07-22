package hnsw

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"sort"
	"sync"

	"github.com/third-apps/go-zvec/persist"
	"github.com/third-apps/go-zvec/types"
)

type ShardedHNSWIndex struct {
	shards []*HNSWIndex
	mask   uint64
}

func NewShardedHNSWIndex(numShards, dimension int, metricType types.MetricType, m, efConstruction int) *ShardedHNSWIndex {
	if numShards <= 0 || (numShards&(numShards-1)) != 0 {
		numShards = 16
	}
	shards := make([]*HNSWIndex, numShards)
	for i := range shards {
		shards[i] = NewHNSWIndex(dimension, metricType, m, efConstruction)
	}
	return &ShardedHNSWIndex{
		shards: shards,
		mask:   uint64(numShards - 1),
	}
}

func (s *ShardedHNSWIndex) shardID(pk string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(pk))
	return h.Sum64() & s.mask
}

func (s *ShardedHNSWIndex) shard(pk string) *HNSWIndex {
	return s.shards[s.shardID(pk)]
}

func (s *ShardedHNSWIndex) Add(vector []float32, pk string) uint64 {
	return s.shard(pk).Add(vector, pk)
}

func (s *ShardedHNSWIndex) Search(query []float32, topK int) []types.SearchResult {
	if topK <= 0 {
		return nil
	}

	results := make([][]types.SearchResult, len(s.shards))
	var wg sync.WaitGroup
	for i, sh := range s.shards {
		wg.Add(1)
		i, sh := i, sh
		go func() {
			defer wg.Done()
			results[i] = sh.Search(query, topK)
		}()
	}
	wg.Wait()

	total := 0
	for _, r := range results {
		total += len(r)
	}
	all := make([]types.SearchResult, 0, total)
	for _, r := range results {
		all = append(all, r...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Score > all[j].Score
	})
	if len(all) > topK {
		all = all[:topK]
	}
	return all
}

func (s *ShardedHNSWIndex) SearchWithFilter(query []float32, topK int, filterFn func(pk string) bool) []types.SearchResult {
	if topK <= 0 {
		return nil
	}

	results := make([][]types.SearchResult, len(s.shards))
	var wg sync.WaitGroup
	for i, sh := range s.shards {
		wg.Add(1)
		i, sh := i, sh
		go func() {
			defer wg.Done()
			results[i] = sh.SearchWithFilter(query, topK, filterFn)
		}()
	}
	wg.Wait()

	total := 0
	for _, r := range results {
		total += len(r)
	}
	all := make([]types.SearchResult, 0, total)
	for _, r := range results {
		all = append(all, r...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Score > all[j].Score
	})
	if len(all) > topK {
		all = all[:topK]
	}
	return all
}

func (s *ShardedHNSWIndex) Delete(pk string) bool {
	return s.shard(pk).Delete(pk)
}

func (s *ShardedHNSWIndex) Size() int {
	total := 0
	for _, sh := range s.shards {
		total += sh.Size()
	}
	return total
}

func (s *ShardedHNSWIndex) Close() error {
	var errs []error
	for _, sh := range s.shards {
		if err := sh.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *ShardedHNSWIndex) MemoryBytes() uint64 {
	total := uint64(0)
	for _, sh := range s.shards {
		total += sh.MemoryBytes()
	}
	return total
}

func (s *ShardedHNSWIndex) Save(w io.Writer) error {
	if err := persist.WriteUint64(w, uint64(len(s.shards))); err != nil {
		return err
	}

	for _, sh := range s.shards {
		var buf bytes.Buffer
		if err := sh.Save(&buf); err != nil {
			return err
		}
		if err := persist.WriteUint64(w, uint64(buf.Len())); err != nil {
			return err
		}
		if _, err := w.Write(buf.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func (s *ShardedHNSWIndex) Load(r io.Reader) error {
	br := bufio.NewReader(r)

	numShards, err := persist.ReadUint64(br)
	if err != nil {
		return err
	}
	if int(numShards) != len(s.shards) {
		return fmt.Errorf("shard count mismatch: expected %d, got %d", len(s.shards), numShards)
	}

	for _, sh := range s.shards {
		dataLen, err := persist.ReadUint64(br)
		if err != nil {
			return err
		}
		if dataLen > 1<<30 {
			return fmt.Errorf("shard data too large: %d bytes", dataLen)
		}
		data := make([]byte, dataLen)
		if _, err := io.ReadFull(br, data); err != nil {
			return err
		}
		if err := sh.Load(bytes.NewReader(data)); err != nil {
			return err
		}
	}
	return nil
}

func (s *ShardedHNSWIndex) SetEF(ef int) {
	for _, sh := range s.shards {
		sh.SetEF(ef)
	}
}

func (s *ShardedHNSWIndex) Dimension() int {
	if len(s.shards) == 0 {
		return 0
	}
	return s.shards[0].Dimension()
}
