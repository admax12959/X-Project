import { describe, expect, it } from 'vitest';
import { lineRules, statusTargets } from './lineLifecycle.ts';

describe('statusTargets', () => {
  it('follows the state machine: active -> frozen, frozen -> active or retired, retired is final', () => {
    expect(statusTargets('active')).toEqual(['frozen']);
    expect(statusTargets('frozen')).toEqual(['active', 'retired']);
    expect(statusTargets('retired')).toEqual([]);
  });

  it('offers nothing on the mainline, which has no status', () => {
    expect(statusTargets(undefined)).toEqual([]);
  });
});

describe('lineRules', () => {
  const line = (status: 'active' | 'frozen' | 'retired' | undefined) => ({ isMain: status === undefined, status });

  it('lets an Owner change and edit an active line, with the note optional', () => {
    expect(lineRules(line('active'), true)).toEqual({
      targets: ['frozen'],
      canEdit: true,
      canRegister: true,
      registerBlockedReason: null,
      noteRequired: false,
    });
  });

  it('requires a note on a frozen line', () => {
    const r = lineRules(line('frozen'), true);
    expect(r.noteRequired).toBe(true);
    expect(r.canRegister).toBe(true);
    expect(r.canEdit).toBe(true);
    expect(r.targets).toEqual(['active', 'retired']);
  });

  it('makes a retired line read-only and explains why registering is disabled', () => {
    const r = lineRules(line('retired'), true);
    expect(r.targets).toEqual([]);
    expect(r.canEdit).toBe(false);
    expect(r.canRegister).toBe(false);
    expect(r.registerBlockedReason).toBe('lineage.panel.retiredHint');
  });

  it('gives the mainline no status operations and no edits, but it takes versions', () => {
    const r = lineRules(line(undefined), true);
    expect(r.targets).toEqual([]);
    expect(r.canEdit).toBe(false);
    expect(r.canRegister).toBe(true);
    expect(r.noteRequired).toBe(false);
  });

  it('gives a non-Owner no operations at all', () => {
    const r = lineRules(line('active'), false);
    expect(r.targets).toEqual([]);
    expect(r.canEdit).toBe(false);
    expect(r.canRegister).toBe(false);
  });
});
