// Layout of the optional graph column of the lineage table (#11). Pure: no React, no DOM.
export type GraphNode = {
  id: string;
  isMain: boolean;
  status?: 'active' | 'frozen' | 'retired';
  versionCount: number;
  children: readonly GraphNode[];
  sourceVersion?: { id: string };
};

export type GraphInputRow = { node: GraphNode; depth: number };

export type GraphSegment = { x1: number; y1: number; x2: number; y2: number };

export type GraphRow = {
  id: string;
  lane: number;
  dot: { kind: 'main' | 'active' | 'frozen' | 'retired'; empty: boolean };
  segments: GraphSegment[];
};

/** Deepest lane over the whole tree, collapsed or not, so the column does not jump on collapse. */
export function graphLanes(root: GraphNode): number {
  return root.children.reduce((max, c) => Math.max(max, graphLanes(c) + 1), 0);
}

// Geometry: x is in lanes (a dot of depth d is at x = d), y is the fraction of the row height
// (0 top, 0.5 the dot's centre, 1 bottom). A vertical rail from row a to row b is emitted as
// one segment per row it touches: centre-to-bottom in a, full height in between, top-to-centre
// in b.
function vertical(out: GraphSegment[][], x: number, from: number, to: number) {
  out[from].push({ x1: x, y1: 0.5, x2: x, y2: 1 });
  for (let i = from + 1; i < to; i++) out[i].push({ x1: x, y1: 0, x2: x, y2: 1 });
  out[to].push({ x1: x, y1: 0, x2: x, y2: 0.5 });
}

export function layoutGraph(rows: readonly GraphInputRow[]): GraphRow[] {
  const segments: GraphSegment[][] = rows.map(() => []);

  // Children of each row among the visible rows (a collapsed parent has none).
  const children = new Map<number, number[]>();
  const stack: number[] = [];
  rows.forEach(({ depth }, i) => {
    stack.length = depth;
    if (depth > 0) children.set(stack[depth - 1], [...(children.get(stack[depth - 1]) ?? []), i]);
    stack[depth] = i;
  });

  for (const [parent, kids] of children) {
    const x = rows[parent].depth;
    // Forks from the same version (consecutive siblings) share a junction on the rail.
    const groups: number[][] = [];
    for (const k of kids) {
      const v = rows[k].node.sourceVersion?.id;
      const last = groups[groups.length - 1];
      if (v !== undefined && last && rows[last[0]].node.sourceVersion?.id === v) last.push(k);
      else groups.push([k]);
    }
    vertical(segments, x, parent, groups[groups.length - 1][0]);
    for (const g of groups) {
      if (g.length === 1) {
        segments[g[0]].push({ x1: x, y1: 0.5, x2: x + 1, y2: 0.5 });
        continue;
      }
      segments[g[0]].push({ x1: x, y1: 0.5, x2: x + 0.5, y2: 0.5 });
      vertical(segments, x + 0.5, g[0], g[g.length - 1]);
      for (const k of g) segments[k].push({ x1: x + 0.5, y1: 0.5, x2: x + 1, y2: 0.5 });
    }
  }

  return rows.map(({ node, depth }, i) => ({
    id: node.id,
    lane: depth,
    dot: { kind: node.isMain ? 'main' : (node.status ?? 'active'), empty: !node.isMain && node.versionCount === 0 },
    segments: segments[i],
  }));
}
