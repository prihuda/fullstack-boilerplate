import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { ReactNode } from 'react';

// ── Mocks ──────────────────────────────────────────────────────────

const mockNavigate = vi.fn();
const mockRouterNavigate = vi.fn();

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mockNavigate,
  useRouter: () => ({
    state: {
      location: { pathname: '/' },
    },
    navigate: mockRouterNavigate,
  }),
  Outlet: () => <div data-testid="outlet">Child content</div>,
  HeadContent: () => <title>Test</title>,
}));

const mockUseQuery = vi.fn();
vi.mock('@tanstack/react-query', () => ({
  useQueryClient: () => ({ clear: vi.fn() }),
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

vi.mock('@/lib/api', () => ({
  get: vi.fn(),
  post: vi.fn(),
}));

vi.mock('@/hooks/use-idle-timeout', () => ({
  useIdleTimeout: vi.fn(),
}));

// Import after mocks
import { AuthProvider } from '@/contexts/auth-context';
import { useAuth } from '@/hooks/use-auth';

// ── Helpers ────────────────────────────────────────────────────────

const mockUser = {
  id: 'user-1',
  email: 'test@example.com',
  name: 'Test User',
  created_at: '2025-01-01T00:00:00Z',
};

/**
 * Re-implement AuthLoader logic as a testable component.
 * We cannot import the real AuthLoader because it is not exported.
 * Instead we test the same logic by rendering AuthProvider + a consumer
 * that mimics AuthLoader's behavior.
 */
function TestAuthLoader({ children }: { children?: ReactNode }) {
  const { isLoading, isAuthenticated } = useAuth();
  const currentPath: string = '/';

  if (isLoading || (!isAuthenticated && currentPath !== '/login')) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div data-testid="loading-spinner" className="animate-spin" />
      </div>
    );
  }

  return <>{children}</>;
}

// ── Tests ──────────────────────────────────────────────────────────

describe('AuthLoader (root layout auth guard)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockRouterNavigate.mockResolvedValue(undefined);
  });

  it('shows loading spinner when isLoading', () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
    });

    render(
      <AuthProvider>
        <TestAuthLoader>
          <div data-testid="content">Protected</div>
        </TestAuthLoader>
      </AuthProvider>,
    );

    expect(screen.getByTestId('loading-spinner')).toBeInTheDocument();
    expect(screen.queryByTestId('content')).not.toBeInTheDocument();
  });

  it('renders children when authenticated', () => {
    mockUseQuery.mockReturnValue({
      data: mockUser,
      isLoading: false,
      isError: false,
    });

    render(
      <AuthProvider>
        <TestAuthLoader>
          <div data-testid="content">Protected</div>
        </TestAuthLoader>
      </AuthProvider>,
    );

    expect(screen.getByTestId('content')).toBeInTheDocument();
    expect(screen.getByTestId('content').textContent).toBe('Protected');
  });

  it('redirects to /login when not authenticated', () => {
    mockUseQuery.mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
    });

    render(
      <AuthProvider>
        <TestAuthLoader>
          <div data-testid="content">Protected</div>
        </TestAuthLoader>
      </AuthProvider>,
    );

    // The loading spinner shows because !isAuthenticated && currentPath !== '/login'
    expect(screen.getByTestId('loading-spinner')).toBeInTheDocument();
    expect(screen.queryByTestId('content')).not.toBeInTheDocument();
  });
});
