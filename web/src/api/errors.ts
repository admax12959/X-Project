// Render backend error codes in the current language via "error.<code>" keys.
import { hasKey, translate, type Params } from '../i18n/index.ts';
import { ApiError } from './client.ts';
import type { ApiFieldError } from './types.ts';

function flatParams(p: ApiFieldError['params']): Params {
  const out: Params = {};
  for (const [k, v] of Object.entries(p ?? {})) {
    if (typeof v === 'string' || typeof v === 'number') out[k] = v;
  }
  return out;
}

/** Message for one error entry. */
export function errorMessage(e: ApiFieldError): string {
  const key = `error.${e.code}`;
  return hasKey(key) ? translate(key, flatParams(e.params)) : translate('error.unknown', { code: e.code });
}

/** Map of field → first message, plus "" for request-level errors. Non-API errors become error.internal. */
export function errorsByField(err: unknown): Record<string, string> {
  if (!(err instanceof ApiError)) return { '': translate('error.internal') };
  const out: Record<string, string> = {};
  for (const e of err.errors) {
    const f = e.field ?? '';
    out[f] ??= errorMessage(e);
  }
  return out;
}
