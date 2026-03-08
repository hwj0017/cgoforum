package service

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"cgoforum/internal/domain"
	"cgoforum/internal/repository/dao"
)

type rankCacheStub struct {
	hotIDs []int64
}

func (s *rankCacheStub) GetActiveArticleIDs(ctx context.Context, window string, minScore float64) ([]int64, error) {
	return nil, nil
}
func (s *rankCacheStub) GetTopArticleIDs(ctx context.Context, window string, offset, limit int64) ([]int64, error) {
	return nil, nil
}
func (s *rankCacheStub) GetHotArticleIDs(ctx context.Context, window string, offset, limit int64) ([]int64, error) {
	return s.hotIDs, nil
}
func (s *rankCacheStub) GetHotScore(ctx context.Context, window string, articleID int64) (float64, error) {
	return 0, nil
}
func (s *rankCacheStub) GetHotScores(ctx context.Context, window string, articleIDs []int64) (map[int64]float64, error) {
	return map[int64]float64{}, nil
}
func (s *rankCacheStub) ReplaceHotRank(ctx context.Context, window string, scores map[int64]float64) error {
	return nil
}

type feedCacheStub struct {
	getSources func(articleIDs []int64) (map[int64]domain.Article, []int64, error)
	setSources func(articles []domain.Article) error
}

func (s *feedCacheStub) GetLastPull(ctx context.Context, userID int64) (string, error) {
	return "", nil
}
func (s *feedCacheStub) SetLastPull(ctx context.Context, userID int64, timestamp string) error {
	return nil
}
func (s *feedCacheStub) GetAuthorArticleIDs(ctx context.Context, authorID int64, minScore, maxScore string) ([]string, error) {
	return nil, nil
}
func (s *feedCacheStub) AddAuthorArticle(ctx context.Context, authorID int64, articleID int64, publishedAt float64) error {
	return nil
}
func (s *feedCacheStub) RemoveAuthorArticle(ctx context.Context, authorID int64, articleID int64) error {
	return nil
}
func (s *feedCacheStub) GetFollowedAuthors(ctx context.Context, userID int64) ([]string, error) {
	return nil, nil
}
func (s *feedCacheStub) SetFollowedAuthors(ctx context.Context, userID int64, authorIDs []string) error {
	return nil
}
func (s *feedCacheStub) InvalidateFollowCaches(ctx context.Context, userID, authorID int64) error {
	return nil
}
func (s *feedCacheStub) InvalidateFollowedAuthors(ctx context.Context, userID int64) error {
	return nil
}
func (s *feedCacheStub) InvalidateFollowerCount(ctx context.Context, authorID int64) error {
	return nil
}
func (s *feedCacheStub) GetArticleSources(ctx context.Context, articleIDs []int64) (map[int64]domain.Article, []int64, error) {
	if s.getSources != nil {
		return s.getSources(articleIDs)
	}
	return map[int64]domain.Article{}, articleIDs, nil
}
func (s *feedCacheStub) SetArticleSources(ctx context.Context, articles []domain.Article) error {
	if s.setSources != nil {
		return s.setSources(articles)
	}
	return nil
}
func (s *feedCacheStub) InvalidateArticleSource(ctx context.Context, articleID int64) error {
	return nil
}

type articleDAOStub struct{}

func (a *articleDAOStub) Create(ctx context.Context, article *domain.Article, stat *domain.ArticleStat) error {
	return nil
}
func (a *articleDAOStub) FindByID(ctx context.Context, id int64) (*domain.Article, error) {
	return nil, nil
}
func (a *articleDAOStub) FindByIDAndIncrViewCount(ctx context.Context, id int64) (*domain.Article, error) {
	return nil, nil
}
func (a *articleDAOStub) Update(ctx context.Context, article *domain.Article) error { return nil }
func (a *articleDAOStub) UpdateByOwner(ctx context.Context, userID, articleID int64, title, summary, contentMD, coverImg string, status int16, now time.Time) (*domain.Article, error) {
	return nil, nil
}
func (a *articleDAOStub) Delete(ctx context.Context, id int64) error { return nil }
func (a *articleDAOStub) DeleteByOwnerOrAdmin(ctx context.Context, userID, articleID int64, isAdmin bool) (*domain.Article, error) {
	return nil, nil
}
func (a *articleDAOStub) ListPublished(ctx context.Context, cursor string, limit int) ([]domain.Article, error) {
	return nil, nil
}
func (a *articleDAOStub) ListByAuthor(ctx context.Context, userID int64, cursor string, limit int) ([]domain.Article, error) {
	return nil, nil
}
func (a *articleDAOStub) FindByIDs(ctx context.Context, ids []int64) ([]domain.Article, error) {
	// Mock implementation for testing purposes
	return []domain.Article{
		{ID: 11, Title: "hot", ContentMD: "raw", PublishedAt: &time.Time{}},
	}, nil
}
func (a *articleDAOStub) IncrViewCount(ctx context.Context, articleID int64) error { return nil }
func (a *articleDAOStub) UpsertEmbedding(ctx context.Context, articleID int64, chunkID string, embedding []float32, sectionTitle string, contentText string, modelVersion string) error {
	return nil
}
func (a *articleDAOStub) VectorSearchArticleIDs(ctx context.Context, queryEmbedding []float32, limit int) ([]int64, error) {
	return nil, nil
}

type statDAOStub struct{}

func (s *statDAOStub) UpsertStat(ctx context.Context, articleID int64, field string, delta int) error {
	return nil
}
func (s *statDAOStub) GetStat(ctx context.Context, articleID int64) (*domain.ArticleStat, error) {
	return nil, nil
}
func (s *statDAOStub) BatchGetStats(ctx context.Context, ids []int64) ([]domain.ArticleStat, error) {
	return nil, nil
}
func (s *statDAOStub) UpdateHotScore(ctx context.Context, articleID int64, score24h, score7d float64) error {
	return nil
}
func (s *statDAOStub) UpdateHotScore24h(ctx context.Context, articleID int64, score float64) error {
	return nil
}
func (s *statDAOStub) UpdateHotScore7d(ctx context.Context, articleID int64, score float64) error {
	return nil
}
func (s *statDAOStub) BatchUpdateHotScores(ctx context.Context, window string, scores map[int64]float64) error {
	return nil
}

var _ dao.ArticleDAO = (*articleDAOStub)(nil)
var _ dao.StatDAO = (*statDAOStub)(nil)

func TestRankServiceListHot_UsesCacheAndPreservesOrder(t *testing.T) {
	rc := &rankCacheStub{hotIDs: []int64{3, 1, 2}}
	fc := &feedCacheStub{
		getSources: func(articleIDs []int64) (map[int64]domain.Article, []int64, error) {
			return map[int64]domain.Article{
				1: {ID: 1, Title: "a"},
				2: {ID: 2, Title: "b"},
				3: {ID: 3, Title: "c"},
			}, nil, nil
		},
	}
	ad := &articleDAOStub{}

	svc := NewRankService(rc, fc, ad, &statDAOStub{}, zap.NewNop())
	list, err := svc.ListHot(context.Background(), "24h", 3)
	if err != nil {
		t.Fatalf("ListHot returned error: %v", err)
	}
	if len(list) != 3 || list[0].ID != 3 || list[1].ID != 1 || list[2].ID != 2 {
		t.Fatalf("unexpected order or length: %+v", list)
	}
}

func TestRankServiceListHot_CacheMissFallbackAndBackfill(t *testing.T) {
	rc := &rankCacheStub{hotIDs: []int64{11}}
	setCalled := false
	fc := &feedCacheStub{
		getSources: func(articleIDs []int64) (map[int64]domain.Article, []int64, error) {
			return map[int64]domain.Article{}, []int64{11}, nil
		},
		setSources: func(articles []domain.Article) error {
			setCalled = true
			if len(articles) != 1 || articles[0].ID != 11 {
				t.Fatalf("unexpected backfill payload: %+v", articles)
			}
			if articles[0].ContentMD != "" {
				t.Fatal("expected metadata-only article backfill")
			}
			return nil
		},
	}
	ad := &articleDAOStub{}

	svc := NewRankService(rc, fc, ad, &statDAOStub{}, zap.NewNop())
	list, err := svc.ListHot(context.Background(), "24h", 3)
	if err != nil {
		t.Fatalf("ListHot returned error: %v", err)
	}
	if len(list) != 1 || list[0].ID != 11 {
		t.Fatalf("unexpected order or length: %+v", list)
	}
	if !setCalled {
		t.Fatal("expected SetArticleSources to be called")
	}
}
