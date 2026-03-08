package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"cgoforum/internal/domain"
)

type articleCreateDAOStub struct {
	savedArticle *domain.Article
	savedStat    *domain.ArticleStat
}

func (a *articleCreateDAOStub) Create(ctx context.Context, article *domain.Article, stat *domain.ArticleStat) error {
	a.savedArticle = article
	a.savedStat = stat
	return nil
}

func (a *articleCreateDAOStub) FindByID(ctx context.Context, id int64) (*domain.Article, error) {
	return nil, nil
}

func (a *articleCreateDAOStub) FindByIDAndIncrViewCount(ctx context.Context, id int64) (*domain.Article, error) {
	return nil, nil
}

func (a *articleCreateDAOStub) Update(ctx context.Context, article *domain.Article) error { return nil }

func (a *articleCreateDAOStub) UpdateByOwner(ctx context.Context, userID, articleID int64, title, summary, contentMD, coverImg string, status int16, now time.Time) (*domain.Article, error) {
	return nil, nil
}

func (a *articleCreateDAOStub) Delete(ctx context.Context, id int64) error { return nil }

func (a *articleCreateDAOStub) DeleteByOwnerOrAdmin(ctx context.Context, userID, articleID int64, isAdmin bool) (*domain.Article, error) {
	return nil, nil
}

func (a *articleCreateDAOStub) ListPublished(ctx context.Context, cursor string, limit int) ([]domain.Article, error) {
	return nil, nil
}

func (a *articleCreateDAOStub) ListByAuthor(ctx context.Context, userID int64, cursor string, limit int) ([]domain.Article, error) {
	return nil, nil
}

func (a *articleCreateDAOStub) FindByIDs(ctx context.Context, ids []int64) ([]domain.Article, error) {
	return nil, nil
}

func (a *articleCreateDAOStub) IncrViewCount(ctx context.Context, articleID int64) error { return nil }

func (a *articleCreateDAOStub) UpsertEmbedding(ctx context.Context, articleID int64, chunkID string, embedding []float32, sectionTitle string, contentText string, modelVersion string) error {
	return nil
}

func (a *articleCreateDAOStub) VectorSearchArticleIDs(ctx context.Context, queryEmbedding []float32, limit int) ([]int64, error) {
	return nil, nil
}

type addAuthorCall struct {
	authorID    int64
	articleID   int64
	publishedAt float64
}

type articleCreateFeedCacheStub struct {
	addAuthorCalls []addAuthorCall
	setSourcesArgs [][]domain.Article
}

func (s *articleCreateFeedCacheStub) GetLastPull(ctx context.Context, userID int64) (string, error) {
	return "", nil
}

func (s *articleCreateFeedCacheStub) SetLastPull(ctx context.Context, userID int64, timestamp string) error {
	return nil
}

func (s *articleCreateFeedCacheStub) GetAuthorArticleIDs(ctx context.Context, authorID int64, minScore, maxScore string) ([]string, error) {
	return nil, nil
}

func (s *articleCreateFeedCacheStub) AddAuthorArticle(ctx context.Context, authorID int64, articleID int64, publishedAt float64) error {
	s.addAuthorCalls = append(s.addAuthorCalls, addAuthorCall{authorID: authorID, articleID: articleID, publishedAt: publishedAt})
	return nil
}

func (s *articleCreateFeedCacheStub) RemoveAuthorArticle(ctx context.Context, authorID int64, articleID int64) error {
	return nil
}

func (s *articleCreateFeedCacheStub) GetFollowedAuthors(ctx context.Context, userID int64) ([]string, error) {
	return nil, nil
}

func (s *articleCreateFeedCacheStub) SetFollowedAuthors(ctx context.Context, userID int64, authorIDs []string) error {
	return nil
}

func (s *articleCreateFeedCacheStub) InvalidateFollowCaches(ctx context.Context, userID, authorID int64) error {
	return nil
}

func (s *articleCreateFeedCacheStub) InvalidateFollowedAuthors(ctx context.Context, userID int64) error {
	return nil
}

func (s *articleCreateFeedCacheStub) InvalidateFollowerCount(ctx context.Context, authorID int64) error {
	return nil
}

func (s *articleCreateFeedCacheStub) GetArticleSources(ctx context.Context, articleIDs []int64) (map[int64]domain.Article, []int64, error) {
	return map[int64]domain.Article{}, articleIDs, nil
}

func (s *articleCreateFeedCacheStub) SetArticleSources(ctx context.Context, articles []domain.Article) error {
	copied := make([]domain.Article, len(articles))
	copy(copied, articles)
	s.setSourcesArgs = append(s.setSourcesArgs, copied)
	return nil
}

func (s *articleCreateFeedCacheStub) InvalidateArticleSource(ctx context.Context, articleID int64) error {
	return nil
}

type articleEventCall struct {
	eventType string
	article   domain.Article
}

type embeddingTaskCall struct {
	articleID    int64
	text         string
	modelVersion string
}

type articleCreatePublisherStub struct {
	articleEvents  []articleEventCall
	embeddingTasks []embeddingTaskCall
}

func (s *articleCreatePublisherStub) PublishArticleEvent(ctx context.Context, eventType string, article *domain.Article) error {
	if article == nil {
		return nil
	}
	s.articleEvents = append(s.articleEvents, articleEventCall{eventType: eventType, article: *article})
	return nil
}

func (s *articleCreatePublisherStub) PublishInteractionEvent(ctx context.Context, eventType string, userID, articleID int64, action, note string) error {
	return nil
}

func (s *articleCreatePublisherStub) PublishEmbeddingTask(ctx context.Context, articleID int64, text, modelVersion string) error {
	s.embeddingTasks = append(s.embeddingTasks, embeddingTaskCall{articleID: articleID, text: text, modelVersion: modelVersion})
	return nil
}

func TestArticleServiceCreate_Published_TriggersPostProcessing(t *testing.T) {
	daoStub := &articleCreateDAOStub{}
	feedStub := &articleCreateFeedCacheStub{}
	pubStub := &articleCreatePublisherStub{}

	svc := NewArticleService(daoStub, feedStub, pubStub, zap.NewNop())

	article, err := svc.Create(
		context.Background(),
		1001,
		"Go 测试标题",
		"测试摘要",
		"<script>alert(1)</script><b>正文</b>",
		"",
		1,
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if article == nil {
		t.Fatal("expected created article")
	}
	if daoStub.savedArticle == nil || daoStub.savedStat == nil {
		t.Fatal("expected DAO Create to receive article and stat")
	}
	if strings.Contains(strings.ToLower(daoStub.savedArticle.ContentMD), "<script") {
		t.Fatalf("expected content to be sanitized, got: %s", daoStub.savedArticle.ContentMD)
	}
	if article.PublishedAt == nil {
		t.Fatal("expected published article to have PublishedAt")
	}

	if len(feedStub.addAuthorCalls) != 1 {
		t.Fatalf("expected AddAuthorArticle called once, got %d", len(feedStub.addAuthorCalls))
	}
	if feedStub.addAuthorCalls[0].authorID != 1001 || feedStub.addAuthorCalls[0].articleID != article.ID {
		t.Fatalf("unexpected AddAuthorArticle args: %+v", feedStub.addAuthorCalls[0])
	}
	if len(feedStub.setSourcesArgs) != 1 || len(feedStub.setSourcesArgs[0]) != 1 || feedStub.setSourcesArgs[0][0].ID != article.ID {
		t.Fatalf("unexpected SetArticleSources args: %+v", feedStub.setSourcesArgs)
	}

	if len(pubStub.articleEvents) != 1 {
		t.Fatalf("expected PublishArticleEvent called once, got %d", len(pubStub.articleEvents))
	}
	if pubStub.articleEvents[0].eventType != "article.published" {
		t.Fatalf("unexpected event type: %s", pubStub.articleEvents[0].eventType)
	}

	if len(pubStub.embeddingTasks) != 1 {
		t.Fatalf("expected PublishEmbeddingTask called once, got %d", len(pubStub.embeddingTasks))
	}
	emb := pubStub.embeddingTasks[0]
	if emb.articleID != article.ID {
		t.Fatalf("unexpected embedding article_id: %d", emb.articleID)
	}
	if emb.modelVersion != "bge-m3-v1" {
		t.Fatalf("unexpected embedding model version: %s", emb.modelVersion)
	}
	if strings.Contains(emb.text, "<") || strings.Contains(emb.text, ">") {
		t.Fatalf("expected embedding text to be plain text, got: %s", emb.text)
	}
}

func TestArticleServiceCreate_Draft_DoesNotTriggerPostProcessing(t *testing.T) {
	daoStub := &articleCreateDAOStub{}
	feedStub := &articleCreateFeedCacheStub{}
	pubStub := &articleCreatePublisherStub{}

	svc := NewArticleService(daoStub, feedStub, pubStub, zap.NewNop())

	article, err := svc.Create(
		context.Background(),
		2002,
		"草稿标题",
		"草稿摘要",
		"草稿内容",
		"",
		0,
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if article == nil {
		t.Fatal("expected created article")
	}
	if article.PublishedAt != nil {
		t.Fatal("expected draft article to not have PublishedAt")
	}

	if len(feedStub.addAuthorCalls) != 0 {
		t.Fatalf("expected no AddAuthorArticle call, got %d", len(feedStub.addAuthorCalls))
	}
	if len(feedStub.setSourcesArgs) != 0 {
		t.Fatalf("expected no SetArticleSources call, got %d", len(feedStub.setSourcesArgs))
	}
	if len(pubStub.articleEvents) != 0 {
		t.Fatalf("expected no article events, got %d", len(pubStub.articleEvents))
	}
	if len(pubStub.embeddingTasks) != 0 {
		t.Fatalf("expected no embedding tasks, got %d", len(pubStub.embeddingTasks))
	}
}

func TestArticleServiceCreate_Published_EmbeddingTextTruncatedTo2000(t *testing.T) {
	daoStub := &articleCreateDAOStub{}
	feedStub := &articleCreateFeedCacheStub{}
	pubStub := &articleCreatePublisherStub{}

	svc := NewArticleService(daoStub, feedStub, pubStub, zap.NewNop())

	longContent := strings.Repeat("a", 5000)
	article, err := svc.Create(
		context.Background(),
		3003,
		"title",
		"summary",
		longContent,
		"",
		1,
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if article == nil {
		t.Fatal("expected created article")
	}
	if len(pubStub.embeddingTasks) != 1 {
		t.Fatalf("expected one embedding task, got %d", len(pubStub.embeddingTasks))
	}
	if got := len(pubStub.embeddingTasks[0].text); got != 2000 {
		t.Fatalf("expected embedding text length 2000, got %d", got)
	}
}
