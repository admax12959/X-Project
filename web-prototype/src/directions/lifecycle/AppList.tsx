import { useState } from 'react';
import { appLines, appVersions, can, daysUntil, EXPIRY_WINDOW_DAYS, isExpiring, lineVersions, TODAY, useStore, userName, type Application, type Line } from '../../data';
import { navigate } from '../../router';
import { CreateAppDialog } from './dialogs';
import { Crumbs, PageHead, SectionHead, Shell } from './Shell';
import { addDays, daysBetween, lineSolidEnd, lineStart, months, nextMonth, span, type State } from './time';
import { Button, Code, Input } from './ui';

const openLine = (appKey: string, lineId?: string) => navigate(`/apps/${appKey}${lineId ? `?line=${encodeURIComponent(lineId)}` : ''}`);

export function AppList() {
  const s = useStore();
  const perm = can(s.role);
  const [mine, setMine] = useState(false);
  const [q, setQ] = useState('');
  const [creating, setCreating] = useState(false);
  const isMine = (a: Application) => a.owners.includes(s.me) || a.pes.includes(s.me);
  const scoped = s.apps.filter((a) => !mine || isMine(a));

  return (
    <Shell crumbs={<Crumbs items={[{ label: '应用' }]} />}>
      <PageHead
        title="维护线到期"
        sub={<>还在维护的维护线，按计划终止日期排列。横轴从今天开始按日历计算，灰色区是 {EXPIRY_WINDOW_DAYS} 天提醒窗口。</>}
        actions={
          <label className="flex h-9 cursor-pointer items-center gap-2 border border-[var(--lc-ink)] px-3 text-[length:var(--lc-text-sm)] font-semibold has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-[var(--lc-focus)]">
            <input type="checkbox" className="lc-check" checked={mine} onChange={(e) => setMine(e.target.checked)} />
            只看我负责的应用
          </label>
        }
      />
      <Runway s={s} apps={scoped} />

      <section className="mt-20" aria-labelledby="app-index">
        <SectionHead
          id="app-index"
          actions={
            <>
              <Input type="search" aria-label="搜索应用" value={q} onChange={(e) => setQ(e.target.value)} placeholder="名称或应用标识" className="!w-64 max-[1100px]:!w-44" />
              {perm.admin && (
                <Button kind="primary" onClick={() => setCreating(true)}>
                  创建应用
                </Button>
              )}
            </>
          }
        >
          应用 <span className="lc-num ml-1 font-normal text-[var(--lc-muted)]">{scoped.length}</span>
        </SectionHead>
        <AppIndex s={s} apps={scoped.filter((a) => !q.trim() || `${a.key} ${a.name}`.toLowerCase().includes(q.trim().toLowerCase()))} query={q} mine={mine} />
      </section>
      {creating && <CreateAppDialog onClose={() => setCreating(false)} />}
    </Shell>
  );
}

// ---------- the focal object: remaining life of every maintained line, on a calendar from today ----------

function Runway({ s, apps }: { s: State; apps: Application[] }) {
  const keys = new Set(apps.map((a) => a.key));
  const live = s.lines.filter((l) => keys.has(l.appKey) && !l.isMain && l.status !== '终止');
  const dated = live.filter((l) => l.plannedEnd).sort((a, b) => a.plannedEnd!.localeCompare(b.plannedEnd!));
  const undated = live.filter((l) => !l.plannedEnd);
  const appOf = (l: Line) => s.apps.find((a) => a.key === l.appKey)!;

  if (!live.length)
    return <p className="border-y-2 border-[var(--lc-ink)] py-6 text-[var(--lc-ink-2)]">{apps.length ? '这些应用没有在维护的维护线，只有主线。' : '你还没有作为 Owner 或 PE 负责的应用。'}</p>;

  const firstEnd = dated[0]?.plannedEnd ?? TODAY;
  const from = firstEnd < TODAY ? addDays(firstEnd, -7) : TODAY;
  const to = nextMonth(addDays(dated.at(-1)?.plannedEnd ?? addDays(TODAY, 90), 20));
  const total = daysBetween(from, to);
  const pct = (d: string) => `${(daysBetween(from, d) / total) * 100}%`;
  const ticks = months(nextMonth(from), addDays(to, -1));
  const windowEnd = addDays(TODAY, EXPIRY_WINDOW_DAYS);
  const soon = dated.filter(isExpiring).length;

  return (
    <figure className="m-0">
      <div className="overflow-x-auto">
        <div className="min-w-[560px]">
          {/* axis row */}
          <div className="grid grid-cols-[var(--lc-runway-cols)] border-b-2 border-[var(--lc-ink)] [--lc-runway-cols:280px_minmax(0,1fr)_128px] max-[1100px]:[--lc-runway-cols:200px_minmax(0,1fr)_104px]">
            <div className="flex items-end pb-2">
              <span className="lc-label">维护线</span>
            </div>
            <div className="relative h-11">
              {ticks.map((m) => (
                <span key={m} className="absolute bottom-2 pl-1.5 whitespace-nowrap" style={{ left: pct(m) }}>
                  {m.endsWith('-01-01') ? <span className="lc-num text-[length:var(--lc-text-sm)] font-bold">{m.slice(0, 4)}</span> : <span className={`lc-num text-[length:var(--lc-text-xs)] text-[var(--lc-muted)] ${Number(m.slice(5, 7)) % 3 === 1 ? '' : 'max-[1100px]:hidden'}`}>{Number(m.slice(5, 7))}月</span>}
                </span>
              ))}
              <span className="absolute top-1 -translate-x-1/2 bg-[var(--lc-paper)] px-1 text-[length:var(--lc-text-xs)] font-bold whitespace-nowrap text-[var(--lc-signal)]" style={{ left: pct(TODAY) }}>
                今天
              </span>
              <span className="absolute top-1 pl-1.5 text-[length:var(--lc-text-xs)] font-semibold whitespace-nowrap text-[var(--lc-ink-2)]" style={{ left: pct(windowEnd) }}>
                +{EXPIRY_WINDOW_DAYS} 天
              </span>
            </div>
            <div className="flex items-end justify-end pb-2">
              <span className="lc-label">剩余</span>
            </div>
          </div>

          {/* rows */}
          <div className="relative">
            {/* ground: 30-day window, month grid, today */}
            <div aria-hidden className="pointer-events-none absolute inset-y-0 grid w-full grid-cols-[var(--lc-runway-cols)] [--lc-runway-cols:280px_minmax(0,1fr)_128px] max-[1100px]:[--lc-runway-cols:200px_minmax(0,1fr)_104px]">
              <div />
              <div className="relative">
                <div className="lc-window absolute inset-y-0" style={{ left: pct(TODAY), width: `calc(${pct(windowEnd)} - ${pct(TODAY)})` }} />
                {ticks.map((m) => (
                  <div key={m} className={`absolute inset-y-0 w-px ${m.endsWith('-01-01') ? 'bg-[var(--lc-rule-2)]' : 'bg-[var(--lc-rule)]'}`} style={{ left: pct(m) }} />
                ))}
                <div className="absolute inset-y-0 z-[3] w-0.5 -translate-x-1/2 bg-[var(--lc-signal)]" style={{ left: pct(TODAY) }} />
              </div>
            </div>

            <ol>
              {dated.map((l) => {
                const app = appOf(l);
                const d = daysUntil(l.plannedEnd!);
                const exp = isExpiring(l);
                const late = d < 0;
                return (
                  <li key={l.id}>
                    <button
                      type="button"
                      onClick={() => openLine(app.key, l.id)}
                      aria-label={`${app.name}「${l.name}」，计划终止日期 ${l.plannedEnd}，${late ? `已过期 ${-d} 天` : `剩余 ${d} 天`}`}
                      className="lc-row relative grid h-[68px] w-full grid-cols-[var(--lc-runway-cols)] items-center border-b border-[var(--lc-rule)] text-left [--lc-runway-cols:280px_minmax(0,1fr)_128px] max-[1100px]:[--lc-runway-cols:200px_minmax(0,1fr)_104px]"
                    >
                      <span className="min-w-0 pr-4">
                        <span className="block truncate text-[length:var(--lc-text-base)] leading-tight font-bold">{l.name}</span>
                        <span className="mt-1 block truncate text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">
                          {app.name} · <Code>{l.branch}</Code>
                          {l.status === '冻结' && ' · 冻结'}
                        </span>
                      </span>
                      <span className="relative block h-full">
                        <span
                          className={`absolute top-1/2 z-[2] h-3 -translate-y-1/2 ${exp ? 'lc-hatch--signal' : l.status === '冻结' ? 'lc-bar-frozen' : 'lc-hatch'}`}
                          style={late ? { left: pct(l.plannedEnd!), width: `calc(${pct(TODAY)} - ${pct(l.plannedEnd!)})` } : { left: pct(TODAY), width: `calc(${pct(l.plannedEnd!)} - ${pct(TODAY)})` }}
                        />
                        <span className={`absolute top-1/2 z-[2] h-6 w-[3px] -translate-x-1/2 -translate-y-1/2 ${exp ? 'bg-[var(--lc-signal)]' : 'bg-[var(--lc-ink)]'}`} style={{ left: pct(l.plannedEnd!) }} />
                        <span className={`lc-num absolute top-1/2 z-[2] -translate-y-1/2 pl-2.5 text-[length:var(--lc-text-sm)] whitespace-nowrap ${exp ? 'font-bold text-[var(--lc-signal)]' : 'text-[var(--lc-ink-2)]'}`} style={{ left: pct(late ? TODAY : l.plannedEnd!) }}>
                          {l.plannedEnd}
                        </span>
                      </span>
                      <span className="flex items-baseline justify-end gap-1 pl-3">
                        {late ? (
                          <span className="lc-signal-text text-[length:var(--lc-text-md)]">已过期 {-d} 天</span>
                        ) : (
                          <>
                            <span className={`lc-num leading-none ${exp ? 'text-[length:var(--lc-text-numeral)] font-extrabold tracking-tight text-[var(--lc-signal)]' : 'text-[length:var(--lc-text-md)] font-bold'}`}>{d}</span>
                            <span className={exp ? 'font-bold text-[var(--lc-signal)]' : 'text-[var(--lc-ink-2)]'}>天</span>
                          </>
                        )}
                      </span>
                    </button>
                  </li>
                );
              })}
            </ol>
          </div>
        </div>
      </div>
      <figcaption className="mt-3 flex flex-wrap gap-x-6 gap-y-1 text-[length:var(--lc-text-sm)] text-[var(--lc-ink-2)]">
        <span>{soon ? <span className="lc-signal-text">即将到期的维护线：{soon} 条在 {EXPIRY_WINDOW_DAYS} 天内。</span> : `${EXPIRY_WINDOW_DAYS} 天内没有到期的维护线。`}到了计划终止日期也不会自动终止，状态由 Owner 手动改。</span>
        {undated.length > 0 && (
          <span>
            未定计划终止日期：
            {undated.map((l, i) => (
              <span key={l.id}>
                {i > 0 && '、'}
                <a href={`/apps/${l.appKey}`} onClick={(e) => (e.preventDefault(), openLine(l.appKey, l.id))} className="lc-link">
                  {appOf(l).name}「{l.name}」
                </a>
              </span>
            ))}
          </span>
        )}
      </figcaption>
    </figure>
  );
}

// ---------- app index: every app with its own lines drawn on one shared calendar ----------

function AppIndex({ s, apps, query, mine }: { s: State; apps: Application[]; query: string; mine: boolean }) {
  if (!apps.length)
    return <p className="py-6 text-[var(--lc-ink-2)]">{query.trim() ? `没有名称或应用标识包含「${query.trim()}」的应用。` : mine ? '你还没有作为 Owner 或 PE 负责的应用。' : '还没有应用。管理员可以创建。'}</p>;

  const allDates = s.apps.flatMap((a) => [...appVersions(s, a.key).map((v) => v.releasedAt), ...appLines(s, a.key).flatMap((l) => (l.plannedEnd ? [l.plannedEnd] : []))]);
  const from = `${[TODAY, ...allDates].sort()[0].slice(0, 4)}-01-01`;
  const to = `${Number([TODAY, ...allDates].sort().at(-1)!.slice(0, 4)) + 1}-01-01`;
  const total = daysBetween(from, to);
  const pct = (d: string) => `${Math.max(0, Math.min(1, daysBetween(from, d) / total)) * 100}%`;
  const years: string[] = [];
  for (let y = Number(from.slice(0, 4)); y < Number(to.slice(0, 4)); y++) years.push(`${y}-01-01`);
  const cols = '[--lc-idx-cols:minmax(0,3fr)_minmax(0,4fr)_minmax(0,2fr)_120px] max-[1100px]:[--lc-idx-cols:minmax(0,3fr)_minmax(0,3fr)_minmax(0,2fr)]';

  return (
    <div>
      <div className={`grid grid-cols-[var(--lc-idx-cols)] gap-x-6 border-b border-[var(--lc-ink)] pb-2 ${cols}`}>
        <span className="lc-label">应用</span>
        <span className="relative h-4">
          {years.map((y) => (
            <span key={y} className="lc-num lc-label absolute pl-1" style={{ left: pct(y) }}>
              {y.slice(0, 4)}
            </span>
          ))}
        </span>
        <span className="lc-label">Owner / PE</span>
        <span className="lc-label text-right max-[1100px]:hidden">最新主线版本</span>
      </div>
      <ul>
        {apps.map((a) => {
          const ls = appLines(s, a.key);
          const main = ls.find((l) => l.isMain);
          const latest = main ? lineVersions(s, main.id).at(-1) : undefined;
          const live = ls.filter((l) => !l.isMain && l.status !== '终止');
          const soon = live.filter(isExpiring);
          const retired = ls.filter((l) => !l.isMain && l.status === '终止').length;
          const lineSummary = live.length ? `${live.length} 条在维护` : retired ? `${retired} 条已终止` : '只有主线';
          return (
            <li key={a.key}>
              <button type="button" onClick={() => openLine(a.key)} className={`lc-row grid w-full grid-cols-[var(--lc-idx-cols)] items-center gap-x-6 border-b border-[var(--lc-rule)] py-4 text-left ${cols}`}>
                <span className="min-w-0">
                  <span className="block truncate text-[length:var(--lc-text-md)] leading-tight font-bold">{a.name}</span>
                  <span className="mt-1 block truncate text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">
                    <Code className="text-[var(--lc-ink-2)]">{a.key}</Code> · {lineSummary}
                    {soon.length > 0 && <span className="lc-signal-text">，{soon.length} 条将到期</span>}
                  </span>
                </span>
                <Strip s={s} lines={ls} pct={pct} years={years} />
                <span className="min-w-0 text-[length:var(--lc-text-sm)]">
                  <span className="block truncate">{a.owners.map(userName).join('、')}</span>
                  <span className="block truncate text-[var(--lc-muted)]">{a.pes.length ? `PE ${a.pes.map(userName).join('、')}` : '未设置 PE'}</span>
                </span>
                <span className="text-right max-[1100px]:hidden">
                  {latest ? <Code className="text-[length:var(--lc-text-base)] font-bold">{latest.number}</Code> : <span className="lc-muted">无版本</span>}
                  {latest && <span className="lc-num block text-[length:var(--lc-text-sm)] text-[var(--lc-muted)]">{span(latest.releasedAt, TODAY)}前</span>}
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/** One app's lines as thin bars on the shared index calendar. */
function Strip({ s, lines, pct, years }: { s: State; lines: Line[]; pct: (d: string) => string; years: string[] }) {
  const order = [...lines].sort((a, b) => Number(b.isMain) - Number(a.isMain) || lineStart(s, a).localeCompare(lineStart(s, b)));
  return (
    <span aria-hidden className="relative block py-1" style={{ height: order.length * 6 + 8 }}>
      {years.map((y) => (
        <span key={y} className="absolute inset-y-0 w-px bg-[var(--lc-rule)]" style={{ left: pct(y) }} />
      ))}
      {order.map((l, i) => {
        const st = lineStart(s, l);
        const end = lineSolidEnd(s, l);
        const retired = l.status === '终止';
        const top = 4 + i * 6;
        const exp = isExpiring(l);
        return (
          <span key={l.id}>
            <span className={`absolute h-[3px] ${retired ? 'bg-[var(--lc-rule-2)]' : l.status === '冻结' ? 'bg-[var(--lc-muted)]' : 'bg-[var(--lc-ink)]'}`} style={{ top, left: pct(st), width: `max(2px, calc(${pct(end)} - ${pct(st)}))` }} />
            {!retired && l.plannedEnd && l.plannedEnd > TODAY && (
              <span className={`absolute h-px ${exp ? 'bg-[var(--lc-signal)]' : 'bg-[var(--lc-ink-2)]'}`} style={{ top: top + 1, left: pct(TODAY), width: `calc(${pct(l.plannedEnd)} - ${pct(TODAY)})` }} />
            )}
            {!retired && l.plannedEnd && <span className={`absolute h-[7px] w-0.5 ${exp ? 'bg-[var(--lc-signal)]' : 'bg-[var(--lc-ink-2)]'}`} style={{ top: top - 2, left: pct(l.plannedEnd) }} />}
          </span>
        );
      })}
      <span className="absolute inset-y-0 w-0.5 bg-[var(--lc-signal)]" style={{ left: pct(TODAY) }} />
    </span>
  );
}
