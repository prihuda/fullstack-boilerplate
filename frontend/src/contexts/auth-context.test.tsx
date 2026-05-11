import { render, screen, act, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactNode } from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

// ── Mocks ──────────────────────────────────────────────────────────

const mockNavigate = vi.fn();
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({
    clear: vi.fn(),
    invalidateQueries: vi.fn(),
  }),
}));

vi.mock('@/lib/api', () => ({
  get: vi.fn(),
  post: vi.fn(),
}));

// Import after mocks so they resolve to the mocked versions
import { AuthProvider } from '@/contexts/auth-context';
import { useAuth } from '@/hooks/use-auth';
import { get, post } from '@/lib/api';

// ── Helpers ────────────────────────────────────────────────────────

const mockUser = {
  id: 'user-1',
  email: 'test@example.com',
  name: 'Test User',
  created_at: '2025-01-01T00:00:00Z',
};

function Wrapper({ children }: { children: ReactNode }) {
  return <AuthProvider>{children}</AuthProvider>;
}

/** Helper to render a child that reads the auth context. */
function renderWithAuth() {
  function Consumer() {
    const auth = useAuth();
    return (
      <div data-testid="auth-state">
        <span data-testid="loading">{String(auth.isLoading)}</span>
        <span data-testid="authenticated">{String(auth.isAuthenticated)}</span>
        <span data-testid="user-email">{auth.user?.email ?? 'none'}</span>
      </div>
    );
  }

  return render(
    <Wrapper>
      <Consumer />
    </Wrapper>,
  );
}

// ── Tests ──────────────────────────────────────────────────────────

describe('AuthProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
  });

  it('provides loading state initially', () => {
    // Keep the /auth/me request pending so loading stays true
    vi.mocked(get).mockReturnValue(new Promise(() => {}));
    renderWithAuth();

    expect(screen.getByTestId('loading').textContent).toBe('true');
    expect(screen.getByTestId('authenticated').textContent).toBe('false');
  });

  it('checkAuth sets authenticated state on success', async () => {
    vi.mocked(get).mockResolvedValue(mockUser);
    renderWithAuth();

    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false');
    });
    expect(screen.getByTestId('authenticated').textContent).toBe('true');
    expect(screen.getByTestId('user-email').textContent).toBe('test@example.com');
  });

  it('checkAuth sets unauthenticated on network error', async () => {
    vi.mocked(get).mockRejectedValue(new Error('Network error'));
    renderWithAuth();

    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false');
    });
    expect(screen.getByTestId('authenticated').textContent).toBe('false');
    expect(screen.getByTestId('user-email').textContent).toBe('none');
  });

  it('login calls POST /auth/login then checkAuth', async () => {
    vi.mocked(get).mockResolvedValue(mockUser);
    vi.mocked(post).mockResolvedValue({
      access_token: 'token',
      token_type: 'Bearer',
      refresh_token: 'rt',
      expires_in: 3600,
      expires_at: '2026-01-01T00:00:00Z',
    });

    function LoginTestComponent() {
      const auth = useAuth();
      return (
        <>
          <div data-testid="auth-state">
            <span data-testid="loading">{String(auth.isLoading)}</span>
            <span data-testid="authenticated">{String(auth.isAuthenticated)}</span>
            <span data-testid="user-email">{auth.user?.email ?? 'none'}</span>
          </div>
          <button onClick={() => void auth.login('test@example.com', 'password')}>Login</button>
        </>
      );
    }

    render(
      <Wrapper>
        <LoginTestComponent />
      </Wrapper>,
    );

    // Wait for initial checkAuth to complete
    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false');
    });

    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /login/i }));
    });

    expect(post).toHaveBeenCalledWith('/auth/login', {
      email: 'test@example.com',
      password: 'password',
    });
    // checkAuth is called after login (showLoading: false, so get is called again)
    expect(get).toHaveBeenCalledTimes(2); // initial + after login
  });

  it('logout calls POST /auth/logout, clears state, navigates to /login', async () => {
    vi.mocked(get).mockResolvedValue(mockUser);
    vi.mocked(post).mockResolvedValue(undefined);

    function LogoutTestComponent() {
      const auth = useAuth();
      return (
        <>
          <div data-testid="auth-state">
            <span data-testid="loading">{String(auth.isLoading)}</span>
            <span data-testid="authenticated">{String(auth.isAuthenticated)}</span>
            <span data-testid="user-email">{auth.user?.email ?? 'none'}</span>
          </div>
          <button onClick={() => void auth.logout()}>Logout</button>
        </>
      );
    }

    render(
      <Wrapper>
        <LogoutTestComponent />
      </Wrapper>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('authenticated').textContent).toBe('true');
    });

    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /logout/i }));
    });

    expect(post).toHaveBeenCalledWith('/auth/logout');
    expect(screen.getByTestId('authenticated').textContent).toBe('false');
    expect(screen.getByTestId('user-email').textContent).toBe('none');
    expect(mockNavigate).toHaveBeenCalledWith({ to: '/login' });
  });
});
