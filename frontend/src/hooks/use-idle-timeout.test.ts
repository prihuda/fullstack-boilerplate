import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { useIdleTimeout } from '@/hooks/use-idle-timeout';

describe('useIdleTimeout', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('calls onIdle after timeout with no activity', () => {
    const onIdle = vi.fn();
    renderHook(() => useIdleTimeout(onIdle, 5000));

    expect(onIdle).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(5001);
    });

    expect(onIdle).toHaveBeenCalledTimes(1);
  });

  it('resets timer on activity', () => {
    const onIdle = vi.fn();
    renderHook(() => useIdleTimeout(onIdle, 5000));

    // Advance 3 seconds — not yet idle
    act(() => {
      vi.advanceTimersByTime(3000);
    });

    // Simulate user activity (keydown)
    act(() => {
      window.dispatchEvent(new KeyboardEvent('keydown'));
    });

    // Advance another 3 seconds — still not idle (timer was reset)
    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(onIdle).not.toHaveBeenCalled();

    // Advance another 3 seconds — now past the reset 5s timeout
    act(() => {
      vi.advanceTimersByTime(3000);
    });

    expect(onIdle).toHaveBeenCalledTimes(1);
  });

  it('cleans up on unmount', () => {
    const onIdle = vi.fn();
    const { unmount } = renderHook(() => useIdleTimeout(onIdle, 5000));

    unmount();

    // Advance well past the timeout
    act(() => {
      vi.advanceTimersByTime(10000);
    });

    expect(onIdle).not.toHaveBeenCalled();
  });
});
