// PROTOTYPE fake data + in-memory store. Reload = reset. Nothing persists.
// Seeded content is bilingual (Text); system messages (validation, change history, boundBy) are Msg keys rendered by i18n.
import { useSyncExternalStore } from 'react';
import { resolveText, type Msg, type Text } from './i18n';

/** Seed helper: bilingual content. */
const T = (en: string, zh: string): Text => ({ en, zh });

export const TODAY = '2026-10-10';

export type Role = 'admin' | 'owner' | 'pe';
export type LineStatus = 'Active' | 'Frozen' | 'Retired';
export type IdentityType = 'ldap' | 'local';

export type LoginIdentity = { type: IdentityType; login: string; boundAt: string; boundBy: Msg };
export type User = {
  username: string;
  displayName: Text;
  email: string;
  phone?: string;
  dept?: Text;
  status: 'Enabled' | 'Disabled';
  identities: LoginIdentity[];
};

export type CodePlatform = { id: string; type: 'GitLab' | 'GitHub'; url: string; tokenTail: string; addedAt: string };

export type Application = {
  key: string;
  name: Text;
  description: Text;
  repo: { platformId: string; path: string; projectId: string };
  mainBranch: string;
  owners: string[];
  pes: string[];
  deployInfo: Text;
};

export type Line = {
  id: string;
  appKey: string;
  isMain: boolean;
  name: Text;
  branch: string;
  sourceVersionId?: string;
  reason?: Text;
  status: LineStatus;
  /** Optional and deliberately low-key: shown as a plain key-value row in line detail, never as a countdown, sort key, colour or reminder. */
  plannedEnd?: string;
  note?: Text;
};

export type Version = {
  id: string;
  lineId: string;
  number: string;
  ref: { kind: 'tag' | 'commit'; value: string };
  releasedAt: string;
  note: Text;
};

export type AuditEntry = { appKey: string; at: string; actor: string; event: Msg };

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
function v(lineId: string, number: string, releasedAt: string, ref: Version['ref'] = { kind: 'tag', value: number.startsWith('v') ? number : `v${number}` }, note: Text = '') {
  const id = `ver${++vid}`;
  versions.push({ id, lineId, number, ref, releasedAt, note });
  return id;
}

const lines: Line[] = [];
function line(l: Omit<Line, 'isMain'> & { isMain?: boolean }) {
  lines.push({ isMain: false, ...l });
  return l.id;
}
const MAINLINE = T('Mainline', '主线');

// payments-core: the lineage stress case (4 levels deep, every status, a line with no versions, a date-style version number)
line({ id: 'payments-core:main', appKey: 'payments-core', isMain: true, name: MAINLINE, branch: 'main', status: 'Active' });
v('payments-core:main', '1.0.0', '2024-03-12', undefined, T('First general release', '首个正式版本'));
const b110 = v('payments-core:main', '1.1.0', '2024-06-20');
v('payments-core:main', '2.0.0', '2024-11-05', undefined, T('New ledger model', '新的账务模型'));
const b210 = v('payments-core:main', '2.1.0', '2025-02-18');
v('payments-core:main', '2.2.0', '2025-05-27');
const b230 = v('payments-core:main', '2.3.0', '2025-08-14');
v('payments-core:main', '3.0.0', '2026-01-09', undefined, T('Switch to the new settlement engine', '切换到新的结算引擎'));
v('payments-core:main', '3.1.0', '2026-04-22');
v('payments-core:main', '3.2.0', '2026-08-30');

line({ id: 'payments-core:pci-1x', appKey: 'payments-core', name: T('1.x PCI freeze', '1.x PCI 冻结线'), branch: 'maint/1.x-pci', sourceVersionId: b110, reason: T('Held at 1.1 during the 2024 PCI DSS assessment; security fixes only until sign-off.', '2024 年 PCI DSS 评估期间锁定在 1.1，评估通过前只做安全修复。'), status: 'Retired', note: T('Assessment passed in May 2025; all customers have moved to 2.x.', '评估已于 2025 年 5 月通过，所有客户都已升级到 2.x。') });
v('payments-core:pci-1x', '1.1.1', '2024-08-02', { kind: 'commit', value: '4e1a9c2' }, T('Fix broken access control on statement queries', '修复对账单查询的越权访问'));
v('payments-core:pci-1x', '1.1.2', '2024-12-11');

line({ id: 'payments-core:northwind', appKey: 'payments-core', name: T('Northwind customization', 'Northwind 定制'), branch: 'cust/northwind', sourceVersionId: b210, reason: T('The Northwind contract requires a custom statement format and settlement cycle. Mainline will not take these changes.', 'Northwind 合同要求定制对账单格式和结算周期，主线不接受这些改动。'), status: 'Active', plannedEnd: '2027-06-30', note: T('Contract renewal is being negotiated.', '合同续约正在谈判中。') });
v('payments-core:northwind', '2.1.0-nw.1', '2025-04-03');
const nw2 = v('payments-core:northwind', '2.1.0-nw.2', '2025-07-15');
v('payments-core:northwind', '2.1.0-nw.3', '2026-02-26');

line({ id: 'payments-core:northwind-emea', appKey: 'payments-core', name: T('Northwind EMEA', 'Northwind 欧洲分部'), branch: 'cust/northwind-emea', sourceVersionId: nw2, reason: T('The EMEA subsidiary needs SEPA-formatted reports, which conflict with the Northwind HQ customization.', '欧洲分部需要 SEPA 格式的报表，与 Northwind 总部的定制冲突。'), status: 'Frozen' });
v('payments-core:northwind-emea', 'emea-1.0', '2025-10-20', { kind: 'tag', value: 'northwind-emea-1.0' });
const emea11 = v('payments-core:northwind-emea', 'emea-1.1', '2026-01-30', { kind: 'commit', value: 'b71d0e8' });

line({ id: 'payments-core:northwind-emea-de', appKey: 'payments-core', name: T('Frankfurt pilot', '法兰克福试点'), branch: 'cust/northwind-emea-de', sourceVersionId: emea11, reason: T('The Frankfurt pilot adds a Bundesbank reporting channel and is kept apart from EMEA until the pilot proves out.', '法兰克福试点增加了德国联邦银行报送渠道，试点验证通过前与欧洲分部线分开维护。'), status: 'Active' });
v('payments-core:northwind-emea-de', 'emea-1.1-de1', '2026-05-08', { kind: 'tag', value: 'de-emea-1.1-1' });

line({ id: 'payments-core:helvetia', appKey: 'payments-core', name: T('Helvetia Bank edition', 'Helvetia Bank 专版'), branch: 'cust/helvetia', sourceVersionId: b210, reason: T('Helvetia Bank needs HSM-backed key management and an on-prem deployment to meet DORA third-party risk rules. Review declined merging it into mainline.', 'Helvetia Bank 需要 HSM 托管密钥和本地部署以满足 DORA 第三方风险要求，评审未同意并入主线。'), status: 'Active', note: T('Forked in September; first version not yet registered.', '9 月分叉，尚未登记第一个版本。') });

line({ id: 'payments-core:lts-2x', appKey: 'payments-core', name: T('2.x long-term support', '2.x 长期支持'), branch: 'release/2.x-lts', sourceVersionId: b230, reason: T('Customers on 2.x cannot move to 3.x yet. Security fixes continue on 2.x.', '2.x 客户暂时无法升级到 3.x，2.x 上继续提供安全修复。'), status: 'Active', plannedEnd: '2027-03-31' });
v('payments-core:lts-2x', '2.3.1', '2025-11-03');
v('payments-core:lts-2x', '2.3.2', '2026-03-17');
v('payments-core:lts-2x', 'v2026.08', '2026-08-12', { kind: 'commit', value: '9f03be1' }, T('Date-style number; the format is not enforced', '日期式版本号；格式不做强制'));

// order-service: mainline is not called main
line({ id: 'order-service:main', appKey: 'order-service', isMain: true, name: MAINLINE, branch: 'develop', status: 'Active' });
v('order-service:main', 'v3.3', '2025-09-01');
const o34 = v('order-service:main', 'v3.4', '2026-02-10');
v('order-service:main', 'v4.0', '2026-07-19');
line({ id: 'order-service:fedramp', appKey: 'order-service', name: T('3.x FedRAMP', '3.x FedRAMP'), branch: 'maint/3.x-fedramp', sourceVersionId: o34, reason: T('The FedRAMP Moderate authorization package covers only 3.4. Deployments inside the authorized boundary cannot take newer builds.', 'FedRAMP Moderate 授权包只覆盖 3.4，授权边界内的部署不能使用更新的构建。'), status: 'Active' });
v('order-service:fedramp', 'v3.4.1-fr', '2026-05-30');

// fraud-engine: mainline only
line({ id: 'fraud-engine:main', appKey: 'fraud-engine', isMain: true, name: MAINLINE, branch: 'main', status: 'Active' });
v('fraud-engine:main', '0.9.0', '2026-03-02');
v('fraud-engine:main', '1.0.0', '2026-09-14');

// settlement-batch: one retired line, sole Owner is a local-only user
line({ id: 'settlement-batch:main', appKey: 'settlement-batch', isMain: true, name: MAINLINE, branch: 'main', status: 'Active' });
const s52 = v('settlement-batch:main', '5.2', '2023-11-20');
v('settlement-batch:main', '6.0', '2025-06-01');
line({ id: 'settlement-batch:legacy', appKey: 'settlement-batch', name: T('Legacy core compatibility', '旧核心兼容'), branch: 'maint/legacy-core', sourceVersionId: s52, reason: T('The end-of-day batch interface had to stay unchanged until the legacy core was shut down.', '旧核心下线前，日终批量接口必须保持不变。'), status: 'Retired' });
v('settlement-batch:legacy', '5.2.7', '2025-03-14');

const M = (key: string, params?: Msg['params']): Msg => ({ key, params });
const FIRST = M('boundBy.firstSignIn');

let state: State = {
  role: 'owner',
  me: 'elarsen',
  users: [
    { username: 'elarsen', displayName: 'Emma Larsen', email: 'emma.larsen@corp.example', dept: T('Payments Platform', '支付平台部'), status: 'Enabled', identities: [{ type: 'ldap', login: 'elarsen', boundAt: '2026-09-02', boundBy: FIRST }] },
    { username: 'dbrooks', displayName: 'Daniel Brooks', email: 'daniel.brooks@corp.example', phone: '+1 415 555 0142', dept: T('Payments Platform', '支付平台部'), status: 'Enabled', identities: [{ type: 'ldap', login: 'dbrooks', boundAt: '2026-09-02', boundBy: FIRST }, { type: 'local', login: 'dbrooks', boundAt: '2026-09-10', boundBy: M('boundBy.self') }] },
    { username: 'praman', displayName: 'Priya Raman', email: 'priya.raman@corp.example', dept: T('Infrastructure', '基础设施部'), status: 'Enabled', identities: [{ type: 'ldap', login: 'praman', boundAt: '2026-09-03', boundBy: FIRST }] },
    { username: 'mweber', displayName: 'Marcus Weber', email: 'marcus.weber@contractor.example', status: 'Enabled', identities: [{ type: 'local', login: 'mweber', boundAt: '2026-09-05', boundBy: M('boundBy.admin', { name: 'admin' }) }] },
    { username: 'srossi', displayName: 'Sofia Rossi', email: 'sofia.rossi@corp.example', dept: T('Risk Engineering', '风险工程部'), status: 'Disabled', identities: [{ type: 'ldap', login: 'srossi', boundAt: '2026-09-04', boundBy: FIRST }] },
    { username: 'admin', displayName: T('Initial administrator', '初始管理员'), email: '', status: 'Enabled', identities: [{ type: 'local', login: 'admin', boundAt: '2026-09-01', boundBy: M('boundBy.firstStartup') }] },
  ],
  platforms: [
    { id: 'gl-corp', type: 'GitLab', url: 'https://gitlab.corp.example', tokenTail: 'x9Qk', addedAt: '2026-09-01' },
    { id: 'gh-ent', type: 'GitHub', url: 'https://github.corp.example', tokenTail: '7fA2', addedAt: '2026-09-08' },
  ],
  apps: [
    { key: 'payments-core', name: T('Payments Core', '支付核心'), description: T('Payment processing, statement generation and reconciliation.', '支付处理、对账单生成和对账。'), repo: { platformId: 'gl-corp', path: 'payments/payments-core', projectId: '1182' }, mainBranch: 'main', owners: ['elarsen', 'dbrooks'], pes: ['praman'], deployInfo: T('Production: Kubernetes cluster prod-eu-2, namespace payments\nRelease window: Tue and Thu after 18:00 CET\nRollback: helm rollback, last 3 revisions kept', '生产环境：Kubernetes 集群 prod-eu-2，命名空间 payments\n发布窗口：周二、周四 18:00（CET）之后\n回滚：helm rollback，保留最近 3 个修订') },
    { key: 'order-service', name: T('Order Service', '订单服务'), description: T('Order intake, the order state machine and fulfilment callbacks.', '下单、订单状态机和履约回调。'), repo: { platformId: 'gh-ent', path: 'platform/order-service', projectId: '552310987' }, mainBranch: 'develop', owners: ['dbrooks'], pes: ['elarsen'], deployInfo: T('FedRAMP environments are deployed by the customer from our signed images.', 'FedRAMP 环境由客户使用我们签名的镜像自行部署。') },
    { key: 'fraud-engine', name: T('Fraud Engine', '反欺诈引擎'), description: T('Real-time transaction fraud rule evaluation.', '实时交易欺诈规则评估。'), repo: { platformId: 'gl-corp', path: 'risk/fraud-engine', projectId: '2047' }, mainBranch: 'main', owners: ['praman'], pes: [], deployInfo: '' },
    { key: 'settlement-batch', name: T('Settlement Batch', '结算批处理'), description: T('End-of-day clearing and settlement file generation.', '日终清算和结算文件生成。'), repo: { platformId: 'gl-corp', path: 'payments/settlement-batch', projectId: '640' }, mainBranch: 'main', owners: ['mweber'], pes: ['elarsen'], deployInfo: T('The nightly batch starts at 01:30 UTC. A failed run needs a manual re-run.', '夜间批处理 01:30（UTC）启动，失败后需要手动重跑。') },
  ],
  lines,
  versions,
  audit: [
    { appKey: 'payments-core', at: '2026-09-21 16:02', actor: 'elarsen', event: M('audit.fork', { line: T('Helvetia Bank edition', 'Helvetia Bank 专版'), version: '2.1.0' }) },
    { appKey: 'payments-core', at: '2026-08-30 22:41', actor: 'dbrooks', event: M('audit.registerVersionMain', { version: '3.2.0' }) },
    { appKey: 'payments-core', at: '2026-08-12 10:15', actor: 'dbrooks', event: M('audit.registerVersion', { version: 'v2026.08', line: T('2.x long-term support', '2.x 长期支持') }) },
    { appKey: 'payments-core', at: '2026-06-03 09:30', actor: 'elarsen', event: M('audit.setStatus', { line: T('Northwind EMEA', 'Northwind 欧洲分部'), status: M('status.Frozen') }) },
    { appKey: 'payments-core', at: '2026-05-11 14:20', actor: 'elarsen', event: M('audit.addRole', { role: M('role.pe'), name: 'Priya Raman' }) },
    { appKey: 'order-service', at: '2026-07-19 11:08', actor: 'dbrooks', event: M('audit.registerVersionMain', { version: 'v4.0' }) },
    { appKey: 'order-service', at: '2026-05-30 15:44', actor: 'dbrooks', event: M('audit.registerVersion', { version: 'v3.4.1-fr', line: '3.x FedRAMP' }) },
    { appKey: 'fraud-engine', at: '2026-09-14 09:12', actor: 'praman', event: M('audit.registerVersionMain', { version: '1.0.0' }) },
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
function log(appKey: string, event: Msg) {
  set({ audit: [{ appKey, at: `${TODAY} ${new Date().toTimeString().slice(0, 5)}`, actor: state.me, event }, ...state.audit] });
}

// ---------- validation (shared by every direction; returns field → Msg, render with tm()) ----------

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
export type Errors<T> = Partial<Record<keyof T, Msg>>;

export function validateApp(input: AppInput, mode: 'create' | 'edit'): Errors<AppInput> {
  const e: Errors<AppInput> = {};
  if (mode === 'create') {
    if (!input.key) e.key = M('err.keyRequired');
    else if (!/^[a-z][a-z0-9-]{1,39}$/.test(input.key)) e.key = M('err.keyFormat');
    else if (state.apps.some((a) => a.key === input.key)) e.key = M('err.keyTaken');
    if (!input.platformId) e.platformId = M('err.platformRequired');
    if (!input.repoPath.trim()) e.repoPath = M('err.repoPathRequired');
    else if (!input.projectId) e.repoPath = M('err.repoVerifyFirst');
  }
  if (!input.name.trim()) e.name = M('err.nameRequired');
  if (!input.mainBranch.trim()) e.mainBranch = M('err.mainBranchRequired');
  if (input.owners.length === 0) e.owners = M('err.ownerRequired');
  const inactive = [...input.owners, ...input.pes].filter((u) => state.users.find((x) => x.username === u)?.status !== 'Enabled');
  if (inactive.length) e.owners = M('err.disabledUsers', { names: inactive.map(userName).join(', ') });
  return e;
}

/** Fake platform API check. Paths containing "missing" fail, everything else returns a stable project ID. */
export function verifyRepo(platformId: string, path: string): { ok: true; projectId: string } | { ok: false; message: Msg } {
  const p = state.platforms.find((x) => x.id === platformId);
  if (!p) return { ok: false, message: M('err.platformRequired') };
  if (!/^[\w.-]+(\/[\w.-]+)+$/.test(path.trim())) return { ok: false, message: M('err.repoPathFormat') };
  if (path.includes('missing')) return { ok: false, message: M('err.repo404', { platform: p.type }) };
  let h = 7;
  for (const c of path) h = (h * 31 + c.charCodeAt(0)) % 900000;
  return { ok: true, projectId: String(100000 + h) };
}

export type LocalUserInput = { username: string; displayName: string; email: string; dept: string; password: string };
export function validateLocalUser(input: LocalUserInput): Errors<LocalUserInput> {
  const e: Errors<LocalUserInput> = {};
  if (!/^[a-z][a-z0-9._-]{1,31}$/.test(input.username)) e.username = M('err.usernameFormat');
  else if (state.users.some((u) => u.username === input.username)) e.username = M('err.usernameTaken');
  if (!input.displayName.trim()) e.displayName = M('err.nameRequired');
  if (input.email && !/^\S+@\S+$/.test(input.email)) e.email = M('err.emailInvalid');
  if (input.password.length < 8) e.password = M('err.passwordShort');
  return e;
}

export const actions = {
  setRole: (role: Role) => set({ role }),

  createApp(input: AppInput) {
    const app: Application = { key: input.key, name: input.name.trim(), description: input.description.trim(), repo: { platformId: input.platformId, path: input.repoPath.trim(), projectId: input.projectId }, mainBranch: input.mainBranch.trim(), owners: input.owners, pes: input.pes, deployInfo: '' };
    set({ apps: [...state.apps, app], lines: [...state.lines, { id: `${app.key}:main`, appKey: app.key, isMain: true, name: MAINLINE, branch: app.mainBranch, status: 'Active' }] });
    log(app.key, M('audit.createApp', { owners: input.owners.map(userName).join(', ') }));
  },

  /** Application Key and Repository Binding are not editable here. Owner/PE changes are written to Change history one by one. */
  updateApp(key: string, patch: Pick<AppInput, 'name' | 'description' | 'mainBranch' | 'owners' | 'pes'>) {
    const before = state.apps.find((a) => a.key === key)!;
    set({ apps: state.apps.map((a) => (a.key === key ? { ...a, name: patch.name.trim(), description: patch.description.trim(), mainBranch: patch.mainBranch.trim(), owners: patch.owners, pes: patch.pes } : a)) });
    const diff = (role: 'owner' | 'pe', was: string[], now: string[]) => {
      now.filter((u) => !was.includes(u)).forEach((u) => log(key, M('audit.addRole', { role: M(`role.${role}`), name: userName(u) })));
      was.filter((u) => !now.includes(u)).forEach((u) => log(key, M('audit.removeRole', { role: M(`role.${role}`), name: userName(u) })));
    };
    diff('owner', before.owners, patch.owners);
    diff('pe', before.pes, patch.pes);
    if (resolveText(before.name) !== patch.name.trim() || resolveText(before.description) !== patch.description.trim() || before.mainBranch !== patch.mainBranch.trim()) log(key, M('audit.editApp'));
  },

  deleteApp(key: string) {
    const lineIds = new Set(state.lines.filter((l) => l.appKey === key).map((l) => l.id));
    set({ apps: state.apps.filter((a) => a.key !== key), lines: state.lines.filter((l) => l.appKey !== key), versions: state.versions.filter((v) => !lineIds.has(v.lineId)) });
  },

  createLocalUser(input: LocalUserInput) {
    set({ users: [...state.users, { username: input.username, displayName: input.displayName.trim(), email: input.email.trim(), dept: input.dept.trim() || undefined, status: 'Enabled', identities: [{ type: 'local', login: input.username, boundAt: TODAY, boundBy: M('boundBy.admin', { name: state.me }) }] }] });
  },

  fork(input: Omit<Line, 'id' | 'isMain' | 'status'>) {
    const id = `${input.appKey}:${input.branch}`;
    set({ lines: [...state.lines, { ...input, id, isMain: false, status: 'Active' }] });
    log(input.appKey, M('audit.fork', { line: input.name, version: versionById(input.sourceVersionId!)!.number }));
    return id;
  },

  registerVersion(input: Omit<Version, 'id'>) {
    const id = `ver${++vid}`;
    set({ versions: [...state.versions, { ...input, id }] });
    const l = lineById(input.lineId)!;
    log(l.appKey, l.isMain ? M('audit.registerVersionMain', { version: input.number }) : M('audit.registerVersion', { version: input.number, line: l.name }));
  },

  setLineStatus(lineId: string, status: LineStatus) {
    const l = lineById(lineId)!;
    set({ lines: state.lines.map((x) => (x.id === lineId ? { ...x, status } : x)) });
    log(l.appKey, M('audit.setStatus', { line: l.name, status: M(`status.${status}`) }));
  },

  setDeployInfo(appKey: string, deployInfo: string) {
    set({ apps: state.apps.map((a) => (a.key === appKey ? { ...a, deployInfo } : a)) });
    log(appKey, M('audit.editDeploy'));
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

/** Build a Msg for boundBy when a direction binds an identity (self-bound from Settings). */
export const boundBySelf = (): Msg => M('boundBy.self');

// ---------- queries ----------

export const lineById = (id: string) => state.lines.find((l) => l.id === id);
export const versionById = (id: string) => state.versions.find((x) => x.id === id);
export const userName = (u: string) => {
  const d = state.users.find((x) => x.username === u)?.displayName;
  return d === undefined ? u : resolveText(d);
};

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
/** Newest registered version on a line, if any. */
export function latestVersion(s: State, lineId: string) {
  const vs = lineVersions(s, lineId);
  return vs[vs.length - 1];
}
/** Every version across every application, newest first, with its line and application attached. */
export function allVersions(s: State) {
  return s.versions
    .map((ver) => {
      const l = s.lines.find((x) => x.id === ver.lineId)!;
      return { version: ver, line: l, app: s.apps.find((a) => a.key === l.appKey)! };
    })
    .sort((a, b) => b.version.releasedAt.localeCompare(a.version.releasedAt));
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
