package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/rhuda/fullstack-boilerplate/backend/internal/model"
	"github.com/uptrace/bun"
)

var ErrTokenReuse = errors.New("refresh token reuse detected")

type RefreshTokenRepository struct {
	db bun.IDB
}

func NewRefreshTokenRepository(db bun.IDB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	_, err := r.db.NewInsert().Model(token).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) GetByTokenHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	token := new(model.RefreshToken)
	err := r.db.NewSelect().Model(token).Where("token_hash = ?", hash).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return token, nil
}

// DeleteByTokenHash deletes a refresh token by its hash.
func (r *RefreshTokenRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := r.db.NewDelete().
		Model(&model.RefreshToken{}).
		Where("token_hash = ?", tokenHash).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}
	return nil
}

// DeleteByUserID deletes all refresh tokens for a user.
func (r *RefreshTokenRepository) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.NewDelete().
		Model(&model.RefreshToken{}).
		Where("user_id = ?", userID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete refresh tokens: %w", err)
	}
	return nil
}

// Rotate replaces an old refresh token with a new one using theft detection.
// It sets replaced_by on the old token atomically (WHERE replaced_by IS NULL).
// If the old token was already replaced (theft), it revokes all tokens for the
// user outside the transaction so revocation persists even on error return.
func (r *RefreshTokenRepository) Rotate(ctx context.Context, oldTokenHash string, newToken *model.RefreshToken) error {
	result, err := r.db.NewUpdate().
		Model((*model.RefreshToken)(nil)).
		Set("replaced_by = ?", newToken.ID).
		Where("token_hash = ? AND replaced_by IS NULL", oldTokenHash).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark old token as replaced: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Token was already replaced — theft detected.
		// Revoke all tokens outside any transaction so revocation persists.
		if revokeErr := r.RevokeAllForUser(ctx, newToken.UserID); revokeErr != nil {
			slog.Error("failed to revoke tokens after theft detection",
				"error", revokeErr,
				"user_id", newToken.UserID,
			)
		}
		return fmt.Errorf("refresh token reuse detected: %s: %w", oldTokenHash[:8], ErrTokenReuse)
	}

	_, err = r.db.NewInsert().Model(newToken).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create new refresh token: %w", err)
	}
	return nil
}

// RevokeAllForUser revokes all active (non-revoked) refresh tokens for a user.
func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := r.db.NewUpdate().
		Model((*model.RefreshToken)(nil)).
		Set("revoked_at = now()").
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to revoke all tokens for user: %w", err)
	}
	return nil
}

// DeleteExpiredAndRevoked removes expired and revoked refresh tokens.
func (r *RefreshTokenRepository) DeleteExpiredAndRevoked(ctx context.Context, db bun.IDB) error {
	_, err := db.NewDelete().
		Model((*model.RefreshToken)(nil)).
		Where("expires_at < now() OR revoked_at IS NOT NULL").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete expired/revoked tokens: %w", err)
	}
	return nil
}
