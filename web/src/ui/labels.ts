// Product names are not translated.
import type { PlatformKind, RefKind, Repo } from '../api/types.ts';
import type { MessageKey } from '../i18n/index.ts';

export const PLATFORM_LABEL: Record<PlatformKind, string> = { gitlab: 'GitLab', github: 'GitHub' };

/** "GitLab · group/repo" */
export function repoLabel(repo: Pick<Repo, 'kind' | 'path'>): string {
  return `${PLATFORM_LABEL[repo.kind] ?? repo.kind} · ${repo.path}`;
}

/** The message for a version reference kind: "Tag" or "Commit". */
export function refKindKey(kind: RefKind): MessageKey {
  return kind === 'tag' ? 'versions.ref.tag' : 'versions.ref.commit';
}
