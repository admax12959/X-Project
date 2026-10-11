import { describe, expect, it } from 'vitest';
import { isOwnerOf } from './roles.ts';

describe('isOwnerOf', () => {
  const app = { owners: [{ id: '1' }, { id: '4' }] };
  it('is true for a listed Owner only', () => {
    expect(isOwnerOf(app, '4')).toBe(true);
    expect(isOwnerOf(app, '2')).toBe(false);
    expect(isOwnerOf({ owners: [] }, '1')).toBe(false);
  });
});
