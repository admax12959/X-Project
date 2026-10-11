// Change-history events: one "history.event.<type>" per backend EventType (internal/domain/history.go).
// Placeholders are filled by web/src/history/render.ts. User input is shown verbatim.
export default {
  'history.event.appCreated': 'Created application, Owner: {owners}',
  'history.event.versionRegistered': 'Registered version {number} on {line}',
  'history.event.unknown': 'Unknown event ({type})',
  'history.col.time': 'Time',
  'history.col.by': 'By',
  'history.col.event': 'Event',
  'history.empty': 'No changes recorded yet.',
  'history.loadFailed': 'Could not load the change history.',
};
