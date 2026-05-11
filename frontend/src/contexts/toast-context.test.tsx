import { render, screen, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ToastProvider, type ToastContextValue } from '@/contexts/toast-context';
import { useToast } from '@/hooks/use-toast';

// ── Helpers ────────────────────────────────────────────────────────

function renderWithToast() {
  let captured: ToastContextValue | null = null;
  function Consumer() {
    captured = useToast();
    return (
      <div>
        <span data-testid="count">{captured!.toasts.length}</span>
        {captured!.toasts.map((t) => (
          <div key={t.id} data-testid={`toast-${t.id}`}>
            {t.message} ({t.type})
          </div>
        ))}
      </div>
    );
  }

  const utils = render(
    <ToastProvider>
      <Consumer />
    </ToastProvider>,
  );

  return { ...utils, getToast: () => captured! };
}

// ── Tests ──────────────────────────────────────────────────────────

describe('ToastProvider', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('addToast adds a toast to the list', () => {
    const { getToast } = renderWithToast();
    const { addToast } = getToast();

    act(() => {
      addToast('Hello world', 'info');
    });

    expect(screen.getByTestId('count').textContent).toBe('1');
    expect(screen.getByText(/Hello world/)).toBeInTheDocument();
  });

  it('removeToast removes a toast by id', () => {
    const { getToast } = renderWithToast();
    const { addToast, removeToast } = getToast();

    act(() => {
      addToast('First', 'info');
    });

    const toastId = getToast().toasts[0].id;

    act(() => {
      removeToast(toastId);
    });

    expect(screen.getByTestId('count').textContent).toBe('0');
  });

  it('auto-dismiss removes toast after duration', () => {
    const { getToast } = renderWithToast();
    const { addToast } = getToast();

    act(() => {
      addToast('Auto dismiss', 'info');
    });

    expect(screen.getByTestId('count').textContent).toBe('1');

    // Advance past the 4000ms TOAST_DURATION
    act(() => {
      vi.advanceTimersByTime(4100);
    });

    expect(screen.getByTestId('count').textContent).toBe('0');
  });

  it('queue is limited to max 5 toasts', () => {
    const { getToast } = renderWithToast();
    const { addToast } = getToast();

    // Add 7 toasts — only the last 5 should remain
    act(() => {
      for (let i = 1; i <= 7; i++) {
        addToast(`Toast ${i}`, 'info');
      }
    });

    expect(screen.getByTestId('count').textContent).toBe('5');
    // Toasts 3-7 should remain (oldest 2 evicted by `.slice(-4)`)
    expect(screen.queryByText(/Toast 1/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Toast 2/)).not.toBeInTheDocument();
    expect(screen.getByText(/Toast 3/)).toBeInTheDocument();
    expect(screen.getByText(/Toast 7/)).toBeInTheDocument();
  });
});
