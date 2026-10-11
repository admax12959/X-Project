// A line reference inside a sentence. Error params and history params carry lines as
// objects {id, isMain, name, branch}; the name is user input and shown verbatim.
import { translate } from './index.ts';

/** True for a line reference object. */
export function isLineRef(v: unknown): v is { isMain: boolean; name?: string } {
  return typeof v === 'object' && v !== null && typeof (v as { isMain?: unknown }).isMain === 'boolean';
}

/** "the mainline" / "主线", or "“name”" / "「name」". */
export function lineInSentence(line: { isMain: boolean; name?: string }): string {
  return line.isMain ? translate('common.line.mainline') : translate('common.line.named', { name: line.name ?? '?' });
}
