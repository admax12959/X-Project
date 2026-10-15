import { describe, expect, it } from 'vitest';
import { initialOf } from './avatarInitial.ts';

describe('initialOf', () => {
  it('upper-cases the first letter of a Latin name', () => {
    expect(initialOf('emma larsen')).toBe('E');
  });
  it('takes the first character of a Chinese name', () => {
    expect(initialOf('  王小明')).toBe('王');
  });
  it('keeps a character outside the BMP whole', () => {
    expect(initialOf('𠮷野')).toBe('𠮷');
  });
  it('falls back to a question mark for an empty name', () => {
    expect(initialOf('  ')).toBe('?');
  });
});
