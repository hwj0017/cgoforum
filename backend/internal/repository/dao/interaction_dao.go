package dao

import (
	"context"
	"errors"
	"strings"
	"time"

	"cgoforum/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// --- Follow DAO ---

type FollowDAO interface {
	CreateFollow(ctx context.Context, followerID, authorID int64) error
	DeleteFollow(ctx context.Context, followerID, authorID int64) error
	ListFollowingIDs(ctx context.Context, userID int64) ([]int64, error)
	IsFollowing(ctx context.Context, followerID, authorID int64) (bool, error)
}

type followDAO struct {
	db *gorm.DB
}

func NewFollowDAO(db *gorm.DB) FollowDAO {
	return &followDAO{db: db}
}

func (d *followDAO) CreateFollow(ctx context.Context, followerID, authorID int64) error {
	follow := &domain.Follow{
		FollowerID: followerID,
		AuthorID:   authorID,
		CreatedAt:  time.Now(),
	}
	res := d.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(follow)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDuplicate
	}
	return nil
}

func (d *followDAO) DeleteFollow(ctx context.Context, followerID, authorID int64) error {
	return d.db.WithContext(ctx).
		Where("follower_id = ? AND author_id = ?", followerID, authorID).
		Delete(&domain.Follow{}).Error
}

func (d *followDAO) ListFollowingIDs(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := d.db.WithContext(ctx).
		Model(&domain.Follow{}).
		Where("follower_id = ?", userID).
		Pluck("author_id", &ids).Error
	return ids, err
}

func (d *followDAO) IsFollowing(ctx context.Context, followerID, authorID int64) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&domain.Follow{}).
		Where("follower_id = ? AND author_id = ?", followerID, authorID).
		Count(&count).Error
	return count > 0, err
}

// --- Like DAO ---

type LikeDAO interface {
	CreateLike(ctx context.Context, userID, articleID int64) error
	DeleteLike(ctx context.Context, userID, articleID int64) error
	IsLiked(ctx context.Context, userID, articleID int64) (bool, error)
	ListLikedUserIDsByArticle(ctx context.Context, articleID int64) ([]int64, error)
}

type likeDAO struct {
	db *gorm.DB
}

func NewLikeDAO(db *gorm.DB) LikeDAO {
	return &likeDAO{db: db}
}

func (d *likeDAO) CreateLike(ctx context.Context, userID, articleID int64) error {
	like := &domain.Like{
		UserID:    userID,
		ArticleID: articleID,
		CreatedAt: time.Now(),
	}
	err := d.db.WithContext(ctx).Create(like).Error
	if err != nil && strings.Contains(err.Error(), "idx_like_pair") {
		return ErrDuplicate
	}
	return err
}

func (d *likeDAO) DeleteLike(ctx context.Context, userID, articleID int64) error {
	return d.db.WithContext(ctx).
		Where("user_id = ? AND article_id = ?", userID, articleID).
		Delete(&domain.Like{}).Error
}

func (d *likeDAO) IsLiked(ctx context.Context, userID, articleID int64) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&domain.Like{}).
		Where("user_id = ? AND article_id = ?", userID, articleID).
		Count(&count).Error
	return count > 0, err
}

func (d *likeDAO) ListLikedUserIDsByArticle(ctx context.Context, articleID int64) ([]int64, error) {
	var userIDs []int64
	err := d.db.WithContext(ctx).
		Model(&domain.Like{}).
		Where("article_id = ?", articleID).
		Pluck("user_id", &userIDs).Error
	return userIDs, err
}

// --- Collect DAO ---

type CollectDAO interface {
	CreateCollectWithStat(ctx context.Context, userID, articleID int64) (bool, error)
	DeleteCollectWithStat(ctx context.Context, userID, articleID int64) (bool, error)
}

type collectDAO struct {
	db *gorm.DB
}

func NewCollectDAO(db *gorm.DB) CollectDAO {
	return &collectDAO{db: db}
}

func (d *collectDAO) CreateCollectWithStat(ctx context.Context, userID, articleID int64) (bool, error) {
	created := false
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		newRow := &domain.Collect{
			UserID:    userID,
			ArticleID: articleID,
			CreatedAt: time.Now(),
		}

		res := tx.WithContext(ctx).
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(newRow)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			created = false
			return nil
		}

		if err := tx.WithContext(ctx).Exec(
			`INSERT INTO sys_article_stat (article_id, collect_count, updated_at)
			 VALUES (?, 1, NOW())
			 ON CONFLICT (article_id) DO UPDATE SET collect_count = sys_article_stat.collect_count + 1, updated_at = NOW()`,
			articleID,
		).Error; err != nil {
			return err
		}

		created = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return created, nil
}

func (d *collectDAO) DeleteCollectWithStat(ctx context.Context, userID, articleID int64) (bool, error) {
	removed := false
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.WithContext(ctx).
			Where("user_id = ? AND article_id = ?", userID, articleID).
			Delete(&domain.Collect{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			removed = false
			return nil
		}

		if err := tx.WithContext(ctx).Exec(
			`INSERT INTO sys_article_stat (article_id, collect_count, updated_at)
			 VALUES (?, 0, NOW())
			 ON CONFLICT (article_id) DO UPDATE SET collect_count = GREATEST(sys_article_stat.collect_count - 1, 0), updated_at = NOW()`,
			articleID,
		).Error; err != nil {
			return err
		}

		removed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return removed, nil
}

// --- Stat DAO ---

type StatDAO interface {
	UpsertStat(ctx context.Context, articleID int64, field string, delta int) error
	GetStat(ctx context.Context, articleID int64) (*domain.ArticleStat, error)
	BatchGetStats(ctx context.Context, ids []int64) ([]domain.ArticleStat, error)
	UpdateHotScore(ctx context.Context, articleID int64, score24h, score7d float64) error
	UpdateHotScore24h(ctx context.Context, articleID int64, score float64) error
	UpdateHotScore7d(ctx context.Context, articleID int64, score float64) error
	BatchUpdateHotScores(ctx context.Context, window string, scores map[int64]float64) error
}

type statDAO struct {
	db *gorm.DB
}

func NewStatDAO(db *gorm.DB) StatDAO {
	return &statDAO{db: db}
}

func (d *statDAO) UpsertStat(ctx context.Context, articleID int64, field string, delta int) error {
	// Only allow explicit stat fields to avoid SQL injection via column name.
	switch field {
	case "view_count", "like_count", "collect_count", "comment_count":
	default:
		return errors.New("invalid stat field")
	}

	setClause := field + " = sys_article_stat." + field + " + EXCLUDED." + field
	return d.db.WithContext(ctx).Exec(
		`INSERT INTO sys_article_stat (article_id, `+field+`, updated_at)
		 VALUES (?, ?, NOW())
		 ON CONFLICT (article_id) DO UPDATE SET `+setClause+`, updated_at = NOW()`,
		articleID, delta,
	).Error
}

func (d *statDAO) GetStat(ctx context.Context, articleID int64) (*domain.ArticleStat, error) {
	var stat domain.ArticleStat
	if err := d.db.WithContext(ctx).First(&stat, articleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &stat, nil
}

func (d *statDAO) BatchGetStats(ctx context.Context, ids []int64) ([]domain.ArticleStat, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var stats []domain.ArticleStat
	if err := d.db.WithContext(ctx).Where("article_id IN ?", ids).Find(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}

func (d *statDAO) UpdateHotScore(ctx context.Context, articleID int64, score24h, score7d float64) error {
	return d.db.WithContext(ctx).Model(&domain.ArticleStat{}).
		Where("article_id = ?", articleID).
		Updates(map[string]interface{}{
			"hot_score_24h": score24h,
			"hot_score_7d":  score7d,
			"last_hot_calc": gorm.Expr("NOW()"),
			"updated_at":    gorm.Expr("NOW()"),
		}).Error
}

func (d *statDAO) UpdateHotScore24h(ctx context.Context, articleID int64, score float64) error {
	return d.db.WithContext(ctx).Model(&domain.ArticleStat{}).
		Where("article_id = ?", articleID).
		Updates(map[string]interface{}{
			"hot_score_24h": score,
			"last_hot_calc": gorm.Expr("NOW()"),
			"updated_at":    gorm.Expr("NOW()"),
		}).Error
}

func (d *statDAO) UpdateHotScore7d(ctx context.Context, articleID int64, score float64) error {
	return d.db.WithContext(ctx).Model(&domain.ArticleStat{}).
		Where("article_id = ?", articleID).
		Updates(map[string]interface{}{
			"hot_score_7d":  score,
			"last_hot_calc": gorm.Expr("NOW()"),
			"updated_at":    gorm.Expr("NOW()"),
		}).Error
}

func (d *statDAO) BatchUpdateHotScores(ctx context.Context, window string, scores map[int64]float64) error {
	if len(scores) == 0 {
		return nil
	}

	col := ""
	switch window {
	case "24h":
		col = "hot_score_24h"
	case "7d":
		col = "hot_score_7d"
	default:
		return errors.New("invalid hot score window")
	}

	query := "INSERT INTO sys_article_stat (article_id, " + col + ", last_hot_calc, updated_at) VALUES "
	args := make([]interface{}, 0, len(scores)*2)
	i := 0
	for articleID, score := range scores {
		if i > 0 {
			query += ","
		}
		query += "(?, ?, NOW(), NOW())"
		args = append(args, articleID, score)
		i++
	}
	query += " ON CONFLICT (article_id) DO UPDATE SET " + col + " = EXCLUDED." + col + ", last_hot_calc = NOW(), updated_at = NOW()"

	return d.db.WithContext(ctx).Exec(query, args...).Error
}

// --- Errors ---

var ErrDuplicate = errors.New("duplicate key violation")
