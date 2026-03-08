package service

import (
	"testing"
	"time"

	"cgoforum/internal/domain"
)

func TestParseFeedCursor(t *testing.T) {
	ts, id, err := parseFeedCursor("1713225600,123")
	if err != nil {
		t.Fatalf("parseFeedCursor returned error: %v", err)
	}
	if ts != 1713225600 || id != 123 {
		t.Fatalf("unexpected cursor values: ts=%d id=%d", ts, id)
	}

	_, _, err = parseFeedCursor("invalid")
	if err == nil {
		t.Fatal("expected error for invalid cursor format")
	}

	_, _, err = parseFeedCursor("abc,123")
	if err == nil {
		t.Fatal("expected error for invalid timestamp")
	}

	_, _, err = parseFeedCursor("1713225600,abc")
	if err == nil {
		t.Fatal("expected error for invalid id")
	}
}

func TestSortFeedArticles(t *testing.T) {
	t1 := time.Unix(1713225600, 0)
	t2 := time.Unix(1713229200, 0)
	t3 := time.Unix(1713229200, 0)

	items := []domain.Article{
		{ID: 1, PublishedAt: &t1},
		{ID: 2, PublishedAt: &t2},
		{ID: 3, PublishedAt: &t3},
	}

	sortFeedArticles(items)

	// Expected: newer published_at first; if same timestamp, bigger id first.
	if items[0].ID != 3 || items[1].ID != 2 || items[2].ID != 1 {
		t.Fatalf("unexpected sort order: got [%d,%d,%d]", items[0].ID, items[1].ID, items[2].ID)
	}
}
