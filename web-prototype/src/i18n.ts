// PROTOTYPE i18n: two locales (en, zh), flat message keys, no library.
// Not a decision about the real app's i18n stack; it exists so both design directions can be judged in both languages.
import { useSyncExternalStore } from 'react';

export type Locale = 'en' | 'zh';

/** Seeded content (names, reasons, notes). Real user-typed content is a plain string and is never translated. */
export type Text = string | { en: string; zh: string };

/** A system-generated message: stored as data, rendered through the dictionary so it follows the locale. */
export type Msg = { key: string; params?: Record<string, string | number | Text | Msg> };

type Entry = string | { one: string; other: string };
export type Dict = Record<string, Entry>;

const KEY = 'proto:locale';
const listeners = new Set<() => void>();
const messages: Record<Locale, Dict> = { en: {}, zh: {} };

/** Each direction registers its own page strings under its own prefix (e.g. `console.`); shared terms live in locales/core.ts. */
export function registerMessages(en: Dict, zh: Dict) {
  Object.assign(messages.en, en);
  Object.assign(messages.zh, zh);
}

function read(): Locale {
  const v = localStorage.getItem(KEY);
  if (v === 'en' || v === 'zh') return v;
  return 'en';
}
let current: Locale = read();
document.documentElement.lang = current === 'zh' ? 'zh-CN' : 'en';

export function setLocale(l: Locale) {
  current = l;
  localStorage.setItem(KEY, l);
  // lang drives CJK font selection and line breaking
  document.documentElement.lang = l === 'zh' ? 'zh-CN' : 'en';
  listeners.forEach((cb) => cb());
}
export const getLocale = () => current;

const warned = new Set<string>();

/** Non-hook versions: usable in data code. Components should call useT() so they re-render on locale change. */
export function resolveText(x: Text | undefined): string {
  if (x === undefined) return '';
  return typeof x === 'string' ? x : x[current];
}

export function translate(key: string, params?: Msg['params']): string {
  const e = messages[current][key];
  if (e === undefined) {
    if (!warned.has(`${current}:${key}`)) {
      warned.add(`${current}:${key}`);
      console.warn(`[i18n] missing ${current} message: ${key}`);
    }
    return key;
  }
  const raw = typeof e === 'string' ? e : Number(params?.n) === 1 ? e.one : e.other;
  return raw.replace(/\{(\w+)\}/g, (_, name) => {
    const p = params?.[name];
    if (p === undefined) return `{${name}}`;
    if (typeof p === 'object' && 'key' in p) return translate(p.key, p.params);
    return typeof p === 'object' ? resolveText(p) : String(p);
  });
}
export const renderMsg = (m: Msg) => translate(m.key, m.params);

const intlLocale = () => (current === 'zh' ? 'zh-CN' : 'en-US');
/** '2026-08-30' → "Aug 30, 2026" / "2026年8月30日". Dates stay ISO in the data. */
export function fmtDate(iso: string): string {
  const d = new Date(`${iso.slice(0, 10)}T00:00:00Z`);
  return new Intl.DateTimeFormat(intlLocale(), { year: 'numeric', month: 'short', day: 'numeric', timeZone: 'UTC' }).format(d);
}
/** '2026-09-21 16:02' → date and time in the locale's order. */
export function fmtDateTime(iso: string): string {
  return `${fmtDate(iso)} ${iso.slice(11, 16)}`;
}

export function useT() {
  const locale = useSyncExternalStore(
    (cb) => (listeners.add(cb), () => listeners.delete(cb)),
    () => current,
  );
  return {
    locale,
    /** Look up a message: t('status.Active'), t('n.versions', { n: 3 }). */
    t: translate,
    /** Resolve seeded content that may be bilingual. Always wrap Application/Line/Version text fields in this. */
    tx: resolveText,
    /** Render a stored Msg (validation errors, change-history events, boundBy). */
    tm: renderMsg,
    fmtDate,
    fmtDateTime,
  };
}
