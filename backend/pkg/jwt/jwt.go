package jwt

import (
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type AccessClaims struct {
	UserID int64 `json:"user_id"`
	Role   int16 `json:"role"`
	jwtv5.RegisteredClaims
}

type RefreshClaims struct {
	UserID int64 `json:"user_id"`
	JTI    string `json:"jti"`
	jwtv5.RegisteredClaims
}

type Handler struct {
	privateKey   *rsa.PrivateKey
	publicKey    *rsa.PublicKey
	accessExpire time.Duration
	refreshExpire time.Duration
	issuer       string
}

func NewHandler(privateKeyPath, publicKeyPath string, accessExpire, refreshExpire time.Duration, issuer string) (*Handler, error) {
	privBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	privKey, err := jwtv5.ParseRSAPrivateKeyFromPEM(privBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	pubBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	pubKey, err := jwtv5.ParseRSAPublicKeyFromPEM(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	return &Handler{
		privateKey:    privKey,
		publicKey:     pubKey,
		accessExpire:  accessExpire,
		refreshExpire: refreshExpire,
		issuer:        issuer,
	}, nil
}

func (h *Handler) GenerateAccessToken(userID int64, role int16) (string, error) {
	now := time.Now()
	claims := AccessClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwtv5.RegisteredClaims{
			Issuer:    h.issuer,
			ExpiresAt: jwtv5.NewNumericDate(now.Add(h.accessExpire)),
			IssuedAt:  jwtv5.NewNumericDate(now),
		},
	}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, claims)
	return token.SignedString(h.privateKey)
}

func (h *Handler) GenerateRefreshToken(userID int64, jti string) (string, error) {
	now := time.Now()
	claims := RefreshClaims{
		UserID: userID,
		JTI:    jti,
		RegisteredClaims: jwtv5.RegisteredClaims{
			Issuer:    h.issuer,
			ExpiresAt: jwtv5.NewNumericDate(now.Add(h.refreshExpire)),
			IssuedAt:  jwtv5.NewNumericDate(now),
			ID:        jti,
		},
	}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, claims)
	return token.SignedString(h.privateKey)
}

func (h *Handler) ParseAccessToken(tokenString string) (*AccessClaims, error) {
	token, err := jwtv5.ParseWithClaims(tokenString, &AccessClaims{}, func(t *jwtv5.Token) (interface{}, error) {
		// Enforce RS256 algorithm to prevent algorithm confusion attack
		if _, ok := t.Method.(*jwtv5.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.publicKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func (h *Handler) ParseRefreshToken(tokenString string) (*RefreshClaims, error) {
	token, err := jwtv5.ParseWithClaims(tokenString, &RefreshClaims{}, func(t *jwtv5.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtv5.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.publicKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// AccessRemainingTTL returns the remaining duration until the access token expires.
// Used to set blacklist TTL on logout.
func (h *Handler) AccessRemainingTTL(claims *AccessClaims) time.Duration {
	if claims.ExpiresAt == nil {
		return 0
	}
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining < 0 {
		return 0
	}
	return remaining
}
