// What the UI offers on a line, by status and role. Mirrors the server's state machine
// (internal/domain/line_lifecycle.go); the server stays the authority and re-checks
// everything. Display rules only, no I/O.
import type { LineStatus } from '../api/types.ts';
import type { MessageKey } from '../i18n/index.ts';

const TARGETS: Record<LineStatus, LineStatus[]> = {
  active: ['frozen'],
  frozen: ['active', 'retired'],
  retired: [],
};

/** Statuses a line can move to. The mainline has no status, hence none. */
export function statusTargets(status: LineStatus | undefined): LineStatus[] {
  return status ? TARGETS[status] : [];
}

export type LineRules = {
  /** Status changes to offer (Owners, maintenance lines only). */
  targets: LineStatus[];
  /** Name, note and planned end can be edited (Owners, maintenance lines, not retired). */
  canEdit: boolean;
  canRegister: boolean;
  /** Why Register version is disabled for an Owner, as a message key. */
  registerBlockedReason: MessageKey | null;
  /** A frozen line takes emergency fixes only: the version needs a note. */
  noteRequired: boolean;
};

export function lineRules(line: { isMain: boolean; status?: LineStatus }, isOwner: boolean): LineRules {
  const retired = line.status === 'retired';
  const maintenance = !line.isMain;
  return {
    targets: isOwner && maintenance ? statusTargets(line.status) : [],
    canEdit: isOwner && maintenance && !retired,
    canRegister: isOwner && !retired,
    registerBlockedReason: isOwner && retired ? 'lineage.panel.retiredHint' : null,
    noteRequired: line.status === 'frozen',
  };
}
