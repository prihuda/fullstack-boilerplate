import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

// ── Mocks ──────────────────────────────────────────────────────────

vi.mock('@tanstack/react-router', () => ({
  useRouter: () => ({
    invalidate: vi.fn().mockResolvedValue(undefined),
  }),
  Outlet: () => <div data-testid="outlet">Child content</div>,
  HeadContent: () => <title>Test</title>,
  createRootRouteWithContext: () => (opts: Record<string, unknown>) => opts,
}));

// Import after mocks — we just verify RootLayout renders without crashing
// since auth guard is now handled by beforeLoad in individual routes.
// RootLayout is a simple passthrough: HeadContent + Outlet.
import { RootLayout } from '@/routes/__root';

// ── Tests ──────────────────────────────────────────────────────────

describe('RootLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders outlet content', () => {
    render(<RootLayout />);

    expect(screen.getByTestId('outlet')).toBeInTheDocument();
    expect(screen.getByTestId('outlet').textContent).toBe('Child content');
  });
});
