import { describe, expect, it } from 'vitest';
import { descendantIds, moveTargets, pathLabel, treeRows, type DepartmentNode } from './tree.ts';

// Technology > (Payments > Cards, Platform); Risk
const depts: DepartmentNode[] = [
  { id: '1', parentId: null, name: 'Technology' },
  { id: '2', parentId: '1', name: 'Payments' },
  { id: '3', parentId: '2', name: 'Cards' },
  { id: '4', parentId: '1', name: 'Platform' },
  { id: '5', parentId: null, name: 'Risk' },
];

describe('treeRows', () => {
  it('lists parents before children, siblings by name, with their depth', () => {
    expect(treeRows(depts).map((r) => [r.dept.name, r.depth])).toEqual([
      ['Risk', 0],
      ['Technology', 0],
      ['Payments', 1],
      ['Cards', 2],
      ['Platform', 1],
    ]);
  });
  it('treats a department whose parent is unknown as top level', () => {
    expect(treeRows([{ id: '9', parentId: '404', name: 'Lost' }]).map((r) => r.depth)).toEqual([0]);
  });
});

describe('descendantIds', () => {
  it('includes the department itself and everything below it', () => {
    expect([...descendantIds(depts, '1')].sort()).toEqual(['1', '2', '3', '4']);
    expect([...descendantIds(depts, '3')]).toEqual(['3']);
  });
});

describe('pathLabel', () => {
  it('joins the names from the top level down', () => {
    expect(pathLabel(depts, '3')).toBe('Technology / Payments / Cards');
    expect(pathLabel(depts, '5')).toBe('Risk');
  });
  it('is empty for no department or an unknown one', () => {
    expect(pathLabel(depts, null)).toBe('');
    expect(pathLabel(depts, '404')).toBe('');
  });
});

describe('moveTargets', () => {
  it('leaves out the department and its descendants, so a move cannot make a cycle', () => {
    expect(moveTargets(depts, '2').map((r) => r.dept.id)).toEqual(['5', '1', '4']);
    expect(moveTargets(depts, '5').map((r) => r.dept.id)).toEqual(['1', '2', '3', '4']);
  });
});
