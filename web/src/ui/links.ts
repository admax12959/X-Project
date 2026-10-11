// Where a link to an application or one of its lines goes. Every page links through here.

export function appPath(appKey: string): string {
  return `/apps/${encodeURIComponent(appKey)}`;
}

/**
 * The application's lineage with this line selected (prototype convention `?line=<id>`).
 * The Lineage tab arrives with #9; until `apps/:key/lineage` is a route this goes to the
 * application page. Switch the target here, and nowhere else, when the tab exists.
 */
export function linePath(appKey: string, _lineId: string): string {
  return appPath(appKey);
}
