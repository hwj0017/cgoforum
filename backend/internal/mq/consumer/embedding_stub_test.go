package consumer

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseEmbeddingPayload_FromEmbeddingGenerate(t *testing.T) {
	input := map[string]interface{}{
		"article_id":    float64(123),
		"text_to_embed": "title summary plain",
		"model_version": "bge-m3-v1",
	}
	fmt.Printf("Input data: %+v\n", input) // Debug log
	articleID, chunks, modelVersion, err := parseEmbeddingPayload("embedding.generate", input)
	if err != nil {
		t.Fatalf("parseEmbeddingPayload returned error: %v", err)
	}
	if articleID != 123 {
		t.Fatalf("unexpected article id: %d", articleID)
	}
	if len(chunks) != 1 || chunks[0]["content_text"] != "title summary plain" {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
	if modelVersion != "bge-m3-v1" {
		t.Fatalf("unexpected model version: %s", modelVersion)
	}
}

func TestParseEmbeddingPayload_EmptyTextRejected(t *testing.T) {
	_, _, _, err := parseEmbeddingPayload("embedding.generate", map[string]interface{}{
		"article_id":    float64(123),
		"text_to_embed": "   ",
	})
	if err == nil {
		t.Fatal("expected error for empty text_to_embed")
	}
}

func TestParseEmbeddingPayload_FromArticleEvent_TruncatesTo8000(t *testing.T) {
	longContent := strings.Repeat("x", 9000)
	articleID, chunks, modelVersion, err := parseEmbeddingPayload("article.published", map[string]interface{}{
		"article_id": float64(123),
		"title":      "t",
		"summary":    "s",
		"content_md": longContent,
	})
	if err != nil {
		t.Fatalf("parseEmbeddingPayload returned error: %v", err)
	}
	if articleID != 123 {
		t.Fatalf("unexpected article id: %d", articleID)
	}
	if len(chunks) == 0 {
		t.Fatal("expected non-empty chunks")
	}
	var totalLength int
	for _, chunk := range chunks {
		totalLength += len(chunk["content_text"])
	}
	if totalLength > 8000 {
		t.Fatalf("total chunk length exceeds 8000: %d", totalLength)
	}
	if modelVersion != "sentence-transformer" {
		t.Fatalf("unexpected model version: %s", modelVersion)
	}
}
