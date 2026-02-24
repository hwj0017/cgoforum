package vectorizer

import (
	"hash/fnv"
	"math"
	"strings"
)

const DefaultDim = 768

// EmbedText creates a deterministic dense vector from text.
// This is a lightweight local embedding implementation for service integration.
func EmbedText(text string) []float32 {
	vec := make([]float32, DefaultDim)
	parts := strings.Fields(strings.ToLower(text))
	if len(parts) == 0 {
		return vec
	}

	for _, token := range parts {
		idx := int(hashToken(token) % uint32(DefaultDim))
		vec[idx] += 1.0
	}

	normalizeL2(vec)
	return vec
}

func hashToken(token string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(token))
	return h.Sum32()
}

func normalizeL2(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return
	}
	norm := float32(math.Sqrt(sum))
	for i := range vec {
		vec[i] = vec[i] / norm
	}
}
