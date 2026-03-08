package domain

import "time"

type Follow struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id,string"`
	FollowerID int64     `gorm:"not null;uniqueIndex:idx_follow_pair" json:"follower_id,string"`
	AuthorID   int64     `gorm:"not null;uniqueIndex:idx_follow_pair" json:"author_id,string"`
	CreatedAt  time.Time `gorm:"not null;index:idx_follower_time,priority:2,sort:desc" json:"created_at"`
}

func (Follow) TableName() string { return "sys_follow" }

type Like struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id,string"`
	UserID    int64     `gorm:"not null;uniqueIndex:idx_like_pair" json:"user_id,string"`
	ArticleID int64     `gorm:"not null;uniqueIndex:idx_like_pair;index" json:"article_id,string"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
}

func (Like) TableName() string { return "sys_like" }

type Collect struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id,string"`
	UserID    int64     `gorm:"not null;uniqueIndex:idx_collect_pair" json:"user_id,string"`
	ArticleID int64     `gorm:"not null;uniqueIndex:idx_collect_pair;index" json:"article_id,string"`
	Note      string    `gorm:"type:text" json:"note"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
}

func (Collect) TableName() string { return "sys_collect" }

type EventLog struct {
	EventID    string    `gorm:"type:varchar(64);primaryKey" json:"event_id"`
	Consumer   string    `gorm:"type:varchar(64);primaryKey" json:"consumer"`
	OccurredAt time.Time `gorm:"not null" json:"occurred_at"`
	CreatedAt  time.Time `gorm:"not null;autoCreateTime" json:"created_at"`
}

func (EventLog) TableName() string { return "sys_event_log" }
