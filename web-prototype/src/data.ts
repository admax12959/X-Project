// PROTOTYPE fake data + in-memory store. Reload = reset. Nothing persists.
import { useSyncExternalStore } from 'react';

export const TODAY = '2026-10-10';
export const EXPIRY_WINDOW_DAYS = 30;

export type Role = 'admin' | 'owner' | 'pe';
export type LineStatus = '活跃' | '冻结' | '终止';
export type IdentityType = 'ldap' | 'local';

export type LoginIdentity = { type: IdentityType; login: string; boundAt: string; boundBy: string };
export type User = {
  username: string;
  displayName: string;
  email: string;
  phone?: string;
  dept?: string;
  status: '正常' | '已禁用';
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
  plannedEnd?: string;
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

// billing-core: the lineage stress case
line({ id: 'billing-core:main', appKey: 'billing-core', isMain: true, name: '主线', branch: 'main', status: '活跃' });
v('billing-core:main', '1.0.0', '2024-03-12', undefined, '首个正式版本');
const b110 = v('billing-core:main', '1.1.0', '2024-06-20');
v('billing-core:main', '2.0.0', '2024-11-05', undefined, '账单模型重构');
const b210 = v('billing-core:main', '2.1.0', '2025-02-18');
v('billing-core:main', '2.2.0', '2025-05-27');
const b230 = v('billing-core:main', '2.3.0', '2025-08-14');
v('billing-core:main', '3.0.0', '2026-01-09', undefined, '切换到新结算引擎');
v('billing-core:main', '3.1.0', '2026-04-22');
v('billing-core:main', '3.2.0', '2026-08-30');

line({ id: 'billing-core:dengbao-1x', appKey: 'billing-core', name: '1.x 等保冻结', branch: 'maint/1.x-dengbao', sourceVersionId: b110, reason: '2024 年等保三级测评期间锁定 1.1，测评通过前只做安全修复', status: '终止', plannedEnd: '2025-06-30', note: '测评已于 2025-05 通过，客户全部升级到 2.x' });
v('billing-core:dengbao-1x', '1.1.1', '2024-08-02', { kind: 'commit', value: '4e1a9c2' }, '修复越权查询账单');
v('billing-core:dengbao-1x', '1.1.2', '2024-12-11');

line({ id: 'billing-core:acme', appKey: 'billing-core', name: 'ACME 定制', branch: 'cust/acme', sourceVersionId: b210, reason: 'ACME 合同要求定制对账单格式和结算周期，主线不接受这部分改动', status: '活跃', plannedEnd: '2026-10-24', note: '合同 2026-10 到期，续约谈判中' });
v('billing-core:acme', '2.1.0-acme.1', '2025-04-03');
const acme2 = v('billing-core:acme', '2.1.0-acme.2', '2025-07-15');
v('billing-core:acme', '2.1.0-acme.3', '2026-02-26');

line({ id: 'billing-core:acme-east', appKey: 'billing-core', name: 'ACME 华东分部', branch: 'cust/acme-east', sourceVersionId: acme2, reason: '华东分部要求按省级财政口径出报表，与 ACME 总部定制冲突', status: '冻结', plannedEnd: '2027-03-31' });
v('billing-core:acme-east', 'R5SP1', '2025-10-20', { kind: 'tag', value: 'acme-east-R5SP1' });
const r5sp2 = v('billing-core:acme-east', 'R5SP2', '2026-01-30', { kind: 'commit', value: 'b71d0e8' });

line({ id: 'billing-core:acme-east-hz', appKey: 'billing-core', name: '杭州试点', branch: 'cust/acme-east-hz', sourceVersionId: r5sp2, reason: '杭州试点接入市民卡支付渠道，只在试点期内维护', status: '活跃', plannedEnd: '2026-12-31' });
v('billing-core:acme-east-hz', 'R5SP2-hz1', '2026-05-08', { kind: 'tag', value: 'hz-R5SP2-1' });

line({ id: 'billing-core:bank', appKey: 'billing-core', name: '城商行专版', branch: 'cust/bank', sourceVersionId: b210, reason: '城商行客户要求国密算法和私有化部署，评审未通过并入主线', status: '活跃', plannedEnd: '2027-06-30', note: '2026-09 分叉，首个版本预计 11 月' });

line({ id: 'billing-core:lts-2x', appKey: 'billing-core', name: '2.x 长期支持', branch: 'release/2.x-lts', sourceVersionId: b230, reason: '存量客户 2026 年内无法升级到 3.x，承诺 2.x 安全修复到期为止', status: '活跃', plannedEnd: '2026-10-15' });
v('billing-core:lts-2x', '2.3.1', '2025-11-03');
v('billing-core:lts-2x', '2.3.2', '2026-03-17');
v('billing-core:lts-2x', 'v2026.08', '2026-08-12', { kind: 'commit', value: '9f03be1' }, '日期版本号，格式不强制');

// order-svc: mainline is not called main
line({ id: 'order-svc:main', appKey: 'order-svc', isMain: true, name: '主线', branch: 'develop', status: '活跃' });
v('order-svc:main', 'v3.3', '2025-09-01');
const o34 = v('order-svc:main', 'v3.4', '2026-02-10');
v('order-svc:main', 'v4.0', '2026-07-19');
line({ id: 'order-svc:gov', appKey: 'order-svc', name: '3.x 政务云', branch: 'maint/3.x-govcloud', sourceVersionId: o34, reason: '政务云环境只允许部署已过安全审查的 3.4', status: '活跃', plannedEnd: '2026-10-31' });
v('order-svc:gov', 'v3.4.1-gov', '2026-05-30');

// risk-engine: mainline only
line({ id: 'risk-engine:main', appKey: 'risk-engine', isMain: true, name: '主线', branch: 'main', status: '活跃' });
v('risk-engine:main', '0.9.0', '2026-03-02');
v('risk-engine:main', '1.0.0', '2026-09-14');

// settle-batch: one retired line
line({ id: 'settle-batch:main', appKey: 'settle-batch', isMain: true, name: '主线', branch: 'main', status: '活跃' });
const s52 = v('settle-batch:main', '5.2', '2023-11-20');
v('settle-batch:main', '6.0', '2025-06-01');
line({ id: 'settle-batch:legacy', appKey: 'settle-batch', name: '旧核心兼容', branch: 'maint/legacy-core', sourceVersionId: s52, reason: '旧核心下线前需要保持日终批量接口不变', status: '终止', plannedEnd: '2025-12-31' });
v('settle-batch:legacy', '5.2.7', '2025-03-14');

let state: State = {
  role: 'owner',
  me: 'linqiao',
  users: [
    { username: 'linqiao', displayName: '林乔', email: 'linqiao@corp.example', dept: '交易平台部', status: '正常', identities: [{ type: 'ldap', login: 'linqiao', boundAt: '2026-09-02', boundBy: '首次登录' }] },
    { username: 'chenjing', displayName: '陈静', email: 'chenjing@corp.example', phone: '138 0000 4721', dept: '交易平台部', status: '正常', identities: [{ type: 'ldap', login: 'chenjing', boundAt: '2026-09-02', boundBy: '首次登录' }, { type: 'local', login: 'chenjing', boundAt: '2026-09-10', boundBy: '本人绑定' }] },
    { username: 'zhoumin', displayName: '周敏', email: 'zhoumin@corp.example', dept: '基础架构部', status: '正常', identities: [{ type: 'ldap', login: 'zhoumin', boundAt: '2026-09-03', boundBy: '首次登录' }] },
    { username: 'hanlei', displayName: '韩磊', email: 'hanlei@vendor.example', status: '正常', identities: [{ type: 'local', login: 'hanlei', boundAt: '2026-09-05', boundBy: '管理员 admin' }] },
    { username: 'songyu', displayName: '宋宇', email: 'songyu@corp.example', dept: '风控研发部', status: '已禁用', identities: [{ type: 'ldap', login: 'songyu', boundAt: '2026-09-04', boundBy: '首次登录' }] },
    { username: 'admin', displayName: '初始管理员', email: '', status: '正常', identities: [{ type: 'local', login: 'admin', boundAt: '2026-09-01', boundBy: '首次启动' }] },
  ],
  platforms: [
    { id: 'gl-corp', type: 'GitLab', url: 'https://gitlab.corp.example', tokenTail: 'x9Qk', addedAt: '2026-09-01' },
    { id: 'gh-ent', type: 'GitHub', url: 'https://github.corp.example', tokenTail: '7fA2', addedAt: '2026-09-08' },
  ],
  apps: [
    { key: 'billing-core', name: '计费核心', description: '账单生成、计费规则和对账', repo: { platformId: 'gl-corp', path: 'trade/billing-core', projectId: '1182' }, mainBranch: 'main', owners: ['linqiao', 'chenjing'], pes: ['zhoumin'], deployInfo: '生产：k8s 集群 prod-sh-2，命名空间 billing\n发布窗口：周二、周四 22:00 后\n回滚：helm rollback，保留 3 个历史版本' },
    { key: 'order-svc', name: '订单服务', description: '下单、订单状态机和履约回调', repo: { platformId: 'gh-ent', path: 'platform/order-service', projectId: '552310987' }, mainBranch: 'develop', owners: ['chenjing'], pes: ['linqiao'], deployInfo: '政务云环境由客户自行部署，我方只提供镜像' },
    { key: 'risk-engine', name: '风控引擎', description: '实时交易风控规则执行', repo: { platformId: 'gl-corp', path: 'risk/engine', projectId: '2047' }, mainBranch: 'main', owners: ['zhoumin'], pes: [], deployInfo: '' },
    { key: 'settle-batch', name: '结算批处理', description: '日终清算与结算文件生成', repo: { platformId: 'gl-corp', path: 'trade/settle-batch', projectId: '640' }, mainBranch: 'main', owners: ['hanlei'], pes: ['linqiao'], deployInfo: '日终批量 01:30 启动，失败需人工重跑' },
  ],
  lines,
  versions,
  audit: [
    { appKey: 'billing-core', at: '2026-09-21 16:02', actor: 'linqiao', text: '从 2.1.0 分叉维护线「城商行专版」' },
    { appKey: 'billing-core', at: '2026-08-30 22:41', actor: 'chenjing', text: '在主线登记版本 3.2.0' },
    { appKey: 'billing-core', at: '2026-08-12 10:15', actor: 'chenjing', text: '在「2.x 长期支持」登记版本 v2026.08' },
    { appKey: 'billing-core', at: '2026-06-03 09:30', actor: 'linqiao', text: '将「ACME 华东分部」状态改为冻结' },
    { appKey: 'billing-core', at: '2026-05-11 14:20', actor: 'linqiao', text: '添加 PE 周敏' },
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
    if (!input.key) e.key = '填写应用标识';
    else if (!/^[a-z][a-z0-9-]{1,39}$/.test(input.key)) e.key = '小写字母开头，只能用小写字母、数字和 -，2–40 位';
    else if (state.apps.some((a) => a.key === input.key)) e.key = '应用标识已被占用';
    if (!input.platformId) e.platformId = '选择代码平台';
    if (!input.repoPath.trim()) e.repoPath = '填写仓库路径';
    else if (!input.projectId) e.repoPath = '先校验仓库';
  }
  if (!input.name.trim()) e.name = '填写显示名';
  if (!input.mainBranch.trim()) e.mainBranch = '填写主线分支名';
  if (input.owners.length === 0) e.owners = '至少一个 Owner';
  const inactive = [...input.owners, ...input.pes].filter((u) => state.users.find((x) => x.username === u)?.status !== '正常');
  if (inactive.length) e.owners = `已禁用的用户不能设为 Owner/PE：${inactive.map(userName).join('、')}`;
  return e;
}

/** Fake platform API check. Paths containing "missing" fail, everything else returns a stable project ID. */
export function verifyRepo(platformId: string, path: string): { ok: true; projectId: string } | { ok: false; message: string } {
  const p = state.platforms.find((x) => x.id === platformId);
  if (!p) return { ok: false, message: '选择代码平台' };
  if (!/^[\w.-]+(\/[\w.-]+)+$/.test(path.trim())) return { ok: false, message: '路径格式应为 group/repo' };
  if (path.includes('missing')) return { ok: false, message: `${p.type} 返回 404：仓库不存在或服务令牌无权访问` };
  let h = 7;
  for (const c of path) h = (h * 31 + c.charCodeAt(0)) % 900000;
  return { ok: true, projectId: String(100000 + h) };
}

export type LocalUserInput = { username: string; displayName: string; email: string; dept: string; password: string };
export function validateLocalUser(input: LocalUserInput): Errors<LocalUserInput> {
  const e: Errors<LocalUserInput> = {};
  if (!/^[a-z][a-z0-9._-]{1,31}$/.test(input.username)) e.username = '小写字母开头，2–32 位';
  else if (state.users.some((u) => u.username === input.username)) e.username = '用户名已存在';
  if (!input.displayName.trim()) e.displayName = '填写显示名';
  if (input.email && !/^\S+@\S+$/.test(input.email)) e.email = '邮箱格式不对';
  if (input.password.length < 8) e.password = '至少 8 位';
  return e;
}

export const actions = {
  setRole: (role: Role) => set({ role }),

  createApp(input: AppInput) {
    const app: Application = { key: input.key, name: input.name.trim(), description: input.description.trim(), repo: { platformId: input.platformId, path: input.repoPath.trim(), projectId: input.projectId }, mainBranch: input.mainBranch.trim(), owners: input.owners, pes: input.pes, deployInfo: '' };
    set({ apps: [...state.apps, app], lines: [...state.lines, { id: `${app.key}:main`, appKey: app.key, isMain: true, name: '主线', branch: app.mainBranch, status: '活跃' }] });
    log(app.key, `创建应用，Owner：${input.owners.map(userName).join('、')}`);
  },

  /** 应用标识 and 仓库关联 are not editable here. Owner/PE changes are written to 变更历史 one by one. */
  updateApp(key: string, patch: Pick<AppInput, 'name' | 'description' | 'mainBranch' | 'owners' | 'pes'>) {
    const before = state.apps.find((a) => a.key === key)!;
    set({ apps: state.apps.map((a) => (a.key === key ? { ...a, name: patch.name.trim(), description: patch.description.trim(), mainBranch: patch.mainBranch.trim(), owners: patch.owners, pes: patch.pes } : a)) });
    const diff = (role: string, was: string[], now: string[]) => {
      now.filter((u) => !was.includes(u)).forEach((u) => log(key, `添加 ${role} ${userName(u)}`));
      was.filter((u) => !now.includes(u)).forEach((u) => log(key, `移除 ${role} ${userName(u)}`));
    };
    diff('Owner', before.owners, patch.owners);
    diff('PE', before.pes, patch.pes);
    if (before.name !== patch.name.trim() || before.description !== patch.description.trim() || before.mainBranch !== patch.mainBranch.trim()) log(key, '修改应用基本信息');
  },

  deleteApp(key: string) {
    const lineIds = new Set(state.lines.filter((l) => l.appKey === key).map((l) => l.id));
    set({ apps: state.apps.filter((a) => a.key !== key), lines: state.lines.filter((l) => l.appKey !== key), versions: state.versions.filter((v) => !lineIds.has(v.lineId)) });
  },

  createLocalUser(input: LocalUserInput) {
    set({ users: [...state.users, { username: input.username, displayName: input.displayName.trim(), email: input.email.trim(), dept: input.dept.trim() || undefined, status: '正常', identities: [{ type: 'local', login: input.username, boundAt: TODAY, boundBy: `管理员 ${state.me}` }] }] });
  },

  fork(input: Omit<Line, 'id' | 'isMain' | 'status'>) {
    const id = `${input.appKey}:${input.branch}`;
    set({ lines: [...state.lines, { ...input, id, isMain: false, status: '活跃' }] });
    log(input.appKey, `从 ${versionById(input.sourceVersionId!)?.number} 分叉维护线「${input.name}」`);
    return id;
  },

  registerVersion(input: Omit<Version, 'id'>) {
    const id = `ver${++vid}`;
    set({ versions: [...state.versions, { ...input, id }] });
    const l = lineById(input.lineId)!;
    log(l.appKey, `在${l.isMain ? '主线' : `「${l.name}」`}登记版本 ${input.number}`);
  },

  setLineStatus(lineId: string, status: LineStatus) {
    const l = lineById(lineId)!;
    set({ lines: state.lines.map((x) => (x.id === lineId ? { ...x, status } : x)) });
    log(l.appKey, `将「${l.name}」状态改为${status}`);
  },

  setDeployInfo(appKey: string, deployInfo: string) {
    set({ apps: state.apps.map((a) => (a.key === appKey ? { ...a, deployInfo } : a)) });
    log(appKey, '修改部署相关信息');
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

export function daysUntil(date: string) {
  return Math.round((Date.parse(date) - Date.parse(TODAY)) / 86400000);
}
export function isExpiring(l: Line) {
  return !!l.plannedEnd && l.status !== '终止' && daysUntil(l.plannedEnd) <= EXPIRY_WINDOW_DAYS;
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
