import { useState, type ReactNode } from 'react';
import { appLines, can, daysUntil, EXPIRY_WINDOW_DAYS, isExpiring, lineById, lineVersions, useStore, userName, type Application, type Line } from '../../data';
import { navigate } from '../../router';
import { CreateAppDialog } from './appforms';
import { Shell } from './Shell';
import { Button, LinkButton, Mono } from './ui';

type Ev = { at: string; app: Application; kind: 'version' | 'fork' | 'status'; line?: Line; versionId?: string; body: ReactNode; actor?: string; target?: string };

const open = (key: string, q = '') => navigate(`/apps/${key}${q}`);

export function AppList() {
  const s = useStore();
  const perm = can(s.role);
  const [q, setQ] = useState('');
  const [mineOnly, setMineOnly] = useState(false);
  const [feedN, setFeedN] = useState(12);
  const [creating, setCreating] = useState(false);
  const isMine = (a: Application) => a.owners.includes(s.me) || a.pes.includes(s.me);
  const myApps = s.apps.filter(isMine);

  const expiring = s.lines
    .filter(isExpiring)
    .map((line) => ({ line, app: s.apps.find((a) => a.key === line.appKey)! }))
    .filter(({ app }) => app && (perm.admin || isMine(app)))
    .sort((a, b) => daysUntil(a.line.plannedEnd!) - daysUntil(b.line.plannedEnd!));

  // the feed: versions are the record; audit adds forks and status changes (registration audits are the same event as the version)
  const events: Ev[] = [];
  for (const app of myApps) {
    const audit = s.audit.filter((e) => e.appKey === app.key);
    for (const line of appLines(s, app.key))
      for (const v of lineVersions(s, line.id)) {
        const by = audit.find((e) => e.text.endsWith(`登记版本 ${v.number}`));
        events.push({
          at: v.releasedAt, app, kind: 'version', line, versionId: v.id, actor: by?.actor,
          body: (
            <>
              <Mono className="rl-t-md font-medium rl-ink">{v.number}</Mono>
              <span className="ml-2 rl-ink-2">登记在{line.isMain ? '主线' : `「${line.name}」`}</span>
              {v.note && <span className="ml-2 rl-ink-3">{v.note}</span>}
            </>
          ),
        });
      }
    for (const e of audit) {
      const fork = e.text.includes('分叉维护线');
      const status = e.text.includes('状态改为');
      if (!fork && !status) continue;
      const name = e.text.match(/「(.+?)」/)?.[1];
      const line = appLines(s, app.key).find((l) => l.name === name);
      events.push({ at: e.at, app, kind: fork ? 'fork' : 'status', line, actor: e.actor, target: e.text.match(/状态改为(\S+)$/)?.[1], body: <span className="font-medium rl-ink">{e.text}</span> });
    }
  }
  events.sort((a, b) => b.at.localeCompare(a.at));

  const apps = s.apps.filter((a) => (!mineOnly || isMine(a)) && (q.trim() === '' || `${a.key} ${a.name} ${a.description}`.toLowerCase().includes(q.trim().toLowerCase())));

  return (
    <Shell crumbs={[{ label: '应用' }]}>
      {/* focal: what is about to run out */}
      <section aria-labelledby="due-h" className="px-8 pt-8 pb-10">
        <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
          <h1 id="due-h" className="rl-t-xl">即将到期的维护线</h1>
          <span className="rl-ink-2">{EXPIRY_WINDOW_DAYS} 天内到达计划终止日期、尚未终止{perm.admin ? '（全部应用）' : '（我是 Owner / PE 的应用）'}</span>
        </div>
        {expiring.length === 0 ? (
          <p className="mt-6 rl-t-md rl-ink-2">{EXPIRY_WINDOW_DAYS} 天内没有到期的维护线。</p>
        ) : (
          <ol className="mt-5" style={{ borderTop: '1px solid var(--rl-ink)', borderBottom: '1px solid var(--rl-rule)' }}>
            {expiring.map(({ line, app }, i) => {
              const d = daysUntil(line.plannedEnd!);
              const last = lineVersions(s, line.id).at(-1);
              return (
                <li key={line.id} style={i ? { borderTop: '1px solid var(--rl-rule)' } : undefined}>
                  <button className="rl-row gap-6 py-4 pr-4" style={{ gridTemplateColumns: '132px minmax(0,1.3fr) minmax(0,1fr) minmax(0,1fr)' }} onClick={() => open(app.key, `?line=${encodeURIComponent(line.id)}`)}>
                    <span className="flex items-baseline gap-2 pl-1">
                      {d < 0 ? (
                        <><span className="rl-t-2xl">{-d}</span><span className="font-semibold">天前已到期</span></>
                      ) : d === 0 ? (
                        <span className="rl-t-xl">今天到期</span>
                      ) : (
                        <><span className="rl-t-2xl">{d}</span><span className="font-semibold">天后</span></>
                      )}
                    </span>
                    <span className="min-w-0">
                      <span className="block truncate rl-t-md font-semibold">{line.name}</span>
                      <span className="block truncate rl-ink-2">{app.name} · <span className={line.status === '活跃' ? 'rl-green font-medium' : 'font-medium'}>{line.status}</span></span>
                    </span>
                    <span className="min-w-0">
                      <span className="block">计划终止 {line.plannedEnd}</span>
                      <Mono className="block truncate rl-ink-2">{line.branch}</Mono>
                    </span>
                    <span className="min-w-0">
                      <span className="block truncate">{last ? <>最近版本 <Mono>{last.number}</Mono></> : '还没有登记版本'}</span>
                      <span className="block truncate rl-ink-2">{last ? last.releasedAt : ''}</span>
                    </span>
                  </button>
                </li>
              );
            })}
          </ol>
        )}
      </section>

      <div className="grid grid-cols-1 gap-x-12 gap-y-12 px-8 xl:grid-cols-[minmax(0,7fr)_minmax(0,5fr)]">
        {/* the ledger of recent events */}
        <section aria-labelledby="feed-h">
          <div className="flex items-baseline gap-3 pb-3">
            <h2 id="feed-h" className="rl-t-lg">最近动态</h2>
            <span className="rl-ink-2">我是 Owner / PE 的 {myApps.length} 个应用</span>
          </div>
          {events.length === 0 ? (
            <p className="py-4 rl-ink-2" style={{ borderTop: '1px solid var(--rl-ink)' }}>你还不是任何应用的 Owner 或 PE，这里没有动态。</p>
          ) : (
            <ol className="rl-rows" style={{ borderTop: '1px solid var(--rl-ink)' }}>
              {events.slice(0, feedN).map((e, i) => (
                <li key={i}>
                  <button
                    className="rl-row gap-3 py-3 pr-3"
                    style={{ gridTemplateColumns: '108px 20px minmax(0,1fr) auto' }}
                    onClick={() => open(e.app.key, e.versionId ? `?ver=${e.versionId}` : e.line ? `?line=${encodeURIComponent(e.line.id)}` : '')}
                  >
                    <span className="pl-1 rl-ink-2">{e.at.slice(0, 10)}</span>
                    <Glyph kind={e.kind} status={e.kind === 'status' ? (e.target as Line['status'] | undefined) : e.line ? lineById(e.line.id)?.status : undefined} />
                    <span className="min-w-0 truncate">{e.body}</span>
                    <span className="truncate text-right rl-ink-2">
                      {e.app.name}
                      {e.actor && <span className="rl-ink-3"> · {userName(e.actor)}</span>}
                    </span>
                  </button>
                </li>
              ))}
            </ol>
          )}
          {events.length > feedN && (
            <LinkButton className="mt-3" onClick={() => setFeedN(feedN + 12)}>
              显示更早的 {Math.min(12, events.length - feedN)} 条（共 {events.length} 条）
            </LinkButton>
          )}
        </section>

        {/* index */}
        <section aria-labelledby="index-h">
          <div className="flex items-baseline justify-between gap-3 pb-3">
            <h2 id="index-h" className="rl-t-lg">全部应用</h2>
            {perm.admin && <Button kind="primary" onClick={() => setCreating(true)}>创建应用</Button>}
          </div>
          <div className="flex flex-wrap items-center gap-x-4 gap-y-2 py-3" style={{ borderTop: '1px solid var(--rl-ink)' }}>
            <label className="sr-only" htmlFor="app-q">搜索应用</label>
            <input id="app-q" className="rl-input" style={{ width: 240 }} value={q} onChange={(e) => setQ(e.target.value)} placeholder="名称或应用标识" type="search" />
            <label className="flex cursor-pointer items-center gap-2">
              <input type="checkbox" className="rl-check" checked={mineOnly} onChange={(e) => setMineOnly(e.target.checked)} />
              只看我负责的应用
            </label>
            <span className="ml-auto rl-ink-3">{apps.length} / {s.apps.length}</span>
          </div>
          {apps.length === 0 ? (
            <p className="py-4 rl-ink-2" style={{ borderTop: '1px solid var(--rl-rule)' }}>
              没有匹配「{q || '我负责的'}」的应用。{(q || mineOnly) && <LinkButton onClick={() => (setQ(''), setMineOnly(false))}>清除筛选</LinkButton>}
            </p>
          ) : (
            <ul className="rl-rows" style={{ borderTop: '1px solid var(--rl-rule)', borderBottom: '1px solid var(--rl-rule)' }}>
              {apps.map((a) => {
                const ls = appLines(s, a.key);
                const main = ls.find((l) => l.isMain);
                const maint = ls.filter((l) => !l.isMain);
                const live = maint.filter((l) => l.status !== '终止').length;
                const latest = main ? lineVersions(s, main.id).at(-1) : undefined;
                const role = a.owners.includes(s.me) ? 'Owner' : a.pes.includes(s.me) ? 'PE' : null;
                return (
                  <li key={a.key}>
                    <button className="rl-row gap-x-4 py-3 pr-2" style={{ gridTemplateColumns: 'minmax(0,1fr) auto' }} onClick={() => open(a.key)}>
                      <span className="min-w-0 pl-1">
                        <span className="flex items-baseline gap-2">
                          <span className="truncate rl-t-md font-semibold">{a.name}</span>
                          <Mono className="truncate rl-ink-2">{a.key}</Mono>
                        </span>
                        <span className="block truncate rl-ink-2">
                          Owner {a.owners.map(userName).join('、')}
                          {a.pes.length > 0 && <> · PE {a.pes.map(userName).join('、')}</>}
                        </span>
                      </span>
                      <span className="text-right">
                        <span className="block">{latest ? <Mono className="font-medium">{latest.number}</Mono> : <span className="rl-ink-3">无版本</span>}{role && <span className="ml-2 rl-stamp rl-ink-2">{role}</span>}</span>
                        <span className="block rl-ink-2">{maint.length === 0 ? '只有主线' : `${live} 条维护线在维护${maint.length > live ? `，${maint.length - live} 条已终止` : ''}`}</span>
                      </span>
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </section>
      </div>
      {creating && <CreateAppDialog onClose={() => setCreating(false)} />}
    </Shell>
  );
}

function Glyph({ kind, status }: { kind: Ev['kind']; status?: Line['status'] }) {
  return (
    <svg width="20" height="20" aria-hidden className="shrink-0">
      {kind === 'version' && <circle cx="10" cy="10" r={status === '终止' ? 4 : 5} className={status === '活跃' ? 'rl-g-node--live' : status === '冻结' ? 'rl-g-node--frozen' : 'rl-g-node--retired'} />}
      {kind === 'fork' && <path className="rl-g-rail" d="M5,17 V13 Q5,9 9,9 H11 Q15,9 15,5 V2" />}
      {kind === 'status' && status === '冻结' && (
        <>
          <path className="rl-g-rail rl-g-rail--frozen" d="M8,2 V18" />
          <path className="rl-g-rail rl-g-rail--frozen" d="M12,2 V18" />
        </>
      )}
      {kind === 'status' && status !== '冻结' && (
        <>
          <path className="rl-g-rail rl-g-rail--retired" d="M10,18 V9" />
          <path className="rl-g-cap" d="M4,8 H16" />
        </>
      )}
    </svg>
  );
}
