// The department tree arrives as a flat list (parentId = null for a top-level department). These pure helpers
// turn it into what the screens need: ordered rows, subtrees and path labels.

export type DepartmentNode = { id: string; parentId: string | null; name: string };
export type TreeRow<T extends DepartmentNode> = { dept: T; depth: number };

const byName = (a: DepartmentNode, b: DepartmentNode) => a.name.localeCompare(b.name) || a.id.localeCompare(b.id, undefined, { numeric: true });

function childrenIndex<T extends DepartmentNode>(depts: T[]): Map<string | null, T[]> {
  const ids = new Set(depts.map((d) => d.id));
  const index = new Map<string | null, T[]>();
  for (const d of depts) {
    // A parent we do not know about makes the department top level rather than losing it.
    const key = d.parentId !== null && ids.has(d.parentId) ? d.parentId : null;
    index.set(key, [...(index.get(key) ?? []), d]);
  }
  for (const list of index.values()) list.sort(byName);
  return index;
}

/** Depth-first rows: every department after its parent, siblings by name. */
export function treeRows<T extends DepartmentNode>(depts: T[]): TreeRow<T>[] {
  const index = childrenIndex(depts);
  const rows: TreeRow<T>[] = [];
  const visit = (parent: string | null, depth: number) => {
    for (const dept of index.get(parent) ?? []) {
      rows.push({ dept, depth });
      visit(dept.id, depth + 1);
    }
  };
  visit(null, 0);
  return rows;
}

/** The department and every department below it, at any depth. */
export function descendantIds(depts: DepartmentNode[], id: string): Set<string> {
  const index = childrenIndex(depts);
  const out = new Set<string>();
  const visit = (current: string) => {
    out.add(current);
    for (const child of index.get(current) ?? []) if (!out.has(child.id)) visit(child.id);
  };
  visit(id);
  return out;
}

/** "Technology / Payments / Cards"; empty for no department or an unknown one. */
export function pathLabel(depts: DepartmentNode[], id: string | null, separator = ' / '): string {
  const byId = new Map(depts.map((d) => [d.id, d]));
  const names: string[] = [];
  const seen = new Set<string>();
  for (let cur = id !== null ? byId.get(id) : undefined; cur && !seen.has(cur.id); cur = cur.parentId !== null ? byId.get(cur.parentId) : undefined) {
    seen.add(cur.id);
    names.unshift(cur.name);
  }
  return names.join(separator);
}

/** Where a department can move: everything except itself and its descendants. */
export function moveTargets<T extends DepartmentNode>(depts: T[], movingId: string): TreeRow<T>[] {
  const excluded = descendantIds(depts, movingId);
  return treeRows(depts.filter((d) => !excluded.has(d.id)));
}
