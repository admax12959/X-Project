import { describe, expect, it } from 'vitest';
import { interpolate, missingParams } from './interpolate.ts';

describe('interpolate', () => {
  it('fills {name} placeholders and keeps unknown ones', () => {
    expect(interpolate('on {line}, {date}', { line: 'main' })).toBe('on main, {date}');
  });
});

describe('missingParams', () => {
  it('lists the placeholders of a message that params cannot fill', () => {
    expect(missingParams('used by {line}, released {releasedAt}', { releasedAt: '2024-03-12' })).toEqual(['line']);
    expect(missingParams('used by {line}', { line: 'x' })).toEqual([]);
    expect(missingParams('no placeholders', undefined)).toEqual([]);
  });
});
