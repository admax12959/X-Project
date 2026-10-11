// Product names are not translated.
import type { PlatformKind, Repo } from '../api/types.ts';

export const PLATFORM_LABEL: Record<PlatformKind, string> = { gitlab: 'GitLab', github: 'GitHub' };

/** "GitLab · group/repo" */
export function repoLabel(repo: Pick<Repo, 'kind' | 'path'>): string {
  return `${PLATFORM_LABEL[repo.kind] ?? repo.kind} · ${repo.path}`;
}
