package reranker

import (
	"sort"

	"github.com/third-apps/go-zvec/types"
)

type Reranker interface {
	Rerank(results [][]types.SearchResult, topN int) []types.SearchResult
}

type RRFParams struct {
	RankConstant int
}

func NewRRFParams(rankConstant int) *RRFParams {
	if rankConstant <= 0 {
		rankConstant = 60
	}
	return &RRFParams{RankConstant: rankConstant}
}

func (p *RRFParams) Rerank(results [][]types.SearchResult, topN int) []types.SearchResult {
	type scoredDoc struct {
		pk    string
		docID uint64
		score float64
	}

	scores := make(map[string]*scoredDoc)

	for _, list := range results {
		for rank, r := range list {
			if existing, ok := scores[r.PK]; ok {
				existing.score += 1.0 / float64(p.RankConstant+rank+1)
			} else {
				scores[r.PK] = &scoredDoc{
					pk:    r.PK,
					docID: r.DocID,
					score: 1.0 / float64(p.RankConstant+rank+1),
				}
			}
		}
	}

	var sorted []*scoredDoc
	for _, sd := range scores {
		sorted = append(sorted, sd)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	if topN > len(sorted) {
		topN = len(sorted)
	}

	results_ := make([]types.SearchResult, topN)
	for i := 0; i < topN; i++ {
		results_[i] = types.SearchResult{
			DocID: sorted[i].docID,
			Score: float32(sorted[i].score),
			PK:    sorted[i].pk,
		}
	}
	return results_
}

type WeightedParams struct {
	Weights []float64
}

func NewWeightedParams(weights []float64) *WeightedParams {
	return &WeightedParams{Weights: weights}
}

func (p *WeightedParams) Rerank(results [][]types.SearchResult, topN int) []types.SearchResult {
	type scoredDoc struct {
		pk    string
		docID uint64
		score float64
	}

	scores := make(map[string]*scoredDoc)

	normalizedResults := make([][]types.SearchResult, len(results))
	for i, list := range results {
		normalizedResults[i] = normalizeScores(list)
	}

	for i, list := range normalizedResults {
		weight := 1.0
		if i < len(p.Weights) {
			weight = p.Weights[i]
		}
		for _, r := range list {
			if existing, ok := scores[r.PK]; ok {
				existing.score += float64(r.Score) * weight
			} else {
				scores[r.PK] = &scoredDoc{
					pk: r.PK, docID: r.DocID,
					score: float64(r.Score) * weight,
				}
			}
		}
	}

	var sorted []*scoredDoc
	for _, sd := range scores {
		sorted = append(sorted, sd)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	if topN > len(sorted) {
		topN = len(sorted)
	}

	results_ := make([]types.SearchResult, topN)
	for i := 0; i < topN; i++ {
		results_[i] = types.SearchResult{
			DocID: sorted[i].docID,
			Score: float32(sorted[i].score),
			PK:    sorted[i].pk,
		}
	}
	return results_
}

func normalizeScores(results []types.SearchResult) []types.SearchResult {
	if len(results) == 0 {
		return results
	}

	var minScore, maxScore float64
	minScore = float64(results[0].Score)
	maxScore = float64(results[0].Score)
	for _, r := range results {
		s := float64(r.Score)
		if s < minScore {
			minScore = s
		}
		if s > maxScore {
			maxScore = s
		}
	}

	rangeScore := maxScore - minScore
	if rangeScore == 0 {
		normalized := make([]types.SearchResult, len(results))
		for i, r := range results {
			normalized[i] = types.SearchResult{DocID: r.DocID, Score: 1.0, PK: r.PK}
		}
		return normalized
	}

	normalized := make([]types.SearchResult, len(results))
	for i, r := range results {
		normalized[i] = types.SearchResult{
			DocID: r.DocID,
			Score: float32((float64(r.Score) - minScore) / rangeScore),
			PK:    r.PK,
		}
	}
	return normalized
}

type CallbackParams struct {
	Callback func(results [][]types.SearchResult, topN int) []types.SearchResult
}

func (p *CallbackParams) Rerank(results [][]types.SearchResult, topN int) []types.SearchResult {
	if p.Callback == nil {
		return nil
	}
	return p.Callback(results, topN)
}

func Rerank(params interface{}, results [][]types.SearchResult, topN int) []types.SearchResult {
	switch p := params.(type) {
	case *RRFParams:
		return p.Rerank(results, topN)
	case *WeightedParams:
		return p.Rerank(results, topN)
	case *CallbackParams:
		return p.Rerank(results, topN)
	default:
		rrf := NewRRFParams(60)
		return rrf.Rerank(results, topN)
	}
}
