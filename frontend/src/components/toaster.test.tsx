import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

// ── Mock the useToast hook so we control toasts directly ───────────

const mockRemoveToast = vi.fn();

vi.mock('@/hooks/use-toast', () => ({
  useToast: () => ({
    toasts: [
      { id: 'toast-1', message: 'Info message', type: 'info' as const },
      { id: 'toast-2', message: 'Success message', type: 'success' as const },
      { id: 'toast-3', message: 'Error message', type: 'error' as const },
    ],
    addToast: vi.fn(),
    removeToast: mockRemoveToast,
  }),
}));

import { Toaster } from '@/components/toaster';

// ── Tests ──────────────────────────────────────────────────────────

describe('Toaster', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders toasts from context', () => {
    render(<Toaster />);

    expect(screen.getByText(/Info message/)).toBeInTheDocument();
    expect(screen.getByText(/Success message/)).toBeInTheDocument();
    expect(screen.getByText(/Error message/)).toBeInTheDocument();
  });

  it('renders different toast types', () => {
    render(<Toaster />);

    // All three toast types should be rendered
    const toasts = screen.getAllByRole('generic').filter(
      (el) => el.classList.contains('flex') && el.classList.contains('items-center'),
    );
    expect(toasts).toHaveLength(3);
  });

  it('calls removeToast on dismiss click', () => {
    render(<Toaster />);

    const dismissBtns = screen.getAllByRole('button', { name: /dismiss notification/i });
    fireEvent.click(dismissBtns[0]);

    expect(mockRemoveToast).toHaveBeenCalledWith('toast-1');
  });

  it('has aria-live region for accessibility', () => {
    render(<Toaster />);

    const liveRegion = screen.getByRole('status');
    expect(liveRegion).toBeInTheDocument();
    expect(liveRegion).toHaveAttribute('aria-live', 'polite');
  });
});
