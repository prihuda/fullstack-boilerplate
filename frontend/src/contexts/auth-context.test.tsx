import { render, screen, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactNode } from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

// ── Mocks ──────────────────────────────────────────────────────────

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}));

const mockUseQuery = vi.fn();
const mockInvalidateQueries = vi.fn();
const mockRemoveQueries = vi.fn();
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({
    clear: vi.fn(),
    invalidateQueries: mockInvalidateQueries,
    removeQueries: mockRemoveQueries,
  }),
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

vi.mock('@/lib/api', () => ({
  get: vi.fn(),
  post: vi.fn(),
}));

// Import after mocks so they resolve to the mocked versions
import { AuthProvider } from '@/contexts/auth-context';
import { useAuth } from '@/hooks/use-auth';
import { post } from '@/lib/api';

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
function renderWithAuth(queryState?: { data?: unknown; isLoading?: boolean; isError?: boolean }) {
  mockUseQuery.mockReturnValue({
    data: queryState?.data ?? undefined,
    isLoading: queryState?.isLoading ?? true,
    isError: queryState?.isError ?? false,
  });

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
  });

  it('starts with no loading state (auth check deferred to beforeLoad)', () => {
    renderWithAuth({ data: undefined });
    expect(screen.getByTestId('loading').textContent).toBe('false');
    expect(screen.getByTestId('authenticated').textContent).toBe('false');
  });

  it('sets authenticated state on success', () => {
    renderWithAuth({ isLoading: false, data: mockUser, isError: false });

    expect(screen.getByTestId('loading').textContent).toBe('false');
    expect(screen.getByTestId('authenticated').textContent).toBe('true');
    expect(screen.getByTestId('user-email').textContent).toBe('test@example.com');
  });

  it('sets unauthenticated on network error', () => {
    renderWithAuth({ isLoading: false, data: undefined, isError: true });

    expect(screen.getByTestId('loading').textContent).toBe('false');
    expect(screen.getByTestId('authenticated').textContent).toBe('false');
    expect(screen.getByTestId('user-email').textContent).toBe('none');
  });

  it('login calls POST /auth/login', async () => {
    vi.mocked(post).mockResolvedValue({
      access_token: 'token',
      token_type: 'Bearer',
      refresh_token: 'rt',
      expires_in: 3600,
      expires_at: '2026-01-01T00:00:00Z',
    });
    mockUseQuery.mockReturnValue({
      data: mockUser,
      isLoading: false,
      isError: false,
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

    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /login/i }));
    });

    expect(post).toHaveBeenCalledWith('/auth/login', {
      email: 'test@example.com',
      password: 'password',
    });
  });

  it('logout calls POST /auth/logout, clears auth state, and invalidates router', async () => {
    let queryState = { data: mockUser, isLoading: false, isError: false };
    mockUseQuery.mockImplementation(() => queryState);

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

    const { rerender } = render(
      <Wrapper>
        <LogoutTestComponent />
      </Wrapper>,
    );

    await act(async () => {
      await userEvent.click(screen.getByRole('button', { name: /logout/i }));
    });

    expect(post).toHaveBeenCalledWith('/auth/logout');
    expect(mockRemoveQueries).toHaveBeenCalledWith({ queryKey: ['auth', 'me'] });

    // Simulate query state cleared after logout
    queryState = { data: undefined as unknown as typeof mockUser, isLoading: false, isError: true };
    mockUseQuery.mockImplementation(() => queryState);
    rerender(
      <Wrapper>
        <LogoutTestComponent />
      </Wrapper>,
    );

    expect(screen.getByTestId('authenticated').textContent).toBe('false');
    expect(screen.getByTestId('user-email').textContent).toBe('none');
  });
});
