/** The letter shown in place of a picture: the first character of the display name, upper-cased. */
export function initialOf(displayName: string): string {
  const name = displayName.trim();
  // A grapheme cluster keeps an accent or an emoji sequence together; Intl.Segmenter may be missing in old browsers.
  const first =
    typeof Intl !== 'undefined' && 'Segmenter' in Intl
      ? new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(name)[Symbol.iterator]().next().value?.segment
      : [...name][0];
  return first ? first.toUpperCase() : '?';
}
