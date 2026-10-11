import { describe, expect, it } from 'vitest';
import { graphLanes, layoutGraph, type GraphNode } from './lineageGraph.ts';

// Fixtures: a tiny tree DSL, and an ASCII picture of the layout so the expected values are
// readable literals. In the picture lane n sits at column 2n (so n + 0.5 is column 2n + 1):
// '│' a vertical rail, '─' a branch, '├' / '└' where a branch leaves a rail that goes on /
// ends, '◆' mainline, '●' active, '■' frozen, '○' retired, '◌' a line without versions.
type N = GraphNode & { children: N[] };
let seq = 0;
function node(over: Partial<N> = {}, ...children: N[]): N {
  seq += 1;
  return { id: `n${seq}`, isMain: false, status: 'active', versionCount: 1, children, ...over };
}
const main = (...children: N[]) => node({ isMain: true, status: undefined }, ...children);
const fork = (v: string, over: Partial<N> = {}, ...children: N[]) => node({ sourceVersion: { id: v }, ...over }, ...children);

function flatten(root: N, collapsed: Set<string>, depth = 0, out: { node: N; depth: number }[] = []) {
  out.push({ node: root, depth });
  if (!collapsed.has(root.id)) for (const c of root.children) flatten(c, collapsed, depth + 1, out);
  return out;
}

const JOINT: Record<string, string> = { ur: '└', dr: '┌', udr: '├', dlr: '┬', ulr: '┴', udlr: '┼', ul: '┘', dl: '┐', udl: '┤' };
const DOT = { main: '◆', active: '●', frozen: '■', retired: '○' } as const;

function picture(root: N, collapsed = new Set<string>()): string {
  const width = (graphLanes(root) + 1) * 2;
  return layoutGraph(flatten(root, collapsed))
    .map((row) => {
      const cells = Array.from({ length: width }, () => ' ');
      const verticals = row.segments.filter((s) => s.x1 === s.x2);
      const branches = row.segments.filter((s) => s.y1 === 0.5 && s.y2 === 0.5 && s.x1 !== s.x2);
      for (const s of verticals) cells[s.x1 * 2] = '│';
      for (const s of branches) for (let c = Math.min(s.x1, s.x2) * 2; c <= Math.max(s.x1, s.x2) * 2; c++) cells[c] = '─';
      for (let col = 0; col < width; col++) {
        // Joints: where a rail meets a branch, pick the box-drawing character from its four arms.
        const rails = verticals.filter((v) => v.x1 * 2 === col);
        const up = rails.some((v) => Math.min(v.y1, v.y2) === 0);
        const down = rails.some((v) => Math.max(v.y1, v.y2) === 1);
        const left = branches.some((b) => Math.min(b.x1, b.x2) * 2 < col && Math.max(b.x1, b.x2) * 2 >= col);
        const right = branches.some((b) => Math.max(b.x1, b.x2) * 2 > col && Math.min(b.x1, b.x2) * 2 <= col);
        if (rails.length && (left || right)) cells[col] = JOINT[`${up ? 'u' : ''}${down ? 'd' : ''}${left ? 'l' : ''}${right ? 'r' : ''}`] ?? '?';
      }
      cells[row.lane * 2] = row.dot.empty ? '◌' : DOT[row.dot.kind];
      return cells.join('').trimEnd();
    })
    .join('\n');
}

describe('layoutGraph', () => {
  it('a mainline alone is one dot in lane 0', () => {
    expect(picture(main())).toBe('◆');
  });

  it('a fork branches off its parent line', () => {
    expect(picture(main(fork('v1')))).toBe(['◆', '└─●'].join('\n'));
  });

  it('nests to any depth; rails of the ancestors pass through the rows below them', () => {
    const tree = main(fork('v1', {}, fork('v2', {}, fork('v3'))), fork('v4'));
    expect(picture(tree)).toBe(['◆', '├─●', '│ └─●', '│   └─●', '└─●'].join('\n'));
  });

  it('forks from the same version leave the rail at one junction', () => {
    const tree = main(fork('v1'), fork('v1'), fork('v2'));
    expect(picture(tree)).toBe(['◆', '├┬●', '│└●', '└─●'].join('\n'));
  });

  it('forks from different versions leave the rail separately', () => {
    const tree = main(fork('v1'), fork('v2'));
    expect(picture(tree)).toBe(['◆', '├─●', '└─●'].join('\n'));
  });

  it('a junction serves any number of forks and lines nested below them', () => {
    const tree = main(fork('v1', {}, fork('v9')), fork('v1'), fork('v1'));
    expect(picture(tree)).toBe(['◆', '└┬●', ' │└─●', ' ├●', ' └●'].join('\n'));
  });

  it('the shape of the dot tells the status; a line without versions is a ring of its own', () => {
    const tree = main(fork('v1', { status: 'active' }), fork('v2', { status: 'frozen' }), fork('v3', { status: 'retired' }), fork('v4', { versionCount: 0 }), fork('v5', { status: 'retired', versionCount: 0 }));
    expect(picture(tree)).toBe(['◆', '├─●', '├─■', '├─○', '├─◌', '└─◌'].join('\n'));
  });

  it('a line without versions is a leaf: it has no rail down', () => {
    const rows = layoutGraph(flatten(main(fork('v1', { versionCount: 0 })), new Set()));
    expect(rows[1].segments).toEqual([{ x1: 0, y1: 0, x2: 0, y2: 0.5 }, { x1: 0, y1: 0.5, x2: 1, y2: 0.5 }]);
  });

  it('a collapsed line draws no rail down and none for its hidden children', () => {
    const inner = fork('v2', {}, fork('v3'));
    const tree = main(fork('v1', {}, inner), fork('v4'));
    expect(picture(tree, new Set([inner.id]))).toBe(['◆', '├─●', '│ └─●', '└─●'].join('\n'));
    expect(picture(tree, new Set([tree.children[0].id]))).toBe(['◆', '├─●', '└─●'].join('\n'));
  });

  it('the rail ends at the last visible child', () => {
    const last = fork('v2', {}, fork('v3'));
    const tree = main(fork('v1'), last);
    expect(picture(tree, new Set([last.id]))).toBe(['◆', '├─●', '└─●'].join('\n'));
  });

  it('counts lanes over the whole tree, collapsed or not', () => {
    expect(graphLanes(main())).toBe(0);
    expect(graphLanes(main(fork('v1', {}, fork('v2', {}, fork('v3'))), fork('v4')))).toBe(3);
  });

  it('draws the payments-core scenario: four levels, one version forked twice, a line without versions', () => {
    const tree = main(
      fork('1.1.0', { status: 'retired' }), // maint/1.x-pci
      fork('2.1.0', {}, fork('2.1.0-nw.2', { status: 'frozen' }, fork('emea-1.1'))), // cust/northwind > emea > Frankfurt pilot
      fork('2.1.0', { versionCount: 0 }), // cust/helvetia: the same version, no versions yet
      fork('2.3.0'), // release/2.x-lts
    );
    expect(picture(tree)).toBe(['◆', '├─○', '├┬●', '││└─■', '││  └─●', '│└◌', '└─●'].join('\n'));
  });
});
