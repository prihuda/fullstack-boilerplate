package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/rhuda/fullstack-boilerplate/backend/internal/model"
	"github.com/uptrace/bun"
)

// RefreshTokenRepo defines the interface for refresh token repository operations.
type RefreshTokenRepo interface {
	Create(ctx context.Context, token *model.RefreshToken) error
	GetByTokenHash(ctx context.Context, hash string) (*model.RefreshToken, error)
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	RevokeAllForUser(ctx context.Context, userID string) error
	Rotate(ctx context.Context, oldTokenHash string, newToken *model.RefreshToken) (bool, error)
}

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

// Rotate replaces an old refresh token with a new one atomically.
// It sets replaced_by on the old token (WHERE replaced_by IS NULL) and inserts
// the new token in a single transaction to prevent TOCTOU races.
func (r *RefreshTokenRepository) Rotate(ctx context.Context, oldTokenHash string, newToken *model.RefreshToken) (bool, error) {
	var replaced bool
	err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		result, err := tx.NewUpdate().
			Model((*model.RefreshToken)(nil)).
			Set("replaced_by = ?", newToken.ID).
			Where("token_hash = ?", oldTokenHash).
			Where("replaced_by IS NULL").
			Exec(ctx)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			replaced = false
			return nil
		}
		replaced = true
		_, err = tx.NewInsert().Model(newToken).Exec(ctx)
		return err
	})
	if err != nil {
		return false, fmt.Errorf("rotate token: %w", err)
	}
	return replaced, nil
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
