package dao

import (
	"context"
	"errors"
	"strings"

	"cgoforum/internal/domain"
	"gorm.io/gorm"
)

type UserDAO interface {
	Create(ctx context.Context, user *domain.User) error
	CreateIfUsernameNotExists(ctx context.Context, user *domain.User) (bool, error)
	FindByID(ctx context.Context, id int64) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	UpdateLastLogin(ctx context.Context, id int64) error
	UpdateProfile(ctx context.Context, user *domain.User) error
	UpdateStatus(ctx context.Context, id int64, status int16) error
	UpdateRole(ctx context.Context, id int64, role int16) error
}

var ErrUsernameExists = errors.New("username already exists")

type userDAO struct {
	db *gorm.DB
}

func NewUserDAO(db *gorm.DB) UserDAO {
	return &userDAO{db: db}
}

func (d *userDAO) Create(ctx context.Context, user *domain.User) error {
	return d.db.WithContext(ctx).Create(user).Error
}

func (d *userDAO) CreateIfUsernameNotExists(ctx context.Context, user *domain.User) (bool, error) {
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing domain.User
		if err := tx.WithContext(ctx).
			Where("username = ?", user.Username).
			First(&existing).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if existing.ID != 0 {
			return ErrUsernameExists
		}

		if err := tx.WithContext(ctx).Create(user).Error; err != nil {
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
				return ErrUsernameExists
			}
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrUsernameExists) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *userDAO) FindByID(ctx context.Context, id int64) (*domain.User, error) {
	var user domain.User
	if err := d.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (d *userDAO) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	if err := d.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (d *userDAO) UpdateLastLogin(ctx context.Context, id int64) error {
	return d.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", id).
		Update("last_login_at", gorm.Expr("NOW()")).Error
}

func (d *userDAO) UpdateProfile(ctx context.Context, user *domain.User) error {
	return d.db.WithContext(ctx).Model(user).
		Select("nickname", "avatar_url", "updated_at").
		Updates(map[string]interface{}{
			"nickname":    user.Nickname,
			"avatar_url":  user.AvatarURL,
			"updated_at":  gorm.Expr("NOW()"),
		}).Error
}

func (d *userDAO) UpdateStatus(ctx context.Context, id int64, status int16) error {
	return d.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": gorm.Expr("NOW()"),
		}).Error
}

func (d *userDAO) UpdateRole(ctx context.Context, id int64, role int16) error {
	return d.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"role":       role,
			"updated_at": gorm.Expr("NOW()"),
		}).Error
}
