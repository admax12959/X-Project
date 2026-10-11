// PROTOTYPE fake data + in-memory store. Reload = reset. Nothing persists.
// UI copy is English and uses the GLOSSARY.md English names (Application Key, Maintenance Line, Source Version, ...).
import { useSyncExternalStore } from 'react';

export const TODAY = '2026-10-10';

export type Role = 'admin' | 'owner' | 'pe';
export type LineStatus = 'Active' | 'Frozen' | 'Retired';
export type IdentityType = 'ldap' | 'local';

export type LoginIdentity = { type: IdentityType; login: string; boundAt: string; boundBy: string };
export type User = {
  username: string;
  displayName: string;
  email: string;
  phone?: string;
  dept?: string;
  status: 'Enabled' | 'Disabled';
  identities: LoginIdentity[];
};

export type CodePlatform = { id: string; type: 'GitLab' | 'GitHub'; url: string; tokenTail: string; addedAt: string };

export type Application = {
  key: string;
  name: string;
  description: string;
  repo: { platformId: string; path: string; projectId: string };
  mainBranch: string;
  owners: string[];
  pes: string[];
  deployInfo: string;
};

export type Line = {
  id: string;
  appKey: string;
  isMain: boolean;
  name: string;
  branch: string;
  sourceVersionId?: string;
  reason?: string;
  status: LineStatus;
  note?: string;
};

export type Version = {
  id: string;
  lineId: string;
  number: string;
  ref: { kind: 'tag' | 'commit'; value: string };
  releasedAt: string;
  note: string;
};

export type AuditEntry = { appKey: string; at: string; actor: string; text: string };

type State = {
  role: Role;
  me: string;
  users: User[];
  platforms: CodePlatform[];
  apps: Application[];
  lines: Line[];
  versions: Version[];
  audit: AuditEntry[];
};

// ---------- seed ----------

let vid = 0;
const versions: Version[] = [];
function v(lineId: string, number: string, releasedAt: string, ref: Version['ref'] = { kind: 'tag', value: number.startsWith('v') ? number : `v${number}` }, note = '') {
  const id = `ver${++vid}`;
  versions.push({ id, lineId, number, ref, releasedAt, note });
  return id;
}

const lines: Line[] = [];
function line(l: Omit<Line, 'isMain'> & { isMain?: boolean }) {
  lines.push({ isMain: false, ...l });
  return l.id;
}

// payments-core: the lineage stress case (4 levels deep, every status, a line with no versions, a date-style version number)
line({ id: 'payments-core:main', appKey: 'payments-core', isMain: true, name: 'Mainline', branch: 'main', status: 'Active' });
v('payments-core:main', '1.0.0', '2024-03-12', undefined, 'First general release');
const b110 = v('payments-core:main', '1.1.0', '2024-06-20');
v('payments-core:main', '2.0.0', '2024-11-05', undefined, 'New ledger model');
const b210 = v('payments-core:main', '2.1.0', '2025-02-18');
v('payments-core:main', '2.2.0', '2025-05-27');
const b230 = v('payments-core:main', '2.3.0', '2025-08-14');
v('payments-core:main', '3.0.0', '2026-01-09', undefined, 'Switch to the new settlement engine');
v('payments-core:main', '3.1.0', '2026-04-22');
v('payments-core:main', '3.2.0', '2026-08-30');

line({ id: 'payments-core:pci-1x', appKey: 'payments-core', name: '1.x PCI freeze', branch: 'maint/1.x-pci', sourceVersionId: b110, reason: 'Held at 1.1 during the 2024 PCI DSS assessment; security fixes only until sign-off.', status: 'Retired', note: 'Assessment passed in May 2025; all customers have moved to 2.x.' });
v('payments-core:pci-1x', '1.1.1', '2024-08-02', { kind: 'commit', value: '4e1a9c2' }, 'Fix broken access control on statement queries');
v('payments-core:pci-1x', '1.1.2', '2024-12-11');

line({ id: 'payments-core:northwind', appKey: 'payments-core', name: 'Northwind customization', branch: 'cust/northwind', sourceVersionId: b210, reason: 'The Northwind contract requires a custom statement format and settlement cycle. Mainline will not take these changes.', status: 'Active', note: 'Contract renewal is being negotiated.' });
v('payments-core:northwind', '2.1.0-nw.1', '2025-04-03');
const nw2 = v('payments-core:northwind', '2.1.0-nw.2', '2025-07-15');
v('payments-core:northwind', '2.1.0-nw.3', '2026-02-26');

line({ id: 'payments-core:northwind-emea', appKey: 'payments-core', name: 'Northwind EMEA', branch: 'cust/northwind-emea', sourceVersionId: nw2, reason: 'The EMEA subsidiary needs SEPA-formatted reports, which conflict with the Northwind HQ customization.', status: 'Frozen' });
v('payments-core:northwind-emea', 'emea-1.0', '2025-10-20', { kind: 'tag', value: 'northwind-emea-1.0' });
const emea11 = v('payments-core:northwind-emea', 'emea-1.1', '2026-01-30', { kind: 'commit', value: 'b71d0e8' });

line({ id: 'payments-core:northwind-emea-de', appKey: 'payments-core', name: 'Frankfurt pilot', branch: 'cust/northwind-emea-de', sourceVersionId: emea11, reason: 'The Frankfurt pilot adds a Bundesbank reporting channel and is kept apart from EMEA until the pilot proves out.', status: 'Active' });
v('payments-core:northwind-emea-de', 'emea-1.1-de1', '2026-05-08', { kind: 'tag', value: 'de-emea-1.1-1' });

line({ id: 'payments-core:helvetia', appKey: 'payments-core', name: 'Helvetia Bank edition', branch: 'cust/helvetia', sourceVersionId: b210, reason: 'Helvetia Bank needs HSM-backed key management and an on-prem deployment to meet DORA third-party risk rules. Review declined merging it into mainline.', status: 'Active', note: 'Forked in September; first version not yet registered.' });

line({ id: 'payments-core:lts-2x', appKey: 'payments-core', name: '2.x long-term support', branch: 'release/2.x-lts', sourceVersionId: b230, reason: 'Customers on 2.x cannot move to 3.x yet. Security fixes continue on 2.x.', status: 'Active' });
v('payments-core:lts-2x', '2.3.1', '2025-11-03');
v('payments-core:lts-2x', '2.3.2', '2026-03-17');
v('payments-core:lts-2x', 'v2026.08', '2026-08-12', { kind: 'commit', value: '9f03be1' }, 'Date-style number; the format is not enforced');

// order-service: mainline is not called main
line({ id: 'order-service:main', appKey: 'order-service', isMain: true, name: 'Mainline', branch: 'develop', status: 'Active' });
v('order-service:main', 'v3.3', '2025-09-01');
const o34 = v('order-service:main', 'v3.4', '2026-02-10');
v('order-service:main', 'v4.0', '2026-07-19');
line({ id: 'order-service:fedramp', appKey: 'order-service', name: '3.x FedRAMP', branch: 'maint/3.x-fedramp', sourceVersionId: o34, reason: 'The FedRAMP Moderate authorization package covers only 3.4. Deployments inside the authorized boundary cannot take newer builds.', status: 'Active' });
v('order-service:fedramp', 'v3.4.1-fr', '2026-05-30');

// fraud-engine: mainline only
line({ id: 'fraud-engine:main', appKey: 'fraud-engine', isMain: true, name: 'Mainline', branch: 'main', status: 'Active' });
v('fraud-engine:main', '0.9.0', '2026-03-02');
v('fraud-engine:main', '1.0.0', '2026-09-14');

// settlement-batch: one retired line, sole Owner is a local-only user
line({ id: 'settlement-batch:main', appKey: 'settlement-batch', isMain: true, name: 'Mainline', branch: 'main', status: 'Active' });
const s52 = v('settlement-batch:main', '5.2', '2023-11-20');
v('settlement-batch:main', '6.0', '2025-06-01');
line({ id: 'settlement-batch:legacy', appKey: 'settlement-batch', name: 'Legacy core compatibility', branch: 'maint/legacy-core', sourceVersionId: s52, reason: 'The end-of-day batch interface had to stay unchanged until the legacy core was shut down.', status: 'Retired' });
v('settlement-batch:legacy', '5.2.7', '2025-03-14');

let state: State = {
  role: 'owner',
  me: 'elarsen',
  users: [
    { username: 'elarsen', displayName: 'Emma Larsen', email: 'emma.larsen@corp.example', dept: 'Payments Platform', status: 'Enabled', identities: [{ type: 'ldap', login: 'elarsen', boundAt: '2026-09-02', boundBy: 'First sign-in' }] },
    { username: 'dbrooks', displayName: 'Daniel Brooks', email: 'daniel.brooks@corp.example', phone: '+1 415 555 0142', dept: 'Payments Platform', status: 'Enabled', identities: [{ type: 'ldap', login: 'dbrooks', boundAt: '2026-09-02', boundBy: 'First sign-in' }, { type: 'local', login: 'dbrooks', boundAt: '2026-09-10', boundBy: 'Self' }] },
    { username: 'praman', displayName: 'Priya Raman', email: 'priya.raman@corp.example', dept: 'Infrastructure', status: 'Enabled', identities: [{ type: 'ldap', login: 'praman', boundAt: '2026-09-03', boundBy: 'First sign-in' }] },
    { username: 'mweber', displayName: 'Marcus Weber', email: 'marcus.weber@contractor.example', status: 'Enabled', identities: [{ type: 'local', login: 'mweber', boundAt: '2026-09-05', boundBy: 'Administrator admin' }] },
    { username: 'srossi', displayName: 'Sofia Rossi', email: 'sofia.rossi@corp.example', dept: 'Risk Engineering', status: 'Disabled', identities: [{ type: 'ldap', login: 'srossi', boundAt: '2026-09-04', boundBy: 'First sign-in' }] },
    { username: 'admin', displayName: 'Initial administrator', email: '', status: 'Enabled', identities: [{ type: 'local', login: 'admin', boundAt: '2026-09-01', boundBy: 'First startup' }] },
  ],
  platforms: [
    { id: 'gl-corp', type: 'GitLab', url: 'https://gitlab.corp.example', tokenTail: 'x9Qk', addedAt: '2026-09-01' },
    { id: 'gh-ent', type: 'GitHub', url: 'https://github.corp.example', tokenTail: '7fA2', addedAt: '2026-09-08' },
  ],
  apps: [
    { key: 'payments-core', name: 'Payments Core', description: 'Payment processing, statement generation and reconciliation.', repo: { platformId: 'gl-corp', path: 'payments/payments-core', projectId: '1182' }, mainBranch: 'main', owners: ['elarsen', 'dbrooks'], pes: ['praman'], deployInfo: 'Production: Kubernetes cluster prod-eu-2, namespace payments\nRelease window: Tue and Thu after 18:00 CET\nRollback: helm rollback, last 3 revisions kept' },
    { key: 'order-service', name: 'Order Service', description: 'Order intake, the order state machine and fulfilment callbacks.', repo: { platformId: 'gh-ent', path: 'platform/order-service', projectId: '552310987' }, mainBranch: 'develop', owners: ['dbrooks'], pes: ['elarsen'], deployInfo: 'FedRAMP environments are deployed by the customer from our signed images.' },
    { key: 'fraud-engine', name: 'Fraud Engine', description: 'Real-time transaction fraud rule evaluation.', repo: { platformId: 'gl-corp', path: 'risk/fraud-engine', projectId: '2047' }, mainBranch: 'main', owners: ['praman'], pes: [], deployInfo: '' },
    { key: 'settlement-batch', name: 'Settlement Batch', description: 'End-of-day clearing and settlement file generation.', repo: { platformId: 'gl-corp', path: 'payments/settlement-batch', projectId: '640' }, mainBranch: 'main', owners: ['mweber'], pes: ['elarsen'], deployInfo: 'The nightly batch starts at 01:30 UTC. A failed run needs a manual re-run.' },
  ],
  lines,
  versions,
  audit: [
    { appKey: 'payments-core', at: '2026-09-21 16:02', actor: 'elarsen', text: 'Forked maintenance line “Helvetia Bank edition” from 2.1.0' },
    { appKey: 'payments-core', at: '2026-08-30 22:41', actor: 'dbrooks', text: 'Registered version 3.2.0 on Mainline' },
    { appKey: 'payments-core', at: '2026-08-12 10:15', actor: 'dbrooks', text: 'Registered version v2026.08 on “2.x long-term support”' },
    { appKey: 'payments-core', at: '2026-06-03 09:30', actor: 'elarsen', text: 'Set “Northwind EMEA” to Frozen' },
    { appKey: 'payments-core', at: '2026-05-11 14:20', actor: 'elarsen', text: 'Added PE Priya Raman' },
    { appKey: 'order-service', at: '2026-07-19 11:08', actor: 'dbrooks', text: 'Registered version v4.0 on Mainline' },
    { appKey: 'order-service', at: '2026-05-30 15:44', actor: 'dbrooks', text: 'Registered version v3.4.1-fr on “3.x FedRAMP”' },
    { appKey: 'fraud-engine', at: '2026-09-14 09:12', actor: 'praman', text: 'Registered version 1.0.0 on Mainline' },
  ],
};

// ---------- store ----------

const listeners = new Set<() => void>();
export function useStore() {
  return useSyncExternalStore(
    (cb) => (listeners.add(cb), () => listeners.delete(cb)),
    () => state,
  );
}
function set(next: Partial<State>) {
  state = { ...state, ...next };
  listeners.forEach((l) => l());
}
function log(appKey: string, text: string) {
  set({ audit: [{ appKey, at: `${TODAY} ${new Date().toTimeString().slice(0, 5)}`, actor: state.me, text }, ...state.audit] });
}

// ---------- validation (shared by every direction; returns field → message) ----------

export type AppInput = {
  key: string;
  name: string;
  description: string;
  platformId: string;
  repoPath: string;
  projectId: string; // filled by verifyRepo
  mainBranch: string;
  owners: string[];
  pes: string[];
};
export type Errors<T> = Partial<Record<keyof T, string>>;

export function validateApp(input: AppInput, mode: 'create' | 'edit'): Errors<AppInput> {
  const e: Errors<AppInput> = {};
  if (mode === 'create') {
    if (!input.key) e.key = 'Enter an application key';
    else if (!/^[a-z][a-z0-9-]{1,39}$/.test(input.key)) e.key = 'Start with a lowercase letter; use only lowercase letters, digits and hyphens, 2–40 characters';
    else if (state.apps.some((a) => a.key === input.key)) e.key = 'This application key is already taken';
    if (!input.platformId) e.platformId = 'Select a code platform';
    if (!input.repoPath.trim()) e.repoPath = 'Enter a repository path';
    else if (!input.projectId) e.repoPath = 'Verify the repository first';
  }
  if (!input.name.trim()) e.name = 'Enter a display name';
  if (!input.mainBranch.trim()) e.mainBranch = 'Enter the mainline branch name';
  if (input.owners.length === 0) e.owners = 'At least one Owner is required';
  const inactive = [...input.owners, ...input.pes].filter((u) => state.users.find((x) => x.username === u)?.status !== 'Enabled');
  if (inactive.length) e.owners = `Disabled users cannot be Owner or PE: ${inactive.map(userName).join(', ')}`;
  return e;
}

/** Fake platform API check. Paths containing "missing" fail, everything else returns a stable project ID. */
export function verifyRepo(platformId: string, path: string): { ok: true; projectId: string } | { ok: false; message: string } {
  const p = state.platforms.find((x) => x.id === platformId);
  if (!p) return { ok: false, message: 'Select a code platform' };
  if (!/^[\w.-]+(\/[\w.-]+)+$/.test(path.trim())) return { ok: false, message: 'Use the form group/repo' };
  if (path.includes('missing')) return { ok: false, message: `${p.type} returned 404: the repository does not exist or the service token has no access` };
  let h = 7;
  for (const c of path) h = (h * 31 + c.charCodeAt(0)) % 900000;
  return { ok: true, projectId: String(100000 + h) };
}

export type LocalUserInput = { username: string; displayName: string; email: string; dept: string; password: string };
export function validateLocalUser(input: LocalUserInput): Errors<LocalUserInput> {
  const e: Errors<LocalUserInput> = {};
  if (!/^[a-z][a-z0-9._-]{1,31}$/.test(input.username)) e.username = 'Start with a lowercase letter, 2–32 characters';
  else if (state.users.some((u) => u.username === input.username)) e.username = 'This username already exists';
  if (!input.displayName.trim()) e.displayName = 'Enter a display name';
  if (input.email && !/^\S+@\S+$/.test(input.email)) e.email = 'Enter a valid email address';
  if (input.password.length < 8) e.password = 'At least 8 characters';
  return e;
}

export const actions = {
  setRole: (role: Role) => set({ role }),

  createApp(input: AppInput) {
    const app: Application = { key: input.key, name: input.name.trim(), description: input.description.trim(), repo: { platformId: input.platformId, path: input.repoPath.trim(), projectId: input.projectId }, mainBranch: input.mainBranch.trim(), owners: input.owners, pes: input.pes, deployInfo: '' };
    set({ apps: [...state.apps, app], lines: [...state.lines, { id: `${app.key}:main`, appKey: app.key, isMain: true, name: 'Mainline', branch: app.mainBranch, status: 'Active' }] });
    log(app.key, `Created the application. Owners: ${input.owners.map(userName).join(', ')}`);
  },

  /** Application Key and Repository Binding are not editable here. Owner/PE changes are written to Change history one by one. */
  updateApp(key: string, patch: Pick<AppInput, 'name' | 'description' | 'mainBranch' | 'owners' | 'pes'>) {
    const before = state.apps.find((a) => a.key === key)!;
    set({ apps: state.apps.map((a) => (a.key === key ? { ...a, name: patch.name.trim(), description: patch.description.trim(), mainBranch: patch.mainBranch.trim(), owners: patch.owners, pes: patch.pes } : a)) });
    const diff = (role: string, was: string[], now: string[]) => {
      now.filter((u) => !was.includes(u)).forEach((u) => log(key, `Added ${role} ${userName(u)}`));
      was.filter((u) => !now.includes(u)).forEach((u) => log(key, `Removed ${role} ${userName(u)}`));
    };
    diff('Owner', before.owners, patch.owners);
    diff('PE', before.pes, patch.pes);
    if (before.name !== patch.name.trim() || before.description !== patch.description.trim() || before.mainBranch !== patch.mainBranch.trim()) log(key, 'Edited application details');
  },

  deleteApp(key: string) {
    const lineIds = new Set(state.lines.filter((l) => l.appKey === key).map((l) => l.id));
    set({ apps: state.apps.filter((a) => a.key !== key), lines: state.lines.filter((l) => l.appKey !== key), versions: state.versions.filter((v) => !lineIds.has(v.lineId)) });
  },

  createLocalUser(input: LocalUserInput) {
    set({ users: [...state.users, { username: input.username, displayName: input.displayName.trim(), email: input.email.trim(), dept: input.dept.trim() || undefined, status: 'Enabled', identities: [{ type: 'local', login: input.username, boundAt: TODAY, boundBy: `Administrator ${state.me}` }] }] });
  },

  fork(input: Omit<Line, 'id' | 'isMain' | 'status'>) {
    const id = `${input.appKey}:${input.branch}`;
    set({ lines: [...state.lines, { ...input, id, isMain: false, status: 'Active' }] });
    log(input.appKey, `Forked maintenance line “${input.name}” from ${versionById(input.sourceVersionId!)?.number}`);
    return id;
  },

  registerVersion(input: Omit<Version, 'id'>) {
    const id = `ver${++vid}`;
    set({ versions: [...state.versions, { ...input, id }] });
    const l = lineById(input.lineId)!;
    log(l.appKey, `Registered version ${input.number} on ${l.isMain ? 'Mainline' : `“${l.name}”`}`);
  },

  setLineStatus(lineId: string, status: LineStatus) {
    const l = lineById(lineId)!;
    set({ lines: state.lines.map((x) => (x.id === lineId ? { ...x, status } : x)) });
    log(l.appKey, `Set “${l.name}” to ${status}`);
  },

  setDeployInfo(appKey: string, deployInfo: string) {
    set({ apps: state.apps.map((a) => (a.key === appKey ? { ...a, deployInfo } : a)) });
    log(appKey, 'Edited deployment info');
  },

  setUserStatus(username: string, status: User['status']) {
    set({ users: state.users.map((u) => (u.username === username ? { ...u, status } : u)) });
  },

  bindIdentity(username: string, identity: LoginIdentity) {
    set({ users: state.users.map((u) => (u.username === username ? { ...u, identities: [...u.identities, identity] } : u)) });
  },

  unbindIdentity(username: string, type: IdentityType) {
    set({ users: state.users.map((u) => (u.username === username ? { ...u, identities: u.identities.filter((i) => i.type !== type) } : u)) });
  },

  addPlatform(p: CodePlatform) {
    set({ platforms: [...state.platforms, p] });
  },
};

// ---------- queries ----------

export const lineById = (id: string) => state.lines.find((l) => l.id === id);
export const versionById = (id: string) => state.versions.find((x) => x.id === id);
export const userName = (u: string) => state.users.find((x) => x.username === u)?.displayName ?? u;

export function appLines(s: State, appKey: string) {
  return s.lines.filter((l) => l.appKey === appKey);
}
export function lineVersions(s: State, lineId: string) {
  return s.versions.filter((x) => x.lineId === lineId).sort((a, b) => a.releasedAt.localeCompare(b.releasedAt));
}
export function appVersions(s: State, appKey: string) {
  const ids = new Set(appLines(s, appKey).map((l) => l.id));
  return s.versions.filter((x) => ids.has(x.lineId)).sort((a, b) => a.releasedAt.localeCompare(b.releasedAt));
}
export function childLines(s: State, versionId: string) {
  return s.lines.filter((l) => l.sourceVersionId === versionId);
}

/** Lines in lineage order: mainline first, then depth-first by source version date. */
export function lineageOrder(s: State, appKey: string): { line: Line; depth: number }[] {
  const out: { line: Line; depth: number }[] = [];
  const walk = (l: Line, depth: number) => {
    out.push({ line: l, depth });
    for (const ver of lineVersions(s, l.id)) for (const c of childLines(s, ver.id)) walk(c, depth + 1);
  };
  const main = appLines(s, appKey).find((l) => l.isMain);
  if (main) walk(main, 0);
  return out;
}

export function can(role: Role) {
  return {
    manageLines: role === 'owner',
    editApp: role === 'owner',
    editDeploy: role === 'owner' || role === 'pe',
    admin: role === 'admin',
  };
}

export function soleOwnerOf(s: State, username: string) {
  return s.apps.filter((a) => a.owners.length === 1 && a.owners[0] === username);
}
