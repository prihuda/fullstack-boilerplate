import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi } from 'vitest';
import { Input } from '@/components/ui/input';
import type { RefObject } from 'react';

describe('Input', () => {
  it('renders input element', () => {
    render(<Input />);
    expect(screen.getByRole('textbox')).toBeInTheDocument();
  });

  it('applies className', () => {
    render(<Input className="my-custom-class" />);
    const input = screen.getByRole('textbox');
    expect(input.className).toContain('my-custom-class');
  });

  it('forwards ref', () => {
    const ref = { current: null } as unknown as RefObject<HTMLInputElement>;
    render(<Input ref={ref} />);
    expect(ref.current).toBe(screen.getByRole('textbox'));
  });

  it('handles value changes', async () => {
    const onChange = vi.fn();
    render(<Input onChange={onChange} />);

    const input = screen.getByRole('textbox');
    await userEvent.type(input, 'hello');

    expect(onChange).toHaveBeenCalled();
  });
});
