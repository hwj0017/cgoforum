package domain

import (
	"time"

	"github.com/pgvector/pgvector-go"
)

type Article struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id,string"`
	UserID      int64      `gorm:"not null;index" json:"user_id,string"`
	Title       string     `gorm:"type:varchar(200);not null" json:"title"`
	Summary     string     `gorm:"type:varchar(500)" json:"summary"`
	ContentMD   string     `gorm:"type:text;not null" json:"content_md"`
	CoverImg    string     `gorm:"type:varchar(500)" json:"cover_img"`
	Status      int16      `gorm:"type:smallint;default:1" json:"status"` // 0:draft, 1:published, 2:reviewing, 3:blocked
	IsTop       bool       `gorm:"default:false" json:"is_top"`
	CreatedAt   time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"not null" json:"updated_at"`
	PublishedAt *time.Time `json:"published_at"`

	// Associations (not stored in DB directly, loaded via joins)
	User *User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Stat *ArticleStat `gorm:"foreignKey:ArticleID" json:"stat,omitempty"`
}

func (Article) TableName() string { return "sys_article" }

type ArticleStat struct {
	ArticleID    int64      `gorm:"primaryKey;foreignKey:ArticleID;references:ID;OnDelete:CASCADE" json:"article_id,string"`
	ViewCount    int64      `gorm:"default:0" json:"view_count"`
	LikeCount    int64      `gorm:"default:0" json:"like_count"`
	CollectCount int64      `gorm:"default:0" json:"collect_count"`
	CommentCount int64      `gorm:"default:0" json:"comment_count"`
	HotScore24h  *float64   `gorm:"type:decimal(10,4)" json:"hot_score_24h"`
	HotScore7d   *float64   `gorm:"type:decimal(10,4)" json:"hot_score_7d"`
	LastHotCalc  *time.Time `json:"last_hot_calc"`
	UpdatedAt    time.Time  `gorm:"not null" json:"updated_at"`
}

func (ArticleStat) TableName() string { return "sys_article_stat" }

type ArticleEmbedding struct {
	ArticleID    int64           `gorm:"primaryKey;foreignKey:ArticleID;references:ID;OnDelete:CASCADE" json:"article_id,string"`
	ChunkID      string          `gorm:"primaryKey;type:varchar(50)" json:"chunk_id"`
	Embedding    pgvector.Vector `gorm:"type:vector(768)" json:"-"`
	SectionTitle string          `gorm:"type:varchar(200)" json:"section_title"`
	ContentText  string          `gorm:"type:text;not null" json:"content_text"`
	ModelVersion string          `gorm:"type:varchar(20);not null" json:"model_version"`
	UpdatedAt    time.Time       `gorm:"not null" json:"updated_at"`
}

func (ArticleEmbedding) TableName() string { return "sys_article_embedding" }
