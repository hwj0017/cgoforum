package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"cgoforum/internal/domain"
	"cgoforum/internal/repository/cache"
	"cgoforum/internal/repository/dao"
	"cgoforum/pkg/bcryptx"
	jwtpkg "cgoforum/pkg/jwt"
	"cgoforum/pkg/snowflake"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrUserBanned       = errors.New("user is banned")
	ErrUsernameExists   = errors.New("username already exists")
	ErrTokenBlacklisted = errors.New("token is blacklisted")
	ErrPermissionDenied = errors.New("permission denied")
)

type AuthService interface {
	Register(ctx context.Context, username, password, nickname string) (*domain.User, error)
	Login(ctx context.Context, username, password string) (accessToken, refreshToken string, err error)
	RefreshToken(ctx context.Context, refreshTokenStr string) (newAccess, newRefresh string, err error)
	Logout(ctx context.Context, userID int64, jti string, accessTTL time.Duration) error
	BanUser(ctx context.Context, adminID int64, userID int64, reason string, duration time.Duration) error
	UnbanUser(ctx context.Context, adminID int64, userID int64) error
}

type authService struct {
	userDAO    dao.UserDAO
	authCache  cache.AuthCache
	jwtHandler *jwtpkg.Handler
	logger     *zap.Logger
}

func NewAuthService(
	userDAO dao.UserDAO,
	authCache cache.AuthCache,
	jwtHandler *jwtpkg.Handler,
	logger *zap.Logger,
) AuthService {
	return &authService{
		userDAO:    userDAO,
		authCache:  authCache,
		jwtHandler: jwtHandler,
		logger:     logger,
	}
}

func (s *authService) Register(ctx context.Context, username, password, nickname string) (*domain.User, error) {
	// Hash password
	hash, err := bcryptx.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		ID:           snowflake.GenerateID(),
		Username:     username,
		PasswordHash: hash,
		Nickname:     nickname,
		Role:         0,
		Status:       0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	created, err := s.userDAO.CreateIfUsernameNotExists(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	if !created {
		return nil, ErrUsernameExists
	}

	return user, nil
}

func (s *authService) Login(ctx context.Context, username, password string) (string, string, error) {
	user, err := s.userDAO.FindByUsername(ctx, username)
	if err != nil {
		return "", "", fmt.Errorf("find user: %w", err)
	}
	if user == nil {
		return "", "", ErrUserNotFound
	}

	// Check password
	if !bcryptx.CheckPassword(password, user.PasswordHash) {
		return "", "", ErrInvalidPassword
	}

	// Check status
	if user.Status == 1 {
		return "", "", ErrUserBanned
	}

	// Generate tokens
	accessToken, err := s.jwtHandler.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	jti := uuid.New().String()
	refreshToken, err := s.jwtHandler.GenerateRefreshToken(user.ID, jti)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	// Update last login time
	if err := s.userDAO.UpdateLastLogin(ctx, user.ID); err != nil {
		s.logger.Error("failed to update last login", zap.Error(err))
	}

	return accessToken, refreshToken, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	claims, err := s.jwtHandler.ParseRefreshToken(refreshTokenStr)
	if err != nil {
		return "", "", fmt.Errorf("parse refresh token: %w", err)
	}

	// Check ban status
	banInfo, err := s.authCache.GetUserBan(ctx, claims.UserID)
	if err != nil {
		s.logger.Error("check ban status error", zap.Error(err))
	}
	if banInfo != "" {
		return "", "", ErrUserBanned
	}

	// Check token blacklist
	blacklisted, err := s.authCache.IsTokenBlacklisted(ctx, claims.JTI)
	if err != nil {
		s.logger.Error("check blacklist error", zap.Error(err))
	}
	if blacklisted {
		return "", "", ErrTokenBlacklisted
	}

	// Get user for role info
	user, err := s.userDAO.FindByID(ctx, claims.UserID)
	if err != nil {
		return "", "", fmt.Errorf("find user: %w", err)
	}
	if user == nil {
		return "", "", ErrUserNotFound
	}
	if user.Status == 1 {
		return "", "", ErrUserBanned
	}

	// Generate new token pair
	newAccessToken, err := s.jwtHandler.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	newJTI := uuid.New().String()
	newRefreshToken, err := s.jwtHandler.GenerateRefreshToken(user.ID, newJTI)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	// Blacklist old refresh token
	if err := s.authCache.SetTokenBlacklist(ctx, claims.JTI, 7*24*time.Hour); err != nil {
		s.logger.Error("blacklist old refresh token error", zap.Error(err))
	}

	return newAccessToken, newRefreshToken, nil
}

func (s *authService) Logout(ctx context.Context, userID int64, jti string, accessTTL time.Duration) error {
	// Blacklist refresh token
	if jti != "" {
		if err := s.authCache.SetTokenBlacklist(ctx, jti, 7*24*time.Hour); err != nil {
			s.logger.Error("blacklist refresh token error", zap.Error(err))
		}
	}

	return nil
}

func (s *authService) BanUser(ctx context.Context, adminID int64, userID int64, reason string, duration time.Duration) error {
	// Set ban in Redis
	if err := s.authCache.SetUserBan(ctx, userID, reason, duration); err != nil {
		return fmt.Errorf("set user ban: %w", err)
	}

	// Update user status in DB
	if err := s.userDAO.UpdateStatus(ctx, userID, 1); err != nil {
		s.logger.Error("update user status error", zap.Error(err))
	}

	return nil
}

func (s *authService) UnbanUser(ctx context.Context, adminID int64, userID int64) error {
	// Remove ban from Redis
	if err := s.authCache.RemoveUserBan(ctx, userID); err != nil {
		return fmt.Errorf("remove user ban: %w", err)
	}

	// Update user status in DB
	if err := s.userDAO.UpdateStatus(ctx, userID, 0); err != nil {
		s.logger.Error("update user status error", zap.Error(err))
	}

	return nil
}
