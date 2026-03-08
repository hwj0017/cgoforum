package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/meilisearch/meilisearch-go"
	"go.uber.org/zap"

	"cgoforum/internal/domain"
	"cgoforum/internal/repository/cache"
	"cgoforum/internal/repository/dao"
	"cgoforum/pkg/vectorizer"
)

var ErrEmptyQuery = errors.New("search query is empty")

const rrfK = 60.0

type SearchHit struct {
	Article   domain.Article `json:"article"`
	Score     float64        `json:"score"`
	Highlight string         `json:"highlight,omitempty"`
}

type SearchResult struct {
	List []domain.Article `json:"list"`
	Hits []SearchHit      `json:"hits"`
}

type SearchService interface {
	SemanticSearch(ctx context.Context, query string, limit int) (*SearchResult, error)
}

type searchService struct {
	articleDAO       dao.ArticleDAO
	statDAO          dao.StatDAO
	feedCache        cache.FeedCache
	interactionCache cache.InteractionCache
	searchClient     meilisearch.ServiceManager
	searchIndex      string
	embedder         vectorizer.Embedder
	logger           *zap.Logger
}

func NewSearchService(
	articleDAO dao.ArticleDAO,
	statDAO dao.StatDAO,
	feedCache cache.FeedCache,
	interactionCache cache.InteractionCache,
	searchClient meilisearch.ServiceManager,
	searchIndex string,
	embedder vectorizer.Embedder,
	logger *zap.Logger,
) SearchService {
	return &searchService{
		articleDAO:       articleDAO,
		statDAO:          statDAO,
		feedCache:        feedCache,
		interactionCache: interactionCache,
		searchClient:     searchClient,
		searchIndex:      searchIndex,
		embedder:         embedder,
		logger:           logger,
	}
}

func (s *searchService) SemanticSearch(ctx context.Context, query string, limit int) (*SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyQuery
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	recallLimit := limit * 5
	if recallLimit < 100 {
		recallLimit = 100
	}
	if recallLimit > 200 {
		recallLimit = 200
	}

	var lexicalIDs []int64
	lexicalHighlights := make(map[int64]string)
	var semanticIDs []int64
	var lexicalErr error
	var semanticErr error

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		lexicalIDs, lexicalHighlights, lexicalErr = s.lexicalSearch(ctx, query, recallLimit)
	}()

	go func() {
		defer wg.Done()
		semanticIDs, semanticErr = s.semanticIDSearch(ctx, query, recallLimit)
	}()

	wg.Wait()

	if lexicalErr != nil {
		s.logger.Warn("lexical search failed, fallback semantic", zap.Error(lexicalErr), zap.String("query", query))
	}
	if semanticErr != nil {
		s.logger.Warn("semantic search failed, fallback lexical", zap.Error(semanticErr), zap.String("query", query))
	}

	if len(lexicalIDs) == 0 && len(semanticIDs) == 0 {
		return &SearchResult{List: []domain.Article{}, Hits: []SearchHit{}}, nil
	}

	scores, orderedIDs := rrfFuse(lexicalIDs, semanticIDs, limit, rrfK)
	if len(orderedIDs) == 0 {
		return &SearchResult{List: []domain.Article{}, Hits: []SearchHit{}}, nil
	}

	articleMap, misses, err := s.feedCache.GetArticleSources(ctx, orderedIDs)
	if err != nil {
		s.logger.Warn("search get article source cache failed", zap.Error(err))
		articleMap = make(map[int64]domain.Article, len(orderedIDs))
		misses = orderedIDs
	}

	if len(misses) > 0 {
		dbArticles, dbErr := s.articleDAO.FindByIDs(ctx, misses)
		if dbErr != nil {
			return nil, dbErr
		}
		metaArticles := make([]domain.Article, 0, len(dbArticles))
		for _, a := range dbArticles {
			meta := trimToSearchMeta(a)
			articleMap[meta.ID] = meta
			metaArticles = append(metaArticles, meta)
		}
		_ = s.feedCache.SetArticleSources(ctx, metaArticles)
	}

	interactionCounts, interactionMisses, countErr := s.interactionCache.BatchGetCounts(ctx, orderedIDs)
	if countErr != nil {
		s.logger.Warn("batch get interaction counts failed", zap.Error(countErr))
		interactionCounts = map[int64]cache.InteractionCount{}
		interactionMisses = orderedIDs
	}

	if len(interactionMisses) > 0 {
		stats, statErr := s.statDAO.BatchGetStats(ctx, interactionMisses)
		if statErr != nil {
			s.logger.Warn("batch get stats fallback failed", zap.Error(statErr))
		} else {
			warm := make(map[int64]cache.InteractionCount, len(stats))
			for _, st := range stats {
				warm[st.ArticleID] = cache.InteractionCount{
					LikeCount:    st.LikeCount,
					CollectCount: st.CollectCount,
				}
				interactionCounts[st.ArticleID] = warm[st.ArticleID]
			}
			_ = s.interactionCache.BatchSetCounts(ctx, warm)
		}
	}

	articles := make([]domain.Article, 0, len(orderedIDs))
	hits := make([]SearchHit, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		a, ok := articleMap[id]
		if !ok || a.Status != 1 {
			continue
		}
		if c, ok := interactionCounts[id]; ok {
			if a.Stat == nil {
				a.Stat = &domain.ArticleStat{ArticleID: id}
			}
			a.Stat.LikeCount = c.LikeCount
			a.Stat.CollectCount = c.CollectCount
		}

		articles = append(articles, a)
		hits = append(hits, SearchHit{
			Article:   a,
			Score:     round6(scores[id]),
			Highlight: lexicalHighlights[id],
		})
		if len(articles) >= limit {
			break
		}
	}

	return &SearchResult{List: articles, Hits: hits}, nil
}

func (s *searchService) semanticIDSearch(ctx context.Context, query string, limit int) ([]int64, error) {
	queryVec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.articleDAO.VectorSearchArticleIDs(ctx, queryVec, limit)
}

func (s *searchService) lexicalSearch(ctx context.Context, query string, limit int) ([]int64, map[int64]string, error) {
	if s.searchClient == nil || s.searchIndex == "" {
		return nil, map[int64]string{}, nil
	}

	idx := s.searchClient.Index(s.searchIndex)
	resp, err := idx.SearchWithContext(ctx, query, &meilisearch.SearchRequest{
		Limit:                 int64(limit),
		AttributesToRetrieve:  []string{"article_id", "title", "summary", "content_text"},
		AttributesToHighlight: []string{"title", "summary", "content_text"},
		HighlightPreTag:       "<em>",
		HighlightPostTag:      "</em>",
	})
	if err != nil {
		return nil, nil, err
	}

	ids := make([]int64, 0, len(resp.Hits))
	highlights := make(map[int64]string, len(resp.Hits))
	seen := make(map[int64]struct{}, len(resp.Hits))
	for _, hit := range resp.Hits {
		id, ok := parseHitArticleID(hit)
		if !ok {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		highlights[id] = extractHighlight(hit)
	}

	return ids, highlights, nil
}

func parseHitArticleID(hit meilisearch.Hit) (int64, bool) {
	raw, ok := hit["article_id"]
	if !ok {
		return 0, false
	}

	var idNum int64
	if err := json.Unmarshal(raw, &idNum); err == nil {
		return idNum, true
	}

	var idText string
	if err := json.Unmarshal(raw, &idText); err == nil {
		parsed, parseErr := strconv.ParseInt(idText, 10, 64)
		if parseErr == nil {
			return parsed, true
		}
	}

	return 0, false
}

func extractHighlight(hit meilisearch.Hit) string {
	rawFormatted, ok := hit["_formatted"]
	if !ok {
		return ""
	}

	formatted := map[string]string{}
	if err := json.Unmarshal(rawFormatted, &formatted); err != nil {
		return ""
	}

	candidates := []string{formatted["summary"], formatted["title"], formatted["content_text"]}
	for _, v := range candidates {
		v = strings.TrimSpace(v)
		if v != "" {
			if len(v) > 240 {
				return v[:240]
			}
			return v
		}
	}
	return ""
}

func rrfFuse(lexicalIDs []int64, semanticIDs []int64, topN int, k float64) (map[int64]float64, []int64) {
	scores := make(map[int64]float64, len(lexicalIDs)+len(semanticIDs))

	for i, id := range lexicalIDs {
		rank := float64(i + 1)
		scores[id] += 1.0 / (k + rank)
	}
	for i, id := range semanticIDs {
		rank := float64(i + 1)
		scores[id] += 1.0 / (k + rank)
	}

	ids := make([]int64, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}

	sort.SliceStable(ids, func(i, j int) bool {
		si := scores[ids[i]]
		sj := scores[ids[j]]
		if si == sj {
			return ids[i] < ids[j]
		}
		return si > sj
	})

	if topN > 0 && len(ids) > topN {
		ids = ids[:topN]
	}

	return scores, ids
}

func round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

func trimToSearchMeta(a domain.Article) domain.Article {
	return domain.Article{
		ID:          a.ID,
		UserID:      a.UserID,
		Title:       a.Title,
		Summary:     a.Summary,
		CoverImg:    a.CoverImg,
		Status:      a.Status,
		IsTop:       a.IsTop,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
		PublishedAt: a.PublishedAt,
		User:        a.User,
		Stat:        a.Stat,
	}
}
