// i18n runtime: two flat dictionaries (en, zh), {name} interpolation, no library.
// Every visible string goes through t(). Keys are typed: t('login.title').
// User-entered content (application names, fork reasons, ...) is never translated.
import { useSyncExternalStore } from 'react';
import { en, type MessageKey } from './en/index.ts';
import { zh } from './zh/index.ts';
import { interpolate, missingParams, type Params } from './interpolate.ts';

export type { MessageKey };
export type Locale = 'en' | 'zh';
export const LOCALES: Locale[] = ['zh', 'en'];
export type { Params };

const dictionaries: Record<Locale, Record<MessageKey, string>> = { en, zh };
const STORAGE_KEY = 'xp.locale';
const listeners = new Set<() => void>();

function initialLocale(): Locale {
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved === 'en' || saved === 'zh') return saved;
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en';
}

let current: Locale = initialLocale();
applyDocument();

function applyDocument() {
  // lang drives CJK font selection and line breaking
  document.documentElement.lang = current === 'zh' ? 'zh-CN' : 'en';
  document.title = dictionaries[current]['app.brand'];
}

export function setLocale(l: Locale) {
  current = l;
  localStorage.setItem(STORAGE_KEY, l);
  applyDocument();
  listeners.forEach((cb) => cb());
}

export const getLocale = () => current;

export function hasKey(key: string): key is MessageKey {
  return key in dictionaries.en;
}

export function translate(key: MessageKey, params?: Params): string {
  return interpolate(dictionaries[current][key] ?? key, params);
}

/** Placeholders of the message that params has no value for. */
export function missingMessageParams(key: MessageKey, params?: Params): string[] {
  return missingParams(dictionaries[current][key] ?? key, params);
}

/** Hook: re-renders on locale change. */
export function useT() {
  const locale = useSyncExternalStore(
    (cb) => (listeners.add(cb), () => listeners.delete(cb)),
    () => current,
  );
  return { locale, t: translate };
}
