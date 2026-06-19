package memory

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const simpleEmbeddingDim = 1536 // Padrão OpenAI

type embeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

type HNSWIndexer struct {
	mu      sync.RWMutex
	entries map[string]*MemoryEntry
}

func NewHNSWIndexer() *HNSWIndexer {
	return &HNSWIndexer{
		entries: make(map[string]*MemoryEntry),
	}
}

func (h *HNSWIndexer) Index(entry *MemoryEntry) {
	if entry == nil || strings.TrimSpace(entry.ID) == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	entry.Embedding = simpleEmbedding(entry.Content)
	cp := *entry
	h.entries[entry.ID] = &cp
}

func (h *HNSWIndexer) Search(query *MemoryQuery) []*MemoryEntry {
	if query == nil {
		query = &MemoryQuery{}
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	queryEmbedding := simpleEmbedding(query.Content)

	type scored struct {
		e     *MemoryEntry
		score float64
	}
	buf := make([]scored, 0, len(h.entries))
	for _, e := range h.entries {
		if !matchesFilters(e, query.Filters) {
			continue
		}
		score := semanticScore(query.Content, queryEmbedding, e)
		buf = append(buf, scored{e: e, score: score})
	}
	sort.Slice(buf, func(i, j int) bool {
		if buf[i].score == buf[j].score {
			return buf[i].e.ID < buf[j].e.ID
		}
		return buf[i].score > buf[j].score
	})
	if len(buf) > limit {
		buf = buf[:limit]
	}
	out := make([]*MemoryEntry, 0, len(buf))
	for _, s := range buf {
		cp := *s.e
		out = append(out, &cp)
	}
	return out
}

func semanticScore(content string, qEmb []float32, e *MemoryEntry) float64 {
	if len(qEmb) > 0 && len(e.Embedding) > 0 && len(qEmb) == len(e.Embedding) {
		return cosine(qEmb, e.Embedding)
	}

	q := strings.ToLower(strings.TrimSpace(content))
	c := strings.ToLower(e.Content)
	if q == "" {
		return 0
	}
	if strings.Contains(c, q) {
		return 0.5
	}
	return 0
}

func simpleEmbedding(content string) []float32 {
	if strings.ToLower(strings.TrimSpace(os.Getenv("MULTISEND_ENABLE_OPENAI_EMBEDDINGS"))) != "true" {
		return make([]float32, simpleEmbeddingDim)
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" || strings.TrimSpace(content) == "" {
		return make([]float32, simpleEmbeddingDim)
	}

	reqBody := embeddingRequest{
		Input: content,
		Model: "text-embedding-3-small",
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return make([]float32, simpleEmbeddingDim)
	}

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(b))
	if err != nil {
		return make([]float32, simpleEmbeddingDim)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return make([]float32, simpleEmbeddingDim)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return make([]float32, simpleEmbeddingDim)
	}

	var respBody embeddingResponse
	if err := json.NewDecoder(res.Body).Decode(&respBody); err != nil {
		return make([]float32, simpleEmbeddingDim)
	}

	if len(respBody.Data) > 0 {
		return respBody.Data[0].Embedding
	}

	return make([]float32, simpleEmbeddingDim)
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i] * b[i])
		na += float64(a[i] * a[i])
		nb += float64(b[i] * b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
