// Change-history rendering: the API sends {type, params}; the sentence is built here
// in the current UI language from the message "history.event.<type>".
// User input inside params (names, reasons...) is shown verbatim, never translated.
//
// To add an event type: see "Change history" in docs/dev.md.
import type { ChangeEvent } from '../api/types.ts';
import { hasKey, translate, type Locale, type MessageKey, type Params } from '../i18n/index.ts';
import { isLineRef, lineInSentence } from '../i18n/lines.ts';

type Renderer = (params: Record<string, unknown>, locale: Locale) => Params;

/** Display name of a user reference {userId, username, displayName}. */
export function userName(ref: unknown): string {
  const r = ref as { displayName?: string; username?: string } | null;
  return r?.displayName || r?.username || '?';
}

/** "A, B" in English, "A、B" in Chinese. */
export function joinNames(names: string[], locale: Locale): string {
  return names.join(locale === 'zh' ? '、' : ', ');
}

function roleName(role: unknown): string {
  return translate(role === 'pe' ? 'term.pe' : 'term.owner');
}

function users(list: unknown, locale: Locale): string {
  return joinNames(Array.isArray(list) ? list.map(userName) : [], locale);
}

/** The line reference {id, isMain, name, branch} as the line was at the time. */
function lineLabel(line: unknown): string {
  return lineInSentence(isLineRef(line) ? line : { isMain: false });
}

function statusName(status: unknown): string {
  return translate(`lineage.status.${status === 'frozen' || status === 'retired' ? status : 'active'}` as MessageKey);
}

/** One entry per backend EventType (internal/domain/history.go): maps params to message placeholders. */
const renderers: Record<string, Renderer> = {
  appCreated: (p, locale) => ({
    owners: users(p.owners, locale),
    // Events written before PEs were recorded have none; the part is left out then.
    pes: Array.isArray(p.pes) && p.pes.length > 0 ? translate('history.appCreated.pes', { pes: users(p.pes, locale) }) : '',
  }),
  deployInfoChanged: () => ({}),
  memberAdded: (p) => ({ role: roleName(p.role), user: userName(p.user) }),
  memberRemoved: (p) => ({ role: roleName(p.role), user: userName(p.user) }),
  appInfoChanged: (p, locale) => ({
    fields: joinNames(
      Object.keys((p.changes as Record<string, unknown> | undefined) ?? {}).map((f) => translate(`apps.field.${f}` as MessageKey)),
      locale,
    ),
  }),
  versionRegistered: (p) => ({ number: String(p.number), line: lineLabel(p.line) }),
  lineForked: (p) => ({ number: String(p.number), line: lineLabel(p.line) }),
  lineChanged: (p, locale) => ({
    line: lineLabel(p.line),
    fields: joinNames(
      Object.keys((p.changes as Record<string, unknown> | undefined) ?? {}).map((f) => translate(`lineage.field.${f}` as MessageKey)),
      locale,
    ),
  }),
  lineStatusChanged: (p) => ({ line: lineLabel(p.line), from: statusName(p.from), to: statusName(p.to) }),
};

export function renderEvent(e: ChangeEvent, locale: Locale): string {
  const key = `history.event.${e.type}`;
  const render = renderers[e.type];
  if (!render || !hasKey(key)) return translate('history.event.unknown', { type: e.type });
  return translate(key, render(e.params, locale));
}
