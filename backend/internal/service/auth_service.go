package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"database/sql"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/model"
	"github.com/rhuda/fullstack-boilerplate/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrTokenExpired       = errors.New("refresh token expired")
	ErrTokenInvalid       = errors.New("invalid refresh token")
	ErrTokenReuse         = errors.New("refresh token reuse detected")
)

// dummyHash is a valid bcrypt hash used for constant-time comparison
// when user is not found, preventing email enumeration via timing attacks.
var dummyHash = "$2a$12$00000000000000000000000000000000000000000000000000000"

// AuthServicer defines the interface for authentication service operations.
type AuthServicer interface {
	Login(ctx context.Context, email, password string) (*LoginResult, error)
	Refresh(ctx context.Context, rawToken string) (*RefreshResult, error)
	Logout(ctx context.Context, rawToken string) error
	GetUser(ctx context.Context, userID string) (*model.User, error)
}

type AuthService struct {
	userRepo         repository.UserRepo
	refreshTokenRepo repository.RefreshTokenRepo
	jwtSecret        string
}

func NewAuthService(
	userRepo repository.UserRepo,
	refreshTokenRepo repository.RefreshTokenRepo,
	jwtSecret string,
) *AuthService {
	return &AuthService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		jwtSecret:        jwtSecret,
	}
}

type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         *model.User
	ExpiresAt    time.Time
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Prevent user enumeration via timing attack — perform dummy bcrypt comparison
			// so response time is indistinguishable from a real password check
			_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	accessToken, err := s.generateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	rawRefreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	tokenHash := hashToken(rawRefreshToken)

	tokenID, err := generateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}

	refreshToken := &model.RefreshToken{
		ID:        tokenID,
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := s.refreshTokenRepo.Create(ctx, refreshToken); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		User:         user,
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}, nil
}

type RefreshResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

func (s *AuthService) Refresh(ctx context.Context, rawToken string) (*RefreshResult, error) {
	tokenHash := hashToken(rawToken)

	storedToken, err := s.refreshTokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	if time.Now().After(storedToken.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	// Check if token was already replaced — theft detection
	if storedToken.ReplacedBy != nil {
		slog.Warn("refresh token reuse detected — possible token theft",
			"user_id", storedToken.UserID,
			"token_hash_prefix", tokenHash[:8],
		)
		// Revoke all active tokens for this user
		if revokeErr := s.refreshTokenRepo.RevokeAllForUser(ctx, storedToken.UserID); revokeErr != nil {
			slog.Error("failed to revoke all tokens for user after reuse detection",
				"user_id", storedToken.UserID,
				"error", revokeErr,
			)
		}
		return nil, ErrTokenReuse
	}

	// Check if token was revoked
	if storedToken.RevokedAt != nil {
		return nil, ErrTokenInvalid
	}

	// Get user for new JWT
	user, err := s.userRepo.GetByID(ctx, storedToken.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	accessToken, err := s.generateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate new refresh token (rotation with theft detection)
	newRawToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	newTokenHash := hashToken(newRawToken)
	newTokenID, err := generateUUID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}
	newRefreshToken := &model.RefreshToken{
		ID:        newTokenID,
		UserID:    storedToken.UserID,
		TokenHash: newTokenHash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	replaced, err := s.refreshTokenRepo.Rotate(ctx, tokenHash, newRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
	}
	if !replaced {
		// Token was already replaced — theft detected.
		if revokeErr := s.refreshTokenRepo.RevokeAllForUser(ctx, storedToken.UserID); revokeErr != nil {
			slog.Error("failed to revoke tokens after theft detection",
				"error", revokeErr,
				"user_id", storedToken.UserID,
			)
		}
		slog.Warn("refresh token reuse detected during rotation — possible token theft",
			"user_id", storedToken.UserID,
			"token_hash_prefix", tokenHash[:8],
		)
		return nil, ErrTokenReuse
	}

	return &RefreshResult{
		AccessToken:  accessToken,
		RefreshToken: newRawToken,
		ExpiresAt:    time.Now().Add(15 * time.Minute),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	tokenHash := hashToken(rawToken)
	return s.refreshTokenRepo.DeleteByTokenHash(ctx, tokenHash)
}

func (s *AuthService) GetUser(ctx context.Context, userID string) (*model.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

type accessTokenClaims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func (s *AuthService) generateAccessToken(userID, email string) (string, error) {
	now := time.Now()
	claims := accessTokenClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func (s *AuthService) generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
