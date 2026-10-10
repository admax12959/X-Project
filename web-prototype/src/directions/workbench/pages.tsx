import { useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  actions,
  appLines,
  appVersions,
  can,
  childLines,
  daysUntil,
  isExpiring,
  lineageOrder,
  lineVersions,
  soleOwnerOf,
  useStore,
  userName,
  type Application,
  type IdentityType,
  type Line,
  type LineStatus,
  type User,
  type Version,
} from '../../data';
import { navigate, useLocation } from '../../router';
import { BindDialog, CreateAppDialog, CreateUserDialog, DeleteAppDialog, DisableUserDialog, EditAppDialog, ForkDialog, nextStatus, PlatformDialog, RegisterVersionDialog, StatusDialog } from './dialogs';
import { Legend, LineageGraph } from './graph';
import { Page, PageTitle, Shell } from './shell';
import { Btn, Dd, Dialog, Dl, Dt, dueText, Field, IDENTITY_LABEL, Input, LinkBtn, Mono, PaneHeading, PlannedEnd, RailGlyph, Textarea } from './ui';

const lineLabel = (l: Line) => (l.isMain ? '主线' : l.name);

// ================================================================= 登录

export function Login() {
  const [tab, setTab] = useState<IdentityType>('ldap');
  const [login, setLogin] = useState('');
  const [pw, setPw] = useState('');
  const [tried, setTried] = useState(false);
  const errs = { login: !login.trim() ? '填写用户名' : undefined, pw: !pw ? '填写密码' : undefined };
  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    setTried(true);
    if (errs.login || errs.pw) return;
    navigate('/apps');
  };
  return (
    <div className="grid min-h-screen grid-cols-[minmax(0,1fr)_480px] bg-paper">
      <div className="flex flex-col justify-between border-r border-ink px-14 py-12">
        <div />
        <div className="max-w-[30em]">
          <h1 className="text-[40px] leading-none font-bold tracking-[-0.01em]">应用维护</h1>
          <p className="mt-5 text-[length:var(--wb-text-md)] leading-relaxed text-ink-2">
            登记公司内部应用的仓库关联、Owner 和 PE，以及主线和每条维护线的版本：从哪个版本分叉、为什么分叉、计划什么时候终止。
          </p>
        </div>
        <div className="text-[length:var(--wb-text-sm)] text-ink-3">内部工具 · 登录问题联系管理员</div>
      </div>
      <form onSubmit={submit} className="flex flex-col justify-center px-12" noValidate>
        <h2 className="text-[length:var(--wb-text-lg)] font-bold">登录</h2>
        <div role="tablist" className="mt-5 flex border-b border-line">
          {(['ldap', 'local'] as const).map((t) => (
            <button
              key={t}
              type="button"
              role="tab"
              aria-selected={tab === t}
              onClick={() => (setTab(t), setTried(false))}
              className={`-mb-px border-b-2 px-1 pb-2 mr-6 ${tab === t ? 'border-ink font-semibold' : 'border-transparent text-ink-2 hover:text-ink'}`}
            >
              {t === 'ldap' ? 'LDAP' : '本地账号密码'}
            </button>
          ))}
        </div>
        <div className="mt-6 space-y-4">
          <Field label={tab === 'ldap' ? 'LDAP 用户名' : '登录名'} error={tried ? errs.login : undefined}>
            <Input value={login} onChange={(e) => setLogin(e.target.value)} autoComplete="username" invalid={tried && !!errs.login} />
          </Field>
          <Field label="密码" error={tried ? errs.pw : undefined}>
            <Input type="password" value={pw} onChange={(e) => setPw(e.target.value)} autoComplete="current-password" invalid={tried && !!errs.pw} />
          </Field>
          <Btn type="submit" kind="primary" className="h-9 w-full justify-center text-[length:var(--wb-text-base)]">登录</Btn>
          <p className="text-[length:var(--wb-text-sm)] leading-relaxed text-ink-3">
            {tab === 'ldap' ? '用公司 LDAP 用户名和密码登录。第一次登录时会自动创建你的用户。' : '本地账号密码由管理员创建，或在个人设置里自行绑定。'}
          </p>
        </div>
      </form>
    </div>
  );
}

// ================================================================= 应用列表

type SortKey = 'name' | 'lines' | 'latest' | 'due' | 'owners';

export function AppList() {
  const s = useStore();
  const [creating, setCreating] = useState(false);
  const [sort, setSort] = useState<{ key: SortKey; dir: 1 | -1 }>({ key: 'due', dir: 1 });
  const isAdmin = can(s.role).admin;

  const expiring = s.lines
    .filter(isExpiring)
    .sort((a, b) => a.plannedEnd!.localeCompare(b.plannedEnd!));

  const rows = useMemo(() => {
    const data = s.apps.map((a) => {
      const ls = appLines(s, a.key).filter((l) => !l.isMain);
      const vs = appVersions(s, a.key);
      const latest = vs.at(-1);
      const open = ls.filter((l) => l.status !== '终止' && l.plannedEnd).sort((x, y) => x.plannedEnd!.localeCompare(y.plannedEnd!));
      return { a, ls, latest, latestLine: latest ? s.lines.find((l) => l.id === latest.lineId) : undefined, next: open[0] };
    });
    const val = (r: (typeof data)[number]): string | number => {
      switch (sort.key) {
        case 'name': return r.a.key;
        case 'lines': return r.ls.filter((l) => l.status !== '终止').length;
        case 'latest': return r.latest?.releasedAt ?? '';
        case 'due': return r.next?.plannedEnd ?? '9999';
        case 'owners': return r.a.owners.map(userName).join();
      }
    };
    return data.sort((x, y) => (val(x) < val(y) ? -1 : val(x) > val(y) ? 1 : 0) * sort.dir);
  }, [s, sort]);

  const th = (key: SortKey, label: string, cls = '') => (
    <th className={cls} aria-sort={sort.key === key ? (sort.dir === 1 ? 'ascending' : 'descending') : 'none'}>
      <button type="button" onClick={() => setSort((p) => ({ key, dir: p.key === key ? ((-p.dir) as 1 | -1) : 1 }))} className="inline-flex items-center gap-1 font-semibold hover:underline">
        {label}
        <span className={`text-[10px] ${sort.key === key ? 'text-ink' : 'text-line'}`} aria-hidden>{sort.key === key && sort.dir === -1 ? '▲' : '▼'}</span>
      </button>
    </th>
  );

  return (
    <Shell>
      <Page>
        <PageTitle
          title="应用"
          sub={`${s.apps.length} 个应用，${s.lines.filter((l) => !l.isMain && l.status !== '终止').length} 条维护线仍在维护。`}
          actions={isAdmin && <Btn kind="primary" onClick={() => setCreating(true)}>创建应用</Btn>}
        />

        {/* focal: what ends soon */}
        <section className="mb-10">
          <div className="flex items-baseline justify-between border-b-2 border-accent pb-2">
            <h2 className="text-[length:var(--wb-text-lg)] font-bold">
              30 天内到期的维护线
              <span className="ml-2 text-accent">{expiring.length}</span>
            </h2>
            <span className="text-[length:var(--wb-text-sm)] text-ink-3">按计划终止日期排序，点一行打开那条维护线</span>
          </div>
          {expiring.length === 0 ? (
            <p className="py-4 text-ink-2">30 天内没有到期的维护线。</p>
          ) : (
            <ul>
              {expiring.map((l) => {
                const app = s.apps.find((a) => a.key === l.appKey)!;
                const d = daysUntil(l.plannedEnd!);
                const vs = lineVersions(s, l.id);
                return (
                  <li key={l.id} className="border-b border-line-2">
                    <button
                      type="button"
                      onClick={() => navigate(`/apps/${l.appKey}?line=${encodeURIComponent(l.id)}`)}
                      className="grid w-full grid-cols-[96px_minmax(0,1.1fr)_minmax(0,1.6fr)_150px] items-center gap-6 py-3.5 text-left hover:bg-[var(--wb-hover-row)]"
                    >
                      <span className="pl-1 leading-none text-accent">
                        <span className="text-[length:var(--wb-text-xl)] font-bold">{d < 0 ? -d : d}</span>
                        <span className="ml-1 font-semibold">{d < 0 ? '天前过期' : '天后'}</span>
                      </span>
                      <span className="min-w-0">
                        <span className="block truncate text-[length:var(--wb-text-md)] font-semibold">{l.name}</span>
                        <span className="mt-0.5 block truncate text-[length:var(--wb-text-sm)] text-ink-2">
                          {app.name} · <Mono>{l.branch}</Mono> · {l.status}
                        </span>
                      </span>
                      <span className="min-w-0 text-[length:var(--wb-text-sm)] leading-snug text-ink-2">
                        <span className="line-clamp-2">{l.reason}</span>
                      </span>
                      <span className="text-right text-[length:var(--wb-text-sm)]">
                        <span className="block font-semibold">{l.plannedEnd}</span>
                        <span className="text-ink-3">最近版本 {vs.at(-1)?.number ?? '无'}</span>
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </section>

        <section>
          <h2 className="mb-2 text-[length:var(--wb-text-md)] font-bold">全部应用</h2>
          <table className="wb-table">
            <thead>
              <tr>
                {th('name', '应用')}
                {th('lines', '维护线')}
                {th('latest', '最近登记的版本')}
                {th('due', '最近计划终止')}
                {th('owners', 'Owner')}
                <th>PE</th>
                <th>仓库关联</th>
              </tr>
            </thead>
            <tbody>
              {rows.map(({ a, ls, latest, latestLine, next }) => {
                const by = (st: LineStatus) => ls.filter((l) => l.status === st).length;
                const platform = s.platforms.find((p) => p.id === a.repo.platformId);
                return (
                  <tr key={a.key} className="wb-click" onClick={() => navigate(`/apps/${a.key}`)}>
                    <td>
                      <div className="font-semibold">{a.name}</div>
                      <Mono className="text-[length:var(--wb-text-xs)] text-ink-2">{a.key}</Mono>
                    </td>
                    <td className="whitespace-nowrap">
                      {ls.length === 0 ? (
                        <span className="text-ink-3">只有主线</span>
                      ) : (
                        <span className="flex flex-col gap-0.5 text-[length:var(--wb-text-sm)]">
                          {(['活跃', '冻结', '终止'] as const).map((st) =>
                            by(st) ? (
                              <span key={st} className="flex items-center gap-1.5">
                                <RailGlyph status={st} width={14} />
                                <span className={st === '终止' ? 'text-ink-3' : ''}>{st} {by(st)}</span>
                              </span>
                            ) : null,
                          )}
                        </span>
                      )}
                    </td>
                    <td className="whitespace-nowrap">
                      {latest ? (
                        <>
                          <div className="font-semibold">{latest.number}</div>
                          <div className="text-[length:var(--wb-text-xs)] text-ink-2">{latest.releasedAt} · {latestLine && lineLabel(latestLine)}</div>
                        </>
                      ) : (
                        <span className="text-ink-3">还没有版本</span>
                      )}
                    </td>
                    <td className="whitespace-nowrap">
                      {next ? (
                        <>
                          <div className={isExpiring(next) ? 'font-semibold text-accent' : ''}>{next.plannedEnd}{isExpiring(next) && `，${dueText(next)}`}</div>
                          <div className="text-[length:var(--wb-text-xs)] text-ink-2">{next.name}</div>
                        </>
                      ) : (
                        <span className="text-ink-3">—</span>
                      )}
                    </td>
                    <td>{a.owners.map(userName).join('、')}</td>
                    <td>{a.pes.length ? a.pes.map(userName).join('、') : <span className="text-ink-3">未指定</span>}</td>
                    <td className="text-[length:var(--wb-text-sm)]">
                      <Mono>{a.repo.path}</Mono>
                      <div className="text-[length:var(--wb-text-xs)] text-ink-3">{platform?.type}</div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          {s.apps.length === 0 && <p className="py-6 text-ink-2">还没有应用。{isAdmin ? '点右上角“创建应用”登记第一个。' : '应用由管理员创建。'}</p>}
        </section>
      </Page>
      {creating && <CreateAppDialog onClose={() => setCreating(false)} />}
    </Shell>
  );
}

// ================================================================= 应用详情

type Dlg =
  | { k: 'fork'; src: string }
  | { k: 'register'; line: Line }
  | { k: 'status'; line: Line; to: LineStatus }
  | { k: 'edit' }
  | { k: 'delete' }
  | null;

// Keyed by appKey: switching apps in the tree must not carry over open editors, dialogs or scroll state.
export function AppDetail({ appKey }: { appKey: string }) {
  return <AppDetailView key={appKey} appKey={appKey} />;
}

function AppDetailView({ appKey }: { appKey: string }) {
  const s = useStore();
  const { params } = useLocation();
  const [dlg, setDlg] = useState<Dlg>(null);
  const app = s.apps.find((a) => a.key === appKey);
  const rows = app ? lineageOrder(s, appKey) : [];
  const lineIds = rows.map((r) => r.line.id);

  const verParam = params.get('ver');
  const selVerObj = verParam ? s.versions.find((v) => v.id === verParam && lineIds.includes(v.lineId)) : undefined;
  const lineParam = params.get('line');
  const selLineId = selVerObj ? selVerObj.lineId : lineParam && lineIds.includes(lineParam) ? lineParam : null;
  const selLine = selLineId ? rows.find((r) => r.line.id === selLineId)!.line : undefined;

  const select = (lineId: string | null, verId: string | null = null) => {
    const q = new URLSearchParams();
    if (lineId) q.set('line', lineId);
    if (verId) q.set('ver', verId);
    const qs = q.toString();
    navigate(`/apps/${appKey}${qs ? `?${qs}` : ''}`, { replace: true });
  };

  // j/k (or ↑/↓) move between lines, h/l (or ←/→) between versions on the line, Esc steps back out.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.altKey || e.metaKey || e.ctrlKey) return;
      const t = e.target as HTMLElement;
      if (t.closest('input, textarea, select, [contenteditable]') || document.querySelector('[role=dialog]')) return;
      const i = selLineId ? lineIds.indexOf(selLineId) : -1;
      if (e.key === 'j' || e.key === 'ArrowDown') {
        e.preventDefault();
        select(lineIds[Math.min(i + 1, lineIds.length - 1)]);
      } else if (e.key === 'k' || e.key === 'ArrowUp') {
        e.preventDefault();
        select(i <= 0 ? null : lineIds[i - 1]);
      } else if ((e.key === 'h' || e.key === 'ArrowLeft' || e.key === 'l' || e.key === 'ArrowRight') && selLineId) {
        e.preventDefault();
        const vs = lineVersions(s, selLineId);
        if (!vs.length) return;
        const vi = selVerObj ? vs.findIndex((v) => v.id === selVerObj.id) : -1;
        const fwd = e.key === 'l' || e.key === 'ArrowRight';
        const ni = vi === -1 ? (fwd ? 0 : vs.length - 1) : Math.max(0, Math.min(vs.length - 1, vi + (fwd ? 1 : -1)));
        select(selLineId, vs[ni].id);
      } else if (e.key === 'Escape') {
        if (selVerObj) select(selLineId);
        else if (selLineId) select(null);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  if (!app)
    return (
      <Shell>
        <Page>
          <PageTitle title="应用不存在" sub={<>没有应用标识为 <Mono>{appKey}</Mono> 的应用，可能已被管理员删除。</>} actions={<Btn onClick={() => navigate('/apps')}>回到应用列表</Btn>} />
        </Page>
      </Shell>
    );

  const perm = can(s.role);
  const maint = rows.filter((r) => !r.line.isMain).map((r) => r.line);
  const mainLine = rows[0]?.line;
  const allVers = appVersions(s, appKey);
  const defaultSrc = selVerObj?.id ?? (selLine ? lineVersions(s, selLine.id).at(-1)?.id : undefined) ?? (mainLine ? lineVersions(s, mainLine.id).at(-1)?.id : undefined) ?? allVers.at(-1)?.id;
  const soon = maint.filter(isExpiring);
  const count = (st: LineStatus) => maint.filter((l) => l.status === st).length;

  return (
    <Shell>
      {/* center canvas */}
      <main className="wb-scroll min-w-0 flex-1 overflow-y-auto">
        <div className="px-6 pt-5">
          <div className="flex items-start justify-between gap-6">
            <div className="min-w-0">
              <div className="flex items-baseline gap-3">
                <h1 className="text-[length:var(--wb-text-xl)] leading-none font-bold tracking-[-0.01em]">{app.name}</h1>
                <Mono className="text-[length:var(--wb-text-md)] text-ink-2">{app.key}</Mono>
              </div>
              {app.description && <p className="mt-2 text-ink-2">{app.description}</p>}
              <p className="mt-1 text-ink-2">
                主线 {mainLine ? lineVersions(s, mainLine.id).length : 0} 个版本 · {maint.length ? `${maint.length} 条维护线：活跃 ${count('活跃')}、冻结 ${count('冻结')}、终止 ${count('终止')}` : '还没有维护线'}
                {soon.length > 0 && <span className="ml-1 font-semibold text-accent">· {soon.length} 条 30 天内到期</span>}
              </p>
            </div>
            <div className="flex shrink-0 gap-2">
              {perm.manageLines && (
                <Btn kind="primary" disabled={!defaultSrc} title={!defaultSrc ? '先在主线登记一个版本，才能分叉' : undefined} onClick={() => defaultSrc && setDlg({ k: 'fork', src: defaultSrc })}>
                  分叉维护线
                </Btn>
              )}
              {perm.editApp && <Btn onClick={() => setDlg({ k: 'edit' })}>编辑应用</Btn>}
              {perm.admin && <Btn kind="danger" onClick={() => setDlg({ k: 'delete' })}>删除应用</Btn>}
            </div>
          </div>
          <div className="mt-5 mb-2 flex items-end justify-between gap-4">
            <h2 className="text-[length:var(--wb-text-md)] font-bold">分支谱系</h2>
            <Legend />
          </div>
        </div>
        <LineageGraph appKey={appKey} mainBranch={app.mainBranch} selLine={selLineId} selVer={selVerObj?.id ?? null} onLine={(id) => select(id === selLineId && !selVerObj ? null : id)} onVer={(v) => select(v.lineId, v.id)} />
        <div className="flex justify-between px-6 pt-2 text-[length:var(--wb-text-xs)] text-ink-3">
          <span>点线或版本查看详情。键盘：j / k 切换线，h / l 切换版本，Esc 返回上一级。</span>
        </div>

        <VersionLedger app={app} rows={rows.map((r) => r.line)} selVer={selVerObj?.id ?? null} onVer={(v) => select(v.lineId, v.id)} />
      </main>

      {/* inspector */}
      <aside className="wb-scroll w-[348px] shrink-0 overflow-y-auto border-l border-ink bg-paper" aria-label="详情">
        <div className="sticky top-0 z-[2] flex flex-wrap items-center gap-1.5 border-b border-line bg-paper px-5 py-2.5 text-[length:var(--wb-text-sm)]">
          <button type="button" className={selLine ? 'wb-link' : 'font-semibold'} onClick={() => select(null)}>{app.name}</button>
          {selLine && (
            <>
              <span className="text-ink-3">/</span>
              <button type="button" className={selVerObj ? 'wb-link' : 'font-semibold'} onClick={() => select(selLine.id)}>{lineLabel(selLine)}</button>
            </>
          )}
          {selVerObj && (
            <>
              <span className="text-ink-3">/</span>
              <span className="font-semibold">{selVerObj.number}</span>
            </>
          )}
        </div>
        <div className="px-5 pt-4 pb-24">
          {selVerObj && selLine ? (
            <VersionInspector app={app} v={selVerObj} line={selLine} onFork={() => setDlg({ k: 'fork', src: selVerObj.id })} onSelectLine={(id) => select(id)} />
          ) : selLine ? (
            <LineInspector
              app={app}
              line={selLine}
              onVer={(v) => select(v.lineId, v.id)}
              onSelectLine={(id) => select(id)}
              onFork={(src) => setDlg({ k: 'fork', src })}
              onRegister={() => setDlg({ k: 'register', line: selLine })}
              onStatus={(to) => setDlg({ k: 'status', line: selLine, to })}
            />
          ) : (
            <AppInspector app={app} onEdit={() => setDlg({ k: 'edit' })} />
          )}
        </div>
      </aside>

      {dlg?.k === 'fork' && <ForkDialog appKey={appKey} mainBranch={app.mainBranch} sourceVersionId={dlg.src} onClose={() => setDlg(null)} onCreated={(id) => select(id)} />}
      {dlg?.k === 'register' && <RegisterVersionDialog line={dlg.line} mainBranch={app.mainBranch} onClose={() => setDlg(null)} />}
      {dlg?.k === 'status' && <StatusDialog line={dlg.line} to={dlg.to} onClose={() => setDlg(null)} />}
      {dlg?.k === 'edit' && <EditAppDialog app={app} onClose={() => setDlg(null)} />}
      {dlg?.k === 'delete' && <DeleteAppDialog app={app} onClose={() => setDlg(null)} />}
    </Shell>
  );
}

function VersionLedger({ app, rows, selVer, onVer }: { app: Application; rows: Line[]; selVer: string | null; onVer: (v: Version) => void }) {
  const s = useStore();
  const vs = appVersions(s, app.key).slice().reverse();
  const byId = new Map(rows.map((l) => [l.id, l]));
  return (
    <section className="px-6 pt-8 pb-24">
      <h2 className="mb-2 text-[length:var(--wb-text-md)] font-bold">
        全部版本 <span className="font-normal text-ink-3">{vs.length}</span>
      </h2>
      {vs.length === 0 ? (
        <p className="text-ink-2">还没有登记版本。</p>
      ) : (
        <table className="wb-table text-[length:var(--wb-text-sm)]">
          <thead>
            <tr>
              <th>发布时间</th>
              <th>版本号</th>
              <th>所在线</th>
              <th>tag / commit</th>
              <th>说明</th>
            </tr>
          </thead>
          <tbody>
            {vs.map((v) => {
              const l = byId.get(v.lineId)!;
              return (
                <tr key={v.id} className="wb-click" aria-selected={selVer === v.id} onClick={() => onVer(v)}>
                  <td className="whitespace-nowrap text-ink-2">{v.releasedAt}</td>
                  <td className="font-semibold whitespace-nowrap">{v.number}</td>
                  <td className="whitespace-nowrap">
                    <span className="inline-flex items-center gap-1.5">
                      <RailGlyph status={l.status} main={l.isMain} width={14} />
                      <span className={l.status === '终止' ? 'text-ink-3' : ''}>{lineLabel(l)}</span>
                    </span>
                  </td>
                  <td className="whitespace-nowrap">
                    <span className="text-ink-3">{v.ref.kind}</span> <Mono>{v.ref.value}</Mono>
                  </td>
                  <td className="text-ink-2">{v.note || ''}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </section>
  );
}

// ---------------------------------------------------------------- inspectors

function Section({ title, actions, children, first = false }: { title: ReactNode; actions?: ReactNode; children: ReactNode; first?: boolean }) {
  return (
    <section className={first ? '' : 'mt-7 border-t border-line pt-5'}>
      <PaneHeading actions={actions}>{title}</PaneHeading>
      {children}
    </section>
  );
}

function AppInspector({ app, onEdit }: { app: Application; onEdit: () => void }) {
  const s = useStore();
  const perm = can(s.role);
  const platform = s.platforms.find((p) => p.id === app.repo.platformId);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(app.deployInfo);
  const [allHistory, setAllHistory] = useState(false);
  const history = s.audit.filter((e) => e.appKey === app.key);
  const shown = allHistory ? history : history.slice(0, 8);

  return (
    <>
      <Section first title="应用信息" actions={perm.editApp && <Btn onClick={onEdit}>编辑</Btn>}>
        <Dl>
          <Dt>应用标识</Dt>
          <Dd><Mono>{app.key}</Mono></Dd>
          <Dt>显示名</Dt>
          <Dd>{app.name}</Dd>
          <Dt>描述</Dt>
          <Dd>{app.description || <span className="text-ink-3">未填写</span>}</Dd>
          <Dt>仓库关联</Dt>
          <Dd>
            <Mono className="font-semibold">{app.repo.path}</Mono>
            <div className="mt-0.5 text-[length:var(--wb-text-sm)] text-ink-2">
              {platform?.type} · <Mono>{platform?.url.replace(/^https?:\/\//, '')}</Mono> · 项目 ID {app.repo.projectId}
            </div>
          </Dd>
          <Dt>主线分支</Dt>
          <Dd><Mono>{app.mainBranch}</Mono></Dd>
          <Dt>Owner</Dt>
          <Dd>{app.owners.map(userName).join('、')}</Dd>
          <Dt>PE</Dt>
          <Dd>{app.pes.length ? app.pes.map(userName).join('、') : <span className="text-ink-3">未指定</span>}</Dd>
        </Dl>
      </Section>

      <Section
        title="部署相关信息"
        actions={perm.editDeploy && !editing && <Btn onClick={() => (setDraft(app.deployInfo), setEditing(true))}>编辑</Btn>}
      >
        {editing ? (
          <div>
            <Textarea aria-label="部署相关信息" rows={6} value={draft} onChange={(e) => setDraft(e.target.value)} autoFocus />
            <div className="mt-2 flex justify-end gap-2">
              <Btn onClick={() => setEditing(false)}>取消</Btn>
              <Btn kind="primary" onClick={() => (actions.setDeployInfo(app.key, draft.trim()), setEditing(false))}>保存</Btn>
            </div>
          </div>
        ) : app.deployInfo ? (
          <p className="leading-relaxed whitespace-pre-wrap">{app.deployInfo}</p>
        ) : (
          <p className="text-ink-3">还没有填写。Owner 和 PE 可以编辑：部署位置、发布窗口、回滚方式。</p>
        )}
      </Section>

      <Section title={<>变更历史 <span className="font-normal text-ink-3">{history.length}</span></>}>
        {history.length === 0 ? (
          <p className="text-ink-3">没有记录。</p>
        ) : (
          <ol className="space-y-2.5">
            {shown.map((e, i) => (
              <li key={i} className="grid grid-cols-[86px_minmax(0,1fr)] gap-x-2 text-[length:var(--wb-text-sm)]">
                <span className="text-ink-3">{e.at.slice(0, 10)}</span>
                <span>
                  {e.text}
                  <span className="ml-1.5 text-ink-3">{userName(e.actor)}</span>
                </span>
              </li>
            ))}
          </ol>
        )}
        {history.length > 8 && (
          <LinkBtn className="mt-3 text-[length:var(--wb-text-sm)]" onClick={() => setAllHistory(!allHistory)}>{allHistory ? '只看最近 8 条' : `显示全部 ${history.length} 条`}</LinkBtn>
        )}
      </Section>
    </>
  );
}

function StatusTrack({ status }: { status: LineStatus }) {
  const order: LineStatus[] = ['活跃', '冻结', '终止'];
  const at = order.indexOf(status);
  return (
    <ol className="flex items-center text-[length:var(--wb-text-sm)]" aria-label="状态">
      {order.map((st, i) => (
        <li key={st} className="flex items-center">
          {i > 0 && <span className={`mx-2 h-px w-6 ${i <= at ? 'bg-ink' : 'bg-line'}`} aria-hidden />}
          <span className={`flex items-center gap-1.5 ${i === at ? 'font-bold' : i < at ? 'text-ink-2' : 'text-ink-3'}`} aria-current={i === at ? 'step' : undefined}>
            <RailGlyph status={st} width={16} />
            {st}
          </span>
        </li>
      ))}
    </ol>
  );
}

function LineInspector({ app, line, onVer, onSelectLine, onFork, onRegister, onStatus }: { app: Application; line: Line; onVer: (v: Version) => void; onSelectLine: (id: string) => void; onFork: (src: string) => void; onRegister: () => void; onStatus: (to: LineStatus) => void }) {
  const s = useStore();
  const perm = can(s.role);
  const vs = lineVersions(s, line.id);
  const src = line.sourceVersionId ? s.versions.find((v) => v.id === line.sourceVersionId) : undefined;
  const srcLine = src ? s.lines.find((l) => l.id === src.lineId) : undefined;
  const children = vs.flatMap((v) => childLines(s, v.id).map((c) => ({ c, v })));
  const next = nextStatus(line);
  const retired = line.status === '终止';
  const due = isExpiring(line) ? dueText(line) : null;

  return (
    <>
      <div>
        <h2 className="text-[length:var(--wb-text-lg)] leading-tight font-bold">{lineLabel(line)}</h2>
        <div className="mt-1.5 flex items-center gap-2 text-[length:var(--wb-text-sm)]">
          <Mono className="text-ink-2">{line.isMain ? app.mainBranch : line.branch}</Mono>
        </div>
        {due && <p className="mt-2 font-semibold text-accent">计划 {line.plannedEnd} 终止，{due}。</p>}
      </div>

      {!line.isMain && (
        <div className="mt-4 border-y border-line-2 py-3">
          <StatusTrack status={line.status} />
          {retired && <p className="mt-2 text-[length:var(--wb-text-sm)] text-ink-2">已终止：只保留只读记录，不能再登记版本，也不能删除。</p>}
          {line.status === '冻结' && <p className="mt-2 text-[length:var(--wb-text-sm)] text-ink-2">已冻结：只允许紧急修复，登记版本时必须写说明。</p>}
        </div>
      )}

      {perm.manageLines && (
        <div className="mt-4 flex flex-wrap gap-2">
          {!retired && <Btn kind="primary" onClick={onRegister}>登记版本</Btn>}
          {vs.length > 0 && <Btn onClick={() => onFork(vs[vs.length - 1].id)}>从 {vs[vs.length - 1].number} 分叉</Btn>}
          {next && <Btn kind={next === '终止' ? 'danger' : 'plain'} onClick={() => onStatus(next)}>改为{next}</Btn>}
        </div>
      )}

      <Section title="维护线信息">
        <Dl>
          {line.isMain ? (
            <>
              <Dt>说明</Dt>
              <Dd className="text-ink-2">应用持续迭代所在的分支。主线没有源版本和状态变更。</Dd>
            </>
          ) : (
            <>
              <Dt>源版本</Dt>
              <Dd>
                {src && srcLine ? (
                  <LinkBtn onClick={() => onVer(src)}>
                    <span className="font-semibold">{src.number}</span>
                    <span className="text-ink-2"> · {lineLabel(srcLine)} · {src.releasedAt}</span>
                  </LinkBtn>
                ) : (
                  <span className="text-ink-3">—</span>
                )}
              </Dd>
              <Dt>分叉原因</Dt>
              <Dd className="leading-relaxed">{line.reason}</Dd>
              <Dt>计划终止</Dt>
              <Dd><PlannedEnd line={line} /></Dd>
              <Dt>备注</Dt>
              <Dd className={line.note ? '' : 'text-ink-3'}>{line.note || '无'}</Dd>
            </>
          )}
        </Dl>
      </Section>

      <Section title={<>版本 <span className="font-normal text-ink-3">{vs.length}</span></>}>
        {vs.length === 0 ? (
          <p className="text-ink-3">{retired ? '这条线终止前没有登记版本。' : '还没有登记版本。'}</p>
        ) : (
          <ol className="border-t border-line-2">
            {vs.slice().reverse().map((v) => (
              <li key={v.id} className="flex items-baseline gap-3 border-b border-line-2 py-2">
                <button type="button" onClick={() => onVer(v)} className="min-w-0 flex-1 text-left hover:underline">
                  <span className="font-semibold">{v.number}</span>
                  <span className="ml-2 text-[length:var(--wb-text-sm)] text-ink-3">{v.releasedAt}</span>
                  {v.note && <span className="mt-0.5 block truncate text-[length:var(--wb-text-sm)] text-ink-2">{v.note}</span>}
                </button>
                {perm.manageLines && <LinkBtn className="shrink-0 text-[length:var(--wb-text-sm)]" onClick={() => onFork(v.id)}>分叉</LinkBtn>}
              </li>
            ))}
          </ol>
        )}
      </Section>

      {children.length > 0 && (
        <Section title="从这条线分叉出的维护线">
          <ul className="space-y-1.5">
            {children.map(({ c, v }) => (
              <li key={c.id} className="flex items-center gap-2">
                <RailGlyph status={c.status} width={16} />
                <LinkBtn onClick={() => onSelectLine(c.id)}>{c.name}</LinkBtn>
                <span className="text-[length:var(--wb-text-sm)] text-ink-3">从 {v.number}</span>
              </li>
            ))}
          </ul>
        </Section>
      )}
    </>
  );
}

function VersionInspector({ app, v, line, onFork, onSelectLine }: { app: Application; v: Version; line: Line; onFork: () => void; onSelectLine: (id: string) => void }) {
  const s = useStore();
  const perm = can(s.role);
  const kids = childLines(s, v.id);
  return (
    <>
      <h2 className="text-[length:var(--wb-text-xl)] leading-none font-bold">{v.number}</h2>
      <p className="mt-2 text-ink-2">
        {v.releasedAt} 发布于{' '}
        <LinkBtn onClick={() => onSelectLine(line.id)}>{lineLabel(line)}</LinkBtn>
        {line.status === '终止' && '（已终止）'}
      </p>
      {perm.manageLines && (
        <div className="mt-4">
          <Btn kind="primary" onClick={onFork}>从此版本分叉</Btn>
        </div>
      )}
      <Section title="版本信息">
        <Dl>
          <Dt>版本号</Dt>
          <Dd className="font-semibold">{v.number}</Dd>
          <Dt>{v.ref.kind}</Dt>
          <Dd><Mono>{v.ref.value}</Mono></Dd>
          <Dt>发布时间</Dt>
          <Dd>{v.releasedAt}</Dd>
          <Dt>分支</Dt>
          <Dd><Mono>{line.isMain ? app.mainBranch : line.branch}</Mono></Dd>
          <Dt>说明</Dt>
          <Dd className={v.note ? 'leading-relaxed' : 'text-ink-3'}>{v.note || '无'}</Dd>
        </Dl>
      </Section>
      <Section title="以此为源版本的维护线">
        {kids.length === 0 ? (
          <p className="text-ink-3">没有。</p>
        ) : (
          <ul className="space-y-2">
            {kids.map((c) => (
              <li key={c.id}>
                <span className="flex items-center gap-2">
                  <RailGlyph status={c.status} width={16} />
                  <LinkBtn onClick={() => onSelectLine(c.id)} className="font-semibold">{c.name}</LinkBtn>
                  <span className="text-[length:var(--wb-text-sm)] text-ink-3">{c.status}</span>
                </span>
                <span className="mt-0.5 block pl-6 text-[length:var(--wb-text-sm)] text-ink-2">{c.reason}</span>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </>
  );
}

// ================================================================= 个人设置

export function Settings() {
  const s = useStore();
  const me = s.users.find((u) => u.username === s.me)!;
  const [binding, setBinding] = useState<IdentityType | null>(null);
  const [unbinding, setUnbinding] = useState<IdentityType | null>(null);
  const hasLdap = me.identities.some((i) => i.type === 'ldap');
  const unbound = (['ldap', 'local'] as const).filter((t) => !me.identities.some((i) => i.type === t));
  const last = me.identities.length === 1;
  const owns = s.apps.filter((a) => a.owners.includes(me.username));
  const pes = s.apps.filter((a) => a.pes.includes(me.username));

  return (
    <Shell>
      <Page>
        <PageTitle title="个人设置" sub={hasLdap ? '显示名、邮箱和部门以 LDAP 为准，每次登录时同步，这里只能查看。' : '用户信息在这里只能查看。'} />
        <div className="grid max-w-[1040px] grid-cols-[minmax(0,1fr)_minmax(0,1.25fr)] gap-14">
          <section>
            <h2 className="mb-3 border-b-2 border-ink pb-2 text-[length:var(--wb-text-md)] font-bold">用户信息</h2>
            <Dl>
              <Dt>用户名</Dt>
              <Dd>{me.username}</Dd>
              <Dt>显示名</Dt>
              <Dd>{me.displayName}{hasLdap && <span className="ml-2 text-[length:var(--wb-text-xs)] text-ink-3">以 LDAP 为准</span>}</Dd>
              <Dt>邮箱</Dt>
              <Dd>{me.email || <span className="text-ink-3">未填写</span>}{hasLdap && <span className="ml-2 text-[length:var(--wb-text-xs)] text-ink-3">以 LDAP 为准</span>}</Dd>
              <Dt>部门</Dt>
              <Dd>{me.dept || <span className="text-ink-3">未填写</span>}{hasLdap && <span className="ml-2 text-[length:var(--wb-text-xs)] text-ink-3">以 LDAP 为准</span>}</Dd>
              <Dt>手机号</Dt>
              <Dd>{me.phone || <span className="text-ink-3">未填写</span>}</Dd>
              <Dt>Owner</Dt>
              <Dd>{owns.length ? owns.map((a) => a.name).join('、') : <span className="text-ink-3">无</span>}</Dd>
              <Dt>PE</Dt>
              <Dd>{pes.length ? pes.map((a) => a.name).join('、') : <span className="text-ink-3">无</span>}</Dd>
            </Dl>
          </section>

          <section>
            <h2 className="mb-1 border-b-2 border-ink pb-2 text-[length:var(--wb-text-md)] font-bold">登录身份</h2>
            <p className="mt-2 mb-3 text-[length:var(--wb-text-sm)] leading-relaxed text-ink-2">
              一个用户可以用多种登录身份登录。工具不会按邮箱自动合并用户：追加身份时要用那个身份完成一次认证。
            </p>
            <ul className="border-t border-line">
              {me.identities.map((i) => (
                <li key={i.type} className="flex items-center gap-4 border-b border-line-2 py-3">
                  <span className="w-28 font-semibold">{IDENTITY_LABEL[i.type]}</span>
                  <span className="flex-1">
                    {i.login}
                    <span className="block text-[length:var(--wb-text-xs)] text-ink-3">{i.boundAt} 绑定 · {i.boundBy}</span>
                  </span>
                  <Btn disabled={last} title={last ? '至少保留一个登录身份' : undefined} onClick={() => setUnbinding(i.type)}>解绑</Btn>
                </li>
              ))}
            </ul>
            {last && <p className="mt-2 text-[length:var(--wb-text-sm)] text-ink-2">这是你唯一的登录身份，不能解绑。先绑定另一种再解绑它。</p>}
            {unbound.length > 0 && (
              <div className="mt-4 flex gap-2">
                {unbound.map((t) => (
                  <Btn key={t} onClick={() => setBinding(t)}>绑定{IDENTITY_LABEL[t]}</Btn>
                ))}
              </div>
            )}
          </section>
        </div>
      </Page>
      {binding && <BindDialog user={me} type={binding} by="本人绑定" onClose={() => setBinding(null)} />}
      {unbinding && (
        <Dialog
          onClose={() => setUnbinding(null)}
          title={`解绑${IDENTITY_LABEL[unbinding]}`}
          description={`解绑后不能再用${IDENTITY_LABEL[unbinding]}登录。${unbinding === 'ldap' ? '显示名、邮箱、部门不再从 LDAP 同步。' : ''}`}
          footer={
            <>
              <Btn onClick={() => setUnbinding(null)}>取消</Btn>
              <Btn kind="solid-danger" onClick={() => (actions.unbindIdentity(me.username, unbinding), setUnbinding(null))}>解绑</Btn>
            </>
          }
        >
          <p>解绑后你仍可以用{me.identities.filter((i) => i.type !== unbinding).map((i) => IDENTITY_LABEL[i.type]).join('、')}登录。</p>
        </Dialog>
      )}
    </Shell>
  );
}

// ================================================================= 管理后台

function NoAccess({ what }: { what: string }) {
  return (
    <Shell>
      <Page>
        <PageTitle title={what} sub="只有管理员可以查看和管理这里。用提示条切换到管理员视角后再看。" actions={<Btn onClick={() => navigate('/apps')}>回到应用列表</Btn>} />
      </Page>
    </Shell>
  );
}

export function AdminPlatforms() {
  const s = useStore();
  const [adding, setAdding] = useState(false);
  if (!can(s.role).admin) return <NoAccess what="代码平台" />;
  return (
    <Shell>
      <Page>
        <PageTitle title="代码平台" sub="应用建立仓库关联时从这里选择平台。工具只用只读服务令牌访问仓库。" actions={<Btn kind="primary" onClick={() => setAdding(true)}>登记代码平台</Btn>} />
        <ul className="max-w-[960px] border-t-2 border-ink">
          {s.platforms.map((p) => {
            const apps = s.apps.filter((a) => a.repo.platformId === p.id);
            return (
              <li key={p.id} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)] gap-10 border-b border-line py-5">
                <div className="min-w-0">
                  <Mono className="block truncate text-[length:var(--wb-text-md)] font-semibold">{p.url}</Mono>
                  <div className="mt-1.5 text-[length:var(--wb-text-sm)] text-ink-2">
                    {p.type} · 只读服务令牌 <Mono>••••••••{p.tokenTail}</Mono> · {p.addedAt} 登记
                  </div>
                </div>
                <div>
                  <div className="mb-1.5 text-[length:var(--wb-text-sm)] font-semibold">
                    {apps.length ? `${apps.length} 个应用的仓库关联依赖这个平台` : '还没有应用关联到这个平台'}
                  </div>
                  <ul className="space-y-1">
                    {apps.map((a) => (
                      <li key={a.key} className="flex items-baseline gap-3">
                        <LinkBtn onClick={() => navigate(`/apps/${a.key}`)}>{a.name}</LinkBtn>
                        <Mono className="text-[length:var(--wb-text-sm)] text-ink-2">{a.repo.path}</Mono>
                        <span className="text-[length:var(--wb-text-xs)] text-ink-3">项目 ID {a.repo.projectId}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </li>
            );
          })}
        </ul>
      </Page>
      {adding && <PlatformDialog onClose={() => setAdding(false)} />}
    </Shell>
  );
}

export function AdminUsers() {
  const s = useStore();
  const [creating, setCreating] = useState(false);
  const [disabling, setDisabling] = useState<User | null>(null);
  const [binding, setBinding] = useState<{ user: User; type: IdentityType } | null>(null);
  if (!can(s.role).admin) return <NoAccess what="用户" />;
  const off = s.users.filter((u) => u.status === '已禁用').length;
  return (
    <Shell>
      <Page>
        <PageTitle
          title="用户"
          sub={`${s.users.length} 个用户，其中 ${off} 个已禁用。用户不能删除，只能禁用。LDAP 用户第一次登录后才会出现在这里。`}
          actions={<Btn kind="primary" onClick={() => setCreating(true)}>新建本地用户</Btn>}
        />
        <table className="wb-table">
          <thead>
            <tr>
              <th>用户</th>
              <th>邮箱 / 部门</th>
              <th>登录身份</th>
              <th>负责的应用</th>
              <th>状态</th>
              <th className="text-right">操作</th>
            </tr>
          </thead>
          <tbody>
            {s.users.map((u) => {
              const disabled = u.status === '已禁用';
              const unbound = (['ldap', 'local'] as const).filter((t) => !u.identities.some((i) => i.type === t));
              const owner = s.apps.filter((a) => a.owners.includes(u.username));
              const pe = s.apps.filter((a) => a.pes.includes(u.username));
              const sole = soleOwnerOf(s, u.username);
              return (
                <tr key={u.username} className={disabled ? 'text-ink-3' : ''}>
                  <td>
                    <div className="font-semibold">{u.displayName}</div>
                    <div className="text-[length:var(--wb-text-xs)] text-ink-3">{u.username}</div>
                  </td>
                  <td className="text-[length:var(--wb-text-sm)]">
                    <div>{u.email || '—'}</div>
                    <div className="text-ink-3">{u.dept ?? '—'}</div>
                  </td>
                  <td className="text-[length:var(--wb-text-sm)]">
                    {u.identities.map((i) => (
                      <div key={i.type}>
                        <span className="font-medium">{IDENTITY_LABEL[i.type]}</span> <span className="text-ink-3">{i.login}</span>
                      </div>
                    ))}
                  </td>
                  <td className="text-[length:var(--wb-text-sm)]">
                    {owner.length === 0 && pe.length === 0 && <span className="text-ink-3">—</span>}
                    {owner.length > 0 && <div>Owner：{owner.map((a) => a.name).join('、')}</div>}
                    {pe.length > 0 && <div>PE：{pe.map((a) => a.name).join('、')}</div>}
                    {sole.length > 0 && <div className="mt-0.5 font-semibold">唯一 Owner：{sole.map((a) => a.name).join('、')}</div>}
                  </td>
                  <td className={`whitespace-nowrap ${disabled ? 'font-semibold text-ink' : 'text-ink-3'}`}>{u.status}</td>
                  <td className="text-right whitespace-nowrap">
                    <span className="inline-flex items-center gap-3">
                      {unbound.map((t) => (
                        <LinkBtn key={t} className="text-[length:var(--wb-text-sm)]" onClick={() => setBinding({ user: u, type: t })}>绑定{IDENTITY_LABEL[t]}</LinkBtn>
                      ))}
                      {u.username === s.me ? (
                        <span className="text-[length:var(--wb-text-sm)] text-ink-3">当前用户</span>
                      ) : disabled ? (
                        <Btn onClick={() => actions.setUserStatus(u.username, '正常')}>启用</Btn>
                      ) : (
                        <Btn onClick={() => setDisabling(u)}>禁用</Btn>
                      )}
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </Page>
      {creating && <CreateUserDialog onClose={() => setCreating(false)} />}
      {disabling && <DisableUserDialog user={disabling} onClose={() => setDisabling(null)} />}
      {binding && <BindDialog user={binding.user} type={binding.type} by={`管理员 ${s.me}`} onClose={() => setBinding(null)} />}
    </Shell>
  );
}
