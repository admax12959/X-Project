import { describe, expect, it } from 'vitest';
import { fill } from './width.ts';

describe('fill', () => {
  it('fills the container by default', () => {
    expect(fill('')).toBe('w-full');
    expect(fill('font-mono')).toBe('w-full');
    expect(fill('max-w-md')).toBe('w-full');
  });

  it('leaves the width to the caller when one is set', () => {
    expect(fill('w-56')).toBe('');
    expect(fill('font-mono w-64')).toBe('');
  });
});
