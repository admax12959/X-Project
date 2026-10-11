// Change-history rendering: the API sends {type, params}; the sentence is built here
// in the current UI language from the message "history.event.<type>".
// User input inside params (names, reasons...) is shown verbatim, never translated.
//
// To add an event type: see "Change history" in docs/dev.md.
import type { ChangeEvent } from '../api/types.ts';
import { hasKey, translate, type Locale, type MessageKey, type Params } from '../i18n/index.ts';

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

function users(list: unknown, locale: Locale): string {
  return joinNames(Array.isArray(list) ? list.map(userName) : [], locale);
}

/** One entry per backend EventType (internal/domain/history.go): maps params to message placeholders. */
const renderers: Record<string, Renderer> = {
  appCreated: (p, locale) => ({ owners: users(p.owners, locale) }),
  appInfoChanged: (p, locale) => ({
    fields: joinNames(
      Object.keys((p.changes as Record<string, unknown> | undefined) ?? {}).map((f) => translate(`apps.field.${f}` as MessageKey)),
      locale,
    ),
  }),
};

export function renderEvent(e: ChangeEvent, locale: Locale): string {
  const key = `history.event.${e.type}`;
  const render = renderers[e.type];
  if (!render || !hasKey(key)) return translate('history.event.unknown', { type: e.type });
  return translate(key, render(e.params, locale));
}
