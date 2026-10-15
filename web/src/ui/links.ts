// Where a link to an application or one of its lines goes. Every page links through here.

export function appPath(appKey: string): string {
  return `/apps/${encodeURIComponent(appKey)}`;
}

/** The application's Lineage tab with this line selected (`?line=<id>`, the panel opens). */
export function linePath(appKey: string, lineId: string): string {
  return `${appPath(appKey)}/lineage?line=${encodeURIComponent(lineId)}`;
}
