package fts

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Tokenizer interface {
	Tokenize(text string) []string
}

type StandardTokenizer struct{}

func NewStandardTokenizer() *StandardTokenizer {
	return &StandardTokenizer{}
}

func (t *StandardTokenizer) Tokenize(text string) []string {
	var tokens []string
	current := strings.Builder{}
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
	DocID    uint64
	Position int
	Count    int
}

type InvertedIndex struct {
	dict      map[string][]Posting
	totalDocs int
	docIDs    map[uint64]struct{}
}

func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		dict:   make(map[string][]Posting),
		docIDs: make(map[uint64]struct{}),
	}
}

func (idx *InvertedIndex) AddDocument(docID uint64, tokens []string) {
	if _, exists := idx.docIDs[docID]; exists {
		idx.RemoveDocument(docID)
	}
	idx.docIDs[docID] = struct{}{}
	idx.totalDocs++
	positions := make(map[string][]int)
	for pos, token := range tokens {
		positions[token] = append(positions[token], pos)
	}

	for token, posList := range positions {
		idx.dict[token] = append(idx.dict[token], Posting{
			DocID:    docID,
			Position: posList[0],
			Count:    len(posList),
		})
	}
}

func (idx *InvertedIndex) Search(token string) []Posting {
	return idx.dict[strings.ToLower(token)]
}

func (idx *InvertedIndex) TotalDocs() int {
	return idx.totalDocs
}

func (idx *InvertedIndex) DocFreq(token string) int {
	return len(idx.dict[strings.ToLower(token)])
}

func (idx *InvertedIndex) RemoveDocument(docID uint64) {
	removed := false
	for token, postings := range idx.dict {
		filtered := make([]Posting, 0, len(postings))
		for _, p := range postings {
			if p.DocID != docID {
				filtered = append(filtered, p)
			} else {
				removed = true
			}
		}
		if len(filtered) == 0 {
			delete(idx.dict, token)
		} else {
			idx.dict[token] = filtered
		}
	}
	if removed && idx.totalDocs > 0 {
		idx.totalDocs--
		delete(idx.docIDs, docID)
	}
}

type BM25Scorer struct {
	k1       float64
	b        float64
	avgDocLen float64
	docLens  []int
}

func NewBM25Scorer() *BM25Scorer {
	return &BM25Scorer{
		k1:       1.2,
		b:        0.75,
		docLens:  make([]int, 0),
	}
}

func (s *BM25Scorer) UpdateStats(docLens []int) {
	s.docLens = docLens
	var total int
	for _, l := range docLens {
		total += l
	}
	if len(docLens) > 0 {
		s.avgDocLen = float64(total) / float64(len(docLens))
	}
}

func (s *BM25Scorer) Score(docID uint64, docLen int, termFreq int, docFreq int, totalDocs int) float64 {
	idf := math.Log(1.0 + float64(totalDocs-docFreq+1)/(float64(docFreq)+0.5))
	tf := float64(termFreq) * (s.k1 + 1.0)
	avgLen := s.avgDocLen
	if avgLen == 0 {
		avgLen = 1
	}
	tf /= float64(termFreq) + s.k1*(1.0-s.b+s.b*float64(docLen)/avgLen)
	return idf * tf
}

type FTSIndex struct {
	tokenizer Tokenizer
	inverted  *InvertedIndex
	scorer    *BM25Scorer
	docTexts  map[uint64]string
	docLens   map[uint64]int
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
	tokens := idx.tokenizer.Tokenize(text)
	idx.inverted.AddDocument(docID, tokens)
	idx.docTexts[docID] = text
	idx.docLens[docID] = len(tokens)

	var lens []int
	for _, l := range idx.docLens {
		lens = append(lens, l)
	}
	idx.scorer.UpdateStats(lens)
}

func (idx *FTSIndex) Delete(docID uint64) {
	idx.inverted.RemoveDocument(docID)
	delete(idx.docTexts, docID)
	delete(idx.docLens, docID)

	var lens []int
	for _, l := range idx.docLens {
		lens = append(lens, l)
	}
	idx.scorer.UpdateStats(lens)
}

func (idx *FTSIndex) Search(query string, topK int) []struct {
	DocID  uint64
	Score  float64
	DocText string
} {
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
			docFreq := idx.inverted.DocFreq(token)
			totalScore += idx.scorer.Score(docID, docLen, tf, docFreq, totalDocs)
		}
		scores[docID] = totalScore
	}

	type scoredDoc struct {
		docID  uint64
		score  float64
		text   string
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

	out := make([]struct {
		DocID  uint64
		Score  float64
		DocText string
	}, topK)
	for i := 0; i < topK; i++ {
		out[i] = struct {
			DocID  uint64
			Score  float64
			DocText string
		}{results[i].docID, results[i].score, results[i].text}
	}
	return out
}

type QueryOp int

const (
	OpAND QueryOp = 0
	OpOR  QueryOp = 1
)

func (idx *FTSIndex) SearchBoolean(query string, op QueryOp, topK int) []struct {
	DocID  uint64
	Score  int
	DocText string
} {
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
		return results[i].count > results[j].count
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

	out := make([]struct {
		DocID  uint64
		Score  int
		DocText string
	}, len(filtered))
	for i, r := range filtered {
		out[i] = struct {
			DocID  uint64
			Score  int
			DocText string
		}{r.docID, r.count, r.text}
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
	var current strings.Builder
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

func (idx *FTSIndex) SearchAdvanced(query string, topK int) []struct {
	DocID   uint64
	Score   float64
	DocText string
} {
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

	output := make([]struct {
		DocID   uint64
		Score   float64
		DocText string
	}, topK)
	for i := 0; i < topK; i++ {
		output[i].DocID = results[i].docID
		output[i].Score = results[i].score
		output[i].DocText = results[i].docText
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
	docFreq := idx.inverted.DocFreq(term)

	scores := make(map[uint64]float64)
	for _, p := range postings {
		docLen := idx.docLens[p.DocID]
		score := idx.scorer.Score(p.DocID, docLen, p.Count, docFreq, totalDocs)
		scores[p.DocID] = score
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

	docsForFirst := idx.evaluateTerm(tokens[0])
	for _, token := range tokens[1:] {
		docsForToken := idx.evaluateTerm(token)
		intersection := make(map[uint64]float64)
		for docID, score := range docsForFirst {
			if s, ok := docsForToken[docID]; ok {
				intersection[docID] = score + s
			}
		}
		docsForFirst = intersection
	}

	return docsForFirst
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
