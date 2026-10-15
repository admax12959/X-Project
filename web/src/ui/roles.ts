// Display-only role checks. The server decides what is allowed; these only choose what to offer.
import type { Member } from '../api/types.ts';

/** Whether the user is one of the application's Owners. */
export function isOwnerOf(app: { owners: Pick<Member, 'id'>[] }, userId: string): boolean {
  return app.owners.some((o) => o.id === userId);
}
