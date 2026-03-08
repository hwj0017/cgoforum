package domain

import "time"

type User struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id,string"`
	Username     string     `gorm:"uniqueIndex;type:varchar(50);not null" json:"username"`
	PasswordHash string     `gorm:"type:varchar(255);not null" json:"-"`
	Nickname     string     `gorm:"type:varchar(50);not null" json:"nickname"`
	AvatarURL    string     `gorm:"type:varchar(255)" json:"avatar_url"`
	Role         int16      `gorm:"type:smallint;default:0" json:"role"`   // 0:user, 1:admin, 2:super_admin
	Status       int16      `gorm:"type:smallint;default:0" json:"status"` // 0:normal, 1:banned, 2:pending
	CreatedAt    time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"not null" json:"updated_at"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

func (User) TableName() string { return "sys_user" }
