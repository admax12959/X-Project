// Render backend error codes in the current language via "error.<code>" keys.
import { hasKey, missingMessageParams, translate, type Params } from '../i18n/index.ts';
import { isLineRef, lineInSentence } from '../i18n/lines.ts';
import { ApiError } from './client.ts';
import type { ApiFieldError } from './types.ts';

function flatParams(p: ApiFieldError['params']): Params {
  const out: Params = {};
  for (const [k, v] of Object.entries(p ?? {})) {
    if (typeof v === 'string' || typeof v === 'number') out[k] = v;
    // A conflicting line {id, isMain, name, branch} (numberTaken, later branchTaken) becomes
    // its in-sentence label, so messages can say "{line}".
    else if (isLineRef(v)) out[k] = lineInSentence(v);
    // A list of objects with a name (soleOwner's applications) reads as "A, B". Other nested params are dropped.
    else if (Array.isArray(v)) out[k] = v.map((x) => (x as { name?: unknown } | null)?.name ?? '').join(', ');
  }
  return out;
}

/**
 * Message for one error entry. When the server sent no value for a placeholder of the
 * message (a conflict whose other party could not be named), the wording without
 * placeholders, `error.<code>Generic`, is used instead of showing "{line}".
 */
export function errorMessage(e: ApiFieldError): string {
  const key = `error.${e.code}`;
  if (!hasKey(key)) return translate('error.unknown', { code: e.code });
  const params = flatParams(e.params);
  const generic = `${key}Generic`;
  return translate(hasKey(generic) && missingMessageParams(key, params).length > 0 ? generic : key, params);
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
