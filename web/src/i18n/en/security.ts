// Administration > Security policy (#23): whether every user must have two-step verification.
// "security.field.<name>" is also the label of the changed field in the audit log (the API field names).
export default {
  'security.title': 'Security policy',
  'security.intro': 'Administrator accounts always need two-step verification and cannot turn it off. Here you can require it of everyone. A change takes effect at once, for users who are already logged in too.',
  'security.loadFailed': 'Could not load the security policy.',
  'security.field.forceSecondFactor': 'Force two-step verification for everyone',
  'security.force.label': 'Require two-step verification of every user',
  'security.force.help': 'When on, a user who has not turned it on can only set it up after logging in, and nothing else works until they have. Users who have it cannot turn it off themselves.',
  'security.admins.note': 'Administrators are always required: an administrator account must have two-step verification whatever this switch says.',
  'security.saved': 'Saved.',
  'security.save': 'Save',
  'security.saving': 'Saving…',
  'secondFactor.forced.title': 'Turn on two-step verification first',
  'secondFactor.forced.body': 'Your account must have two-step verification (administrator accounts always do, and an administrator can require it of everyone). Nothing else is available until you have set it up.',
  'secondFactor.forced.continue': 'Continue',
};
