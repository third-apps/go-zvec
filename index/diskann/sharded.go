package diskann

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

type ShardedDiskAnnIndex struct {
	shards []*DiskAnnIndex
	mask   uint64
}

func NewShardedDiskAnnIndex(numShards, dimension int, metricType types.MetricType,
	maxDegree, searchList int, alpha float64, saturateGraph bool) *ShardedDiskAnnIndex {
	if numShards <= 0 || (numShards&(numShards-1)) != 0 {
		numShards = 16
	}
	shards := make([]*DiskAnnIndex, numShards)
	for i := range shards {
		shards[i] = NewDiskAnnIndex(dimension, metricType, maxDegree, searchList, alpha, saturateGraph)
	}
	return &ShardedDiskAnnIndex{
		shards: shards,
		mask:   uint64(numShards - 1),
	}
}

func (s *ShardedDiskAnnIndex) shardID(pk string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(pk))
	return h.Sum64() & s.mask
}

func (s *ShardedDiskAnnIndex) shard(pk string) *DiskAnnIndex {
	return s.shards[s.shardID(pk)]
}

func (s *ShardedDiskAnnIndex) Add(vector []float32, pk string) uint64 {
	return s.shard(pk).Add(vector, pk)
}

func (s *ShardedDiskAnnIndex) Search(query []float32, topK int) []types.SearchResult {
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

func (s *ShardedDiskAnnIndex) SearchWithFilter(query []float32, topK int, filterFn func(pk string) bool) []types.SearchResult {
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

func (s *ShardedDiskAnnIndex) Delete(pk string) bool {
	return s.shard(pk).Delete(pk)
}

func (s *ShardedDiskAnnIndex) Size() int {
	total := 0
	for _, sh := range s.shards {
		total += sh.Size()
	}
	return total
}

func (s *ShardedDiskAnnIndex) Close() error {
	var errs []error
	for _, sh := range s.shards {
		if err := sh.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *ShardedDiskAnnIndex) MemoryBytes() uint64 {
	total := uint64(0)
	for _, sh := range s.shards {
		total += sh.MemoryBytes()
	}
	return total
}

func (s *ShardedDiskAnnIndex) Save(w io.Writer) error {
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

func (s *ShardedDiskAnnIndex) Load(r io.Reader) error {
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

func (s *ShardedDiskAnnIndex) SetPath(path string) {
	for i, sh := range s.shards {
		sh.SetPath(fmt.Sprintf("%s_%d", path, i))
	}
}

func (s *ShardedDiskAnnIndex) InitStorage() error {
	var errs []error
	for _, sh := range s.shards {
		if err := sh.InitStorage(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *ShardedDiskAnnIndex) Dimension() int {
	if len(s.shards) == 0 {
		return 0
	}
	return s.shards[0].Dimension()
}

func (s *ShardedDiskAnnIndex) Sync() error {
	var errs []error
	for _, sh := range s.shards {
		if err := sh.Sync(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
