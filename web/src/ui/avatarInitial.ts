/** The letter shown in place of a picture: the first character of the display name, upper-cased. */
export function initialOf(displayName: string): string {
  const first = [...displayName.trim()][0];
  return first ? first.toUpperCase() : '?';
}
