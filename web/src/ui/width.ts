/**
 * Controls fill their container unless the caller sets a width (`w-56`): having both
 * `w-full` and `w-56` in one class list would leave the winner to the CSS order, not to
 * the caller. Returns the class to add to the control's own classes.
 */
export function fill(className: unknown): string {
  return typeof className === 'string' && /(^|\s)w-/.test(className) ? '' : 'w-full';
}
