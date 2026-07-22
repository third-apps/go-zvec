package fts

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

type Tokenizer interface {
	Tokenize(text string) []string
}

type SearchResult struct {
	DocID   uint64
	Score   float64
	DocText string
}

type BooleanSearchResult struct {
	DocID   uint64
	Score   int
	DocText string
}

var builderPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder)
	},
}

func getBuilder() *strings.Builder {
	b := builderPool.Get().(*strings.Builder)
	b.Reset()
	return b
}

func putBuilder(b *strings.Builder) {
	builderPool.Put(b)
}

type StandardTokenizer struct{}

func NewStandardTokenizer() *StandardTokenizer {
	return &StandardTokenizer{}
}

func (t *StandardTokenizer) Tokenize(text string) []string {
	var tokens []string
	current := getBuilder()
	defer putBuilder(current)

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

type WhitespaceTokenizer struct{}

func NewWhitespaceTokenizer() *WhitespaceTokenizer {
	return &WhitespaceTokenizer{}
}

func (t *WhitespaceTokenizer) Tokenize(text string) []string {
	return strings.Fields(text)
}

type LowercaseFilter struct{}

func NewLowercaseFilter() *LowercaseFilter {
	return &LowercaseFilter{}
}

func (f *LowercaseFilter) Filter(tokens []string) []string {
	result := make([]string, len(tokens))
	for i, t := range tokens {
		result[i] = strings.ToLower(t)
	}
	return result
}

type Posting struct {
	DocID     uint64
	Positions []int
	Count     int
}

type InvertedIndex struct {
	mu        sync.RWMutex
	dict      map[string][]Posting
	totalDocs int
	docIDs    map[uint64]struct{}
	docTokens map[uint64][]string
}

func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		dict:      make(map[string][]Posting),
		docIDs:    make(map[uint64]struct{}),
		docTokens: make(map[uint64][]string),
	}
}

func (idx *InvertedIndex) AddDocument(docID uint64, tokens []string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.docIDs[docID]; exists {
		idx.removeDocumentLocked(docID)
	}
	idx.docIDs[docID] = struct{}{}
	idx.totalDocs++

	positions := make(map[string][]int)
	for pos, token := range tokens {
		positions[token] = append(positions[token], pos)
	}

	uniqueTokens := make([]string, 0, len(positions))
	for token, posList := range positions {
		uniqueTokens = append(uniqueTokens, token)
		idx.dict[token] = append(idx.dict[token], Posting{
			DocID:     docID,
			Positions: posList,
			Count:     len(posList),
		})
	}
	idx.docTokens[docID] = uniqueTokens

}

func (idx *InvertedIndex) Search(token string) []Posting {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.dict[strings.ToLower(token)]
}

func (idx *InvertedIndex) TotalDocs() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.totalDocs
}

func (idx *InvertedIndex) DocFreq(token string) int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.dict[strings.ToLower(token)])
}

func (idx *InvertedIndex) IDF(token string, totalDocs int) float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	lower := strings.ToLower(token)
	docFreq := len(idx.dict[lower])
	return math.Log((float64(totalDocs) - float64(docFreq) + 0.5) / (float64(docFreq) + 0.5))
}

func (idx *InvertedIndex) RemoveDocument(docID uint64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeDocumentLocked(docID)
}

func (idx *InvertedIndex) removeDocumentLocked(docID uint64) {
	tokens, ok := idx.docTokens[docID]
	if !ok {
		return
	}

	for _, token := range tokens {
		postings := idx.dict[token]
		filtered := make([]Posting, 0, len(postings))
		for _, p := range postings {
			if p.DocID != docID {
				filtered = append(filtered, p)
			}
		}
		if len(filtered) == 0 {
			delete(idx.dict, token)
		} else {
			idx.dict[token] = filtered
		}
	}

	delete(idx.docTokens, docID)
	if idx.totalDocs > 0 {
		idx.totalDocs--
	}
	delete(idx.docIDs, docID)

	if idx.totalDocs > 0 && idx.totalDocs < len(idx.docIDs)/2 {
		idx.compactMaps()
	}
}

func (idx *InvertedIndex) compactMaps() {
	newDocIDs := make(map[uint64]struct{}, idx.totalDocs)
	newDocTokens := make(map[uint64][]string, idx.totalDocs)
	for k, v := range idx.docIDs {
		newDocIDs[k] = v
	}
	for k, v := range idx.docTokens {
		newDocTokens[k] = v
	}
	idx.docIDs = newDocIDs
	idx.docTokens = newDocTokens
}

type BM25Scorer struct {
	k1        float64
	b         float64
	avgDocLen float64
}

func NewBM25Scorer() *BM25Scorer {
	return &BM25Scorer{
		k1: 1.2,
		b:  0.75,
	}
}

func (s *BM25Scorer) SetAvgDocLen(v float64) {
	s.avgDocLen = v
}

func (s *BM25Scorer) Score(docID uint64, docLen int, termFreq int, docFreq int, totalDocs int) float64 {
	idf := math.Log((float64(totalDocs) - float64(docFreq) + 0.5) / (float64(docFreq) + 0.5))
	return s.ScoreWithIDF(docLen, termFreq, idf)
}

func (s *BM25Scorer) ScoreWithIDF(docLen int, termFreq int, idf float64) float64 {
	tf := float64(termFreq) * (s.k1 + 1.0)
	avgLen := s.avgDocLen
	if avgLen == 0 {
		avgLen = 1
	}
	tf /= float64(termFreq) + s.k1*(1.0-s.b+s.b*float64(docLen)/avgLen)
	return idf * tf
}

type FTSIndex struct {
	mu          sync.RWMutex
	tokenizer   Tokenizer
	inverted    *InvertedIndex
	scorer      *BM25Scorer
	docTexts    map[uint64]string
	docLens     map[uint64]int
	totalDocLen int
	docCount    int
}

func NewFTSIndex(tokenizer Tokenizer) *FTSIndex {
	return &FTSIndex{
		tokenizer: tokenizer,
		inverted:  NewInvertedIndex(),
		scorer:    NewBM25Scorer(),
		docTexts:  make(map[uint64]string),
		docLens:   make(map[uint64]int),
	}
}

func (idx *FTSIndex) Index(docID uint64, text string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	tokens := idx.tokenizer.Tokenize(text)

	if _, exists := idx.docLens[docID]; exists {
		oldLen := idx.docLens[docID]
		idx.totalDocLen -= oldLen
		idx.docCount--
	}

	idx.inverted.AddDocument(docID, tokens)
	idx.docTexts[docID] = text
	idx.docLens[docID] = len(tokens)
	idx.totalDocLen += len(tokens)
	idx.docCount++
	if idx.docCount > 0 {
		idx.scorer.SetAvgDocLen(float64(idx.totalDocLen) / float64(idx.docCount))
	}
}

func (idx *FTSIndex) Delete(docID uint64) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	docLen := idx.docLens[docID]
	idx.inverted.RemoveDocument(docID)
	delete(idx.docTexts, docID)
	delete(idx.docLens, docID)
	idx.totalDocLen -= docLen
	idx.docCount--
	if idx.docCount > 0 {
		idx.scorer.SetAvgDocLen(float64(idx.totalDocLen) / float64(idx.docCount))
	} else {
		idx.scorer.SetAvgDocLen(0)
	}

	if idx.docCount > 0 && idx.docCount < len(idx.docTexts)/2 {
		idx.compactMaps()
	}
}

func (idx *FTSIndex) compactMaps() {
	newTexts := make(map[uint64]string, idx.docCount)
	newLens := make(map[uint64]int, idx.docCount)
	for k, v := range idx.docTexts {
		newTexts[k] = v
	}
	for k, v := range idx.docLens {
		newLens[k] = v
	}
	idx.docTexts = newTexts
	idx.docLens = newLens
}

func (idx *FTSIndex) Search(query string, topK int) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	queryTokens := idx.tokenizer.Tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	totalDocs := idx.inverted.TotalDocs()
	if totalDocs == 0 {
		return nil
	}

	scores := make(map[uint64]float64)
	docTerms := make(map[uint64]map[string]int)

	for _, token := range queryTokens {
		postings := idx.inverted.Search(token)
		for _, p := range postings {
			if docTerms[p.DocID] == nil {
				docTerms[p.DocID] = make(map[string]int)
			}
			docTerms[p.DocID][token] += p.Count
		}
	}

	for docID, terms := range docTerms {
		docLen := idx.docLens[docID]
		var totalScore float64
		for token, tf := range terms {
			idf := idx.inverted.IDF(token, totalDocs)
			totalScore += idx.scorer.ScoreWithIDF(docLen, tf, idf)
		}
		scores[docID] = totalScore
	}

	type scoredDoc struct {
		docID uint64
		score float64
		text  string
	}
	var results []scoredDoc
	for docID, score := range scores {
		results = append(results, scoredDoc{
			docID: docID, score: score, text: idx.docTexts[docID],
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > len(results) {
		topK = len(results)
	}

	out := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		out[i] = SearchResult{results[i].docID, results[i].score, results[i].text}
	}
	return out
}

type QueryOp int

const (
	OpAND QueryOp = 0
	OpOR  QueryOp = 1
)

func (idx *FTSIndex) SearchBoolean(query string, op QueryOp, topK int) []BooleanSearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	queryTokens := idx.tokenizer.Tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	docMatches := make(map[uint64]int)
	for _, token := range queryTokens {
		postings := idx.inverted.Search(token)
		for _, p := range postings {
			docMatches[p.DocID]++
		}
	}

	type scoredDoc struct {
		docID uint64
		count int
		text  string
	}
	var results []scoredDoc
	for docID, count := range docMatches {
		results = append(results, scoredDoc{docID: docID, count: count, text: idx.docTexts[docID]})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].count != results[j].count {
			return results[i].count > results[j].count
		}
		return results[i].docID < results[j].docID
	})

	if topK > len(results) {
		topK = len(results)
	}

	required := len(queryTokens)
	if op == OpOR {
		required = 1
	}

	var filtered []scoredDoc
	for _, r := range results {
		if r.count >= required {
			filtered = append(filtered, r)
			if len(filtered) >= topK {
				break
			}
		}
	}

	out := make([]BooleanSearchResult, len(filtered))
	for i, r := range filtered {
		out[i] = BooleanSearchResult{r.docID, r.count, r.text}
	}
	return out
}

type FTSQueryOp int

const (
	FTSOpOr     FTSQueryOp = 1
	FTSOpAnd    FTSQueryOp = 3
	FTSOpNot    FTSQueryOp = 5
	FTSOpTerm   FTSQueryOp = 7
	FTSOpPhrase FTSQueryOp = 9
)

type FTSQueryNode struct {
	Op       FTSQueryOp
	Term     string
	Children []*FTSQueryNode
}

func ParseFTSQuery(query string) *FTSQueryNode {
	tokens := tokenizeFTSQuery(query)
	node, _ := parseOrExpression(tokens, 0)
	if node == nil {
		return &FTSQueryNode{Op: FTSOpOr}
	}
	return node
}

func tokenizeFTSQuery(query string) []string {
	var tokens []string
	current := getBuilder()
	defer putBuilder(current)
	inQuotes := false

	for _, r := range query {
		if r == '"' {
			if inQuotes {
				if current.Len() > 0 {
					tokens = append(tokens, `"`+current.String()+`"`)
					current.Reset()
				}
				inQuotes = false
			} else {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				inQuotes = true
			}
		} else if unicode.IsSpace(r) && !inQuotes {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else if (r == '(' || r == ')') && !inQuotes {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(r))
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func parseOrExpression(tokens []string, pos int) (*FTSQueryNode, int) {
	left, pos := parseAndExpression(tokens, pos)
	if left == nil {
		return nil, pos
	}

	for pos < len(tokens) && strings.ToUpper(tokens[pos]) == "OR" {
		pos++
		right, newPos := parseAndExpression(tokens, pos)
		if right == nil {
			break
		}
		left = &FTSQueryNode{Op: FTSOpOr, Children: []*FTSQueryNode{left, right}}
		pos = newPos
	}
	return left, pos
}

func parseAndExpression(tokens []string, pos int) (*FTSQueryNode, int) {
	left, pos := parseNotExpression(tokens, pos)
	if left == nil {
		return nil, pos
	}

	for pos < len(tokens) {
		upper := strings.ToUpper(tokens[pos])
		if upper == "AND" {
			pos++
			right, newPos := parseNotExpression(tokens, pos)
			if right == nil {
				break
			}
			left = &FTSQueryNode{Op: FTSOpAnd, Children: []*FTSQueryNode{left, right}}
			pos = newPos
		} else if upper == "NOT" {
			pos++
			right, newPos := parseNotExpression(tokens, pos)
			if right == nil {
				break
			}
			left = &FTSQueryNode{Op: FTSOpNot, Children: []*FTSQueryNode{left, right}}
			pos = newPos
		} else {
			break
		}
	}
	return left, pos
}

func parseNotExpression(tokens []string, pos int) (*FTSQueryNode, int) {
	if pos < len(tokens) && strings.ToUpper(tokens[pos]) == "NOT" {
		pos++
		child, newPos := parsePrimary(tokens, pos)
		if child == nil {
			return nil, pos
		}
		return &FTSQueryNode{Op: FTSOpNot, Children: []*FTSQueryNode{child}}, newPos
	}
	return parsePrimary(tokens, pos)
}

func parsePrimary(tokens []string, pos int) (*FTSQueryNode, int) {
	if pos >= len(tokens) {
		return nil, pos
	}

	if tokens[pos] == "(" {
		pos++
		node, pos := parseOrExpression(tokens, pos)
		if pos < len(tokens) && tokens[pos] == ")" {
			pos++
		}
		return node, pos
	}

	token := tokens[pos]
	pos++

	if len(token) >= 2 && token[0] == '"' && token[len(token)-1] == '"' {
		phrase := token[1 : len(token)-1]
		return &FTSQueryNode{Op: FTSOpPhrase, Term: phrase}, pos
	}

	return &FTSQueryNode{Op: FTSOpTerm, Term: strings.ToLower(token)}, pos
}

func (idx *FTSIndex) SearchAdvanced(query string, topK int) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	node := ParseFTSQuery(query)
	if node == nil {
		return nil
	}

	scores := idx.evaluateNode(node)

	type result struct {
		docID   uint64
		score   float64
		docText string
	}
	var results []result
	for docID, score := range scores {
		results = append(results, result{docID, score, idx.docTexts[docID]})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > len(results) {
		topK = len(results)
	}

	output := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		output[i] = SearchResult{DocID: results[i].docID, Score: results[i].score, DocText: results[i].docText}
	}
	return output
}

func (idx *FTSIndex) evaluateNode(node *FTSQueryNode) map[uint64]float64 {
	switch node.Op {
	case FTSOpTerm:
		return idx.evaluateTerm(node.Term)
	case FTSOpPhrase:
		return idx.evaluatePhrase(node.Term)
	case FTSOpAnd:
		if len(node.Children) < 2 {
			return nil
		}
		left := idx.evaluateNode(node.Children[0])
		right := idx.evaluateNode(node.Children[1])
		result := make(map[uint64]float64)
		for docID, score := range left {
			if s, ok := right[docID]; ok {
				result[docID] = score + s
			}
		}
		return result
	case FTSOpOr:
		if len(node.Children) == 0 {
			return nil
		}
		if len(node.Children) == 1 {
			return idx.evaluateNode(node.Children[0])
		}
		left := idx.evaluateNode(node.Children[0])
		right := idx.evaluateNode(node.Children[1])
		result := make(map[uint64]float64)
		for docID, score := range left {
			result[docID] = score
		}
		for docID, score := range right {
			result[docID] += score
		}
		return result
	case FTSOpNot:
		if len(node.Children) == 1 {
			exclude := idx.evaluateNode(node.Children[0])
			result := make(map[uint64]float64)
			for docID := range idx.docLens {
				if _, excluded := exclude[docID]; !excluded {
					result[docID] = 0
				}
			}
			return result
		}
		if len(node.Children) < 2 {
			return nil
		}
		include := idx.evaluateNode(node.Children[0])
		exclude := idx.evaluateNode(node.Children[1])
		result := make(map[uint64]float64)
		for docID, score := range include {
			if _, excluded := exclude[docID]; !excluded {
				result[docID] = score
			}
		}
		return result
	default:
		return nil
	}
}

func (idx *FTSIndex) evaluateTerm(term string) map[uint64]float64 {
	postings := idx.inverted.Search(term)
	totalDocs := idx.inverted.TotalDocs()
	idf := idx.inverted.IDF(term, totalDocs)

	scores := make(map[uint64]float64)
	for _, p := range postings {
		docLen := idx.docLens[p.DocID]
		scores[p.DocID] = idx.scorer.ScoreWithIDF(docLen, p.Count, idf)
	}
	return scores
}

func (idx *FTSIndex) evaluatePhrase(phrase string) map[uint64]float64 {
	tokens := idx.tokenizer.Tokenize(phrase)
	if len(tokens) == 0 {
		return nil
	}
	if len(tokens) == 1 {
		return idx.evaluateTerm(tokens[0])
	}

	postingsPerToken := make([][]Posting, len(tokens))
	for i, token := range tokens {
		postingsPerToken[i] = idx.inverted.Search(token)
		if len(postingsPerToken[i]) == 0 {
			return nil
		}
	}

	docSets := make([]map[uint64]bool, len(tokens))
	for i, postings := range postingsPerToken {
		docSets[i] = make(map[uint64]bool, len(postings))
		for _, p := range postings {
			docSets[i][p.DocID] = true
		}
	}

	candidateDocs := make(map[uint64]bool)
	for docID := range docSets[0] {
		allContain := true
		for i := 1; i < len(docSets); i++ {
			if !docSets[i][docID] {
				allContain = false
				break
			}
		}
		if allContain {
			candidateDocs[docID] = true
		}
	}

	postingsByDoc := make([]map[uint64]Posting, len(tokens))
	for i, postings := range postingsPerToken {
		postingsByDoc[i] = make(map[uint64]Posting, len(postings))
		for _, p := range postings {
			postingsByDoc[i][p.DocID] = p
		}
	}

	scores := make(map[uint64]float64)
	totalDocs := idx.inverted.TotalDocs()

	for docID := range candidateDocs {
		phraseCount := countPhraseOccurrences(postingsByDoc, docID, len(tokens))
		if phraseCount > 0 {
			var totalScore float64
			for i := 0; i < len(tokens); i++ {

				docLen := idx.docLens[docID]
				idf := idx.inverted.IDF(tokens[i], totalDocs)
				totalScore += idx.scorer.ScoreWithIDF(docLen, phraseCount, idf)
			}
			scores[docID] = totalScore
		}
	}

	return scores
}

func countPhraseOccurrences(postingsByDoc []map[uint64]Posting, docID uint64, numTokens int) int {
	firstPosting := postingsByDoc[0][docID]
	positions := firstPosting.Positions

	count := 0
	for _, startPos := range positions {
		match := true
		for i := 1; i < numTokens; i++ {
			p := postingsByDoc[i][docID]
			targetPos := startPos + i
			found := false
			for _, pos := range p.Positions {
				if pos == targetPos {
					found = true
					break
				}
			}
			if !found {
				match = false
				break
			}
		}
		if match {
			count++
		}
	}
	return count
}

type JiebaTokenizer struct {
	dict       map[string]struct{}
	maxWordLen int
}

func NewJiebaTokenizer() *JiebaTokenizer {
	return &JiebaTokenizer{
		dict:       defaultChineseDict(),
		maxWordLen: 4,
	}
}

func NewJiebaTokenizerWithDict(words []string) *JiebaTokenizer {
	dict := make(map[string]struct{}, len(words))
	maxLen := 1
	for _, w := range words {
		dict[w] = struct{}{}
		runeCount := utf8.RuneCountInString(w)
		if runeCount > maxLen {
			maxLen = runeCount
		}
	}
	return &JiebaTokenizer{dict: dict, maxWordLen: maxLen}
}

func (t *JiebaTokenizer) Tokenize(text string) []string {
	var tokens []string
	runes := []rune(text)
	n := len(runes)
	i := 0

	for i < n {
		matched := false
		maxLen := t.maxWordLen
		if i+maxLen > n {
			maxLen = n - i
		}

		for l := maxLen; l >= 1; l-- {
			word := string(runes[i : i+l])
			if _, ok := t.dict[word]; ok {
				tokens = append(tokens, strings.ToLower(word))
				i += l
				matched = true
				break
			}
		}

		if !matched {
			r := runes[i]
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				tokens = append(tokens, strings.ToLower(string(r)))
			}
			i++
		}
	}

	return tokens
}

func defaultChineseDict() map[string]struct{} {
	words := []string{
		"的", "了", "在", "是", "我", "有", "和", "就", "不", "人",
		"都", "一", "一个", "上", "也", "很", "到", "说", "要", "去",
		"你", "会", "着", "没有", "看", "好", "自己", "这", "他", "她",
		"什么", "那", "被", "从", "它", "吗", "这个", "那个", "可以",
		"我们", "他们", "她们", "因为", "所以", "但是", "如果", "虽然",
		"搜索", "向量", "数据库", "索引", "查询", "文档", "插入", "删除",
		"更新", "创建", "打开", "关闭", "结果", "分数", "距离", "相似",
		"算法", "模型", "训练", "数据", "分析", "处理", "计算", "存储",
		"中文", "英文", "分词", "全文", "匹配", "排序", "过滤", "聚合",
	}
	dict := make(map[string]struct{}, len(words))
	for _, w := range words {
		dict[w] = struct{}{}
	}
	return dict
}
