package service

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/rhuda/fullstack-boilerplate/backend/internal/model"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

// mockUserRepo implements repository.UserRepo for testing.
type mockUserRepo struct {
	users map[string]*model.User // keyed by email
	err   error                  // if non-nil, returned for every call
}

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (*model.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	u, ok := m.users[email]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return u, nil
}

func (m *mockUserRepo) GetByID(_ context.Context, id string) (*model.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, sql.ErrNoRows
}

// mockRefreshTokenRepo implements repository.RefreshTokenRepo for testing.
type mockRefreshTokenRepo struct {
	tokens map[string]*model.RefreshToken // keyed by token hash
	err    error
}

func (m *mockRefreshTokenRepo) Create(_ context.Context, token *model.RefreshToken) error {
	if m.err != nil {
		return m.err
	}
	m.tokens[token.TokenHash] = token
	return nil
}

func (m *mockRefreshTokenRepo) GetByTokenHash(_ context.Context, hash string) (*model.RefreshToken, error) {
	if m.err != nil {
		return nil, m.err
	}
	tok, ok := m.tokens[hash]
	if !ok {
		return nil, fmt.Errorf("token not found")
	}
	return tok, nil
}

func (m *mockRefreshTokenRepo) DeleteByTokenHash(_ context.Context, hash string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.tokens, hash)
	return nil
}

func (m *mockRefreshTokenRepo) RevokeAllForUser(_ context.Context, _ string) error {
	return m.err
}

func (m *mockRefreshTokenRepo) Rotate(_ context.Context, oldHash string, newToken *model.RefreshToken) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	old, ok := m.tokens[oldHash]
	if !ok || old.ReplacedBy != nil {
		return false, nil
	}
	replacedByID := newToken.ID
	old.ReplacedBy = &replacedByID
	m.tokens[newToken.TokenHash] = newToken
	return true, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const testJWTSecret = "test-jwt-secret-key-for-testing"

func newTestService(t *testing.T) (*AuthService, *mockUserRepo, *mockRefreshTokenRepo) {
	t.Helper()
	ur := &mockUserRepo{users: make(map[string]*model.User)}
	rtr := &mockRefreshTokenRepo{tokens: make(map[string]*model.RefreshToken)}
	svc := NewAuthService(ur, rtr, testJWTSecret)
	return svc, ur, rtr
}

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)
	return string(h)
}

func makeTestUser(t *testing.T, email, name, password string) *model.User {
	t.Helper()
	return &model.User{
		ID:           "user-1234-abcd",
		Email:        email,
		Name:         name,
		PasswordHash: hashPassword(t, password),
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		UpdatedAt:    time.Now().UTC().Truncate(time.Second),
	}
}



// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestLogin(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		svc, ur, _ := newTestService(t)
		user := makeTestUser(t, "alice@example.com", "Alice", "password123")
		ur.users[user.Email] = user

		result, err := svc.Login(context.Background(), "alice@example.com", "password123")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.NotEmpty(t, result.AccessToken)
		assert.NotEmpty(t, result.RefreshToken)
		assert.Equal(t, user.ID, result.User.ID)
		assert.False(t, result.ExpiresAt.IsZero())
	})

	t.Run("InvalidPassword", func(t *testing.T) {
		t.Parallel()
		svc, ur, _ := newTestService(t)
		user := makeTestUser(t, "bob@example.com", "Bob", "correct-password")
		ur.users[user.Email] = user

		result, err := svc.Login(context.Background(), "bob@example.com", "wrong-password")

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidCredentials)
		assert.Nil(t, result)
	})

	t.Run("UserNotFound", func(t *testing.T) {
		t.Parallel()
		svc, _, _ := newTestService(t)
		// No users in the mock — GetByEmail returns sql.ErrNoRows.

		result, err := svc.Login(context.Background(), "nobody@example.com", "any-password")

		require.Error(t, err)
		// Must be ErrInvalidCredentials, NOT a different error, to prevent email enumeration.
		assert.ErrorIs(t, err, ErrInvalidCredentials)
		assert.Nil(t, result)
	})

	t.Run("RepoError", func(t *testing.T) {
		t.Parallel()
		svc, ur, _ := newTestService(t)
		ur.err = fmt.Errorf("database connection refused")

		result, err := svc.Login(context.Background(), "alice@example.com", "password123")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "get user by email")
		assert.Nil(t, result)
	})
}

func TestRefresh(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		svc, ur, rtr := newTestService(t)
		user := makeTestUser(t, "carol@example.com", "Carol", "pass")
		ur.users[user.Email] = user

		// First, log in to get a refresh token stored in the mock repo.
		loginResult, err := svc.Login(context.Background(), "carol@example.com", "pass")
		require.NoError(t, err)
		require.NotEmpty(t, loginResult.RefreshToken)

		// Now refresh using that token.
		refreshResult, err := svc.Refresh(context.Background(), loginResult.RefreshToken)

		require.NoError(t, err)
		require.NotNil(t, refreshResult)
		assert.NotEmpty(t, refreshResult.AccessToken)
		assert.NotEmpty(t, refreshResult.RefreshToken)
		// The new refresh token should be different from the old one.
		assert.NotEqual(t, loginResult.RefreshToken, refreshResult.RefreshToken)

		// Verify rotation: old token should have ReplacedBy set.
		oldHash := hashToken(loginResult.RefreshToken)
		oldToken := rtr.tokens[oldHash]
		require.NotNil(t, oldToken)
		require.NotNil(t, oldToken.ReplacedBy)
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		t.Parallel()
		svc, ur, rtr := newTestService(t)
		user := makeTestUser(t, "dave@example.com", "Dave", "pass")
		ur.users[user.Email] = user

		// Create an expired refresh token directly in the mock repo.
		expiredHash := hashToken("expired-raw-token")
		rtr.tokens[expiredHash] = &model.RefreshToken{
			ID:        "tok-expired",
			UserID:    user.ID,
			TokenHash: expiredHash,
			ExpiresAt: time.Now().Add(-1 * time.Hour), // expired
			CreatedAt: time.Now().Add(-8 * 24 * time.Hour),
		}

		result, err := svc.Refresh(context.Background(), "expired-raw-token")

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenExpired)
		assert.Nil(t, result)
	})

	t.Run("TokenReuse", func(t *testing.T) {
		t.Parallel()
		svc, ur, rtr := newTestService(t)
		user := makeTestUser(t, "eve@example.com", "Eve", "pass")
		ur.users[user.Email] = user

		// Create a token that has already been replaced (theft detection).
		replacedByID := "tok-new"
		rawToken := "reused-raw-token"
		tokenHash := hashToken(rawToken)
		rtr.tokens[tokenHash] = &model.RefreshToken{
			ID:         "tok-old",
			UserID:     user.ID,
			TokenHash:  tokenHash,
			ReplacedBy: &replacedByID,
			ExpiresAt:  time.Now().Add(6 * 24 * time.Hour),
			CreatedAt:  time.Now().Add(-1 * 24 * time.Hour),
		}

		result, err := svc.Refresh(context.Background(), rawToken)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenReuse)
		assert.Nil(t, result)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		t.Parallel()
		svc, _, _ := newTestService(t)
		// Token not in the repo at all.

		result, err := svc.Refresh(context.Background(), "nonexistent-token")

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenInvalid)
		assert.Nil(t, result)
	})
}

func TestLogout(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		svc, ur, rtr := newTestService(t)
		user := makeTestUser(t, "frank@example.com", "Frank", "pass")
		ur.users[user.Email] = user

		// Log in to store a refresh token.
		loginResult, err := svc.Login(context.Background(), "frank@example.com", "pass")
		require.NoError(t, err)

		// Verify the token exists in the repo.
		tokenHash := hashToken(loginResult.RefreshToken)
		_, ok := rtr.tokens[tokenHash]
		assert.True(t, ok, "token should exist before logout")

		// Logout.
		err = svc.Logout(context.Background(), loginResult.RefreshToken)
		require.NoError(t, err)

		// Token should be deleted.
		_, ok = rtr.tokens[tokenHash]
		assert.False(t, ok, "token should be deleted after logout")
	})

	t.Run("MissingToken", func(t *testing.T) {
		t.Parallel()
		svc, _, _ := newTestService(t)
		// No token in the repo — DeleteByTokenHash is a no-op for missing keys.
		err := svc.Logout(context.Background(), "nonexistent-token")
		// The mock silently succeeds (delete from empty map).
		assert.NoError(t, err)
	})
}

func TestGetUser(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		svc, ur, _ := newTestService(t)
		user := makeTestUser(t, "grace@example.com", "Grace", "pass")
		ur.users[user.Email] = user

		found, err := svc.GetUser(context.Background(), user.ID)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, user.Email, found.Email)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()
		svc, _, _ := newTestService(t)

		found, err := svc.GetUser(context.Background(), "nonexistent-id")

		require.Error(t, err)
		assert.ErrorIs(t, err, sql.ErrNoRows)
		assert.Nil(t, found)
	})
}
