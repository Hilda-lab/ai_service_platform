package vectordb

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
)

type ScoredChunk struct {
	Content string
	Score   float64
}

func Embed(text string, dims int) []float64 {
	if dims <= 0 {
		dims = 128
	}
	vector := make([]float64, dims)
	tokens := tokenize(text)
	for _, token := range tokens {
		hash := fnv1a(token)
		index := int(hash % uint32(dims))
		vector[index] += 1.0
	}
	normalize(vector)
	return vector
}

func MarshalVector(vector []float64) (string, error) {
	data, err := json.Marshal(vector)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func UnmarshalVector(data string) ([]float64, error) {
	var vector []float64
	if err := json.Unmarshal([]byte(data), &vector); err != nil {
		return nil, err
	}
	return vector, nil
}

func TopKByCosine(query []float64, corpus map[string][]float64, k int) []ScoredChunk {
	items := make([]ScoredChunk, 0, len(corpus))
	for content, vector := range corpus {
		score := cosine(query, vector)
		items = append(items, ScoredChunk{Content: content, Score: score})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})
	if k > 0 && len(items) > k {
		items = items[:k]
	}
	return items
}

func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	size := len(a)
	if len(b) < size {
		size = len(b)
	}
	var dot, normA, normB float64
	for i := 0; i < size; i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func normalize(vector []float64) {
	var sum float64
	for _, value := range vector {
		sum += value * value
	}
	if sum == 0 {
		return
	}
	norm := math.Sqrt(sum)
	for i := range vector {
		vector[i] /= norm
	}
}

func tokenize(text string) []string {
	lower := strings.ToLower(text)
	replacer := strings.NewReplacer(",", " ", ".", " ", "\n", " ", "\r", " ", "\t", " ", "，", " ", "。", " ", "；", " ", "：", " ", "！", " ", "？", " ", "、", " ")
	cleaned := replacer.Replace(lower)
	parts := strings.Fields(cleaned)
	if len(parts) == 0 {
		return []string{lower}
	}
	return parts
}

func fnv1a(text string) uint32 {
	const offset32 = 2166136261
	const prime32 = 16777619
	hash := uint32(offset32)
	for i := 0; i < len(text); i++ {
		hash ^= uint32(text[i])
		hash *= prime32
	}
	return hash
}
