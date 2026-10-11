import { useMemo, useState } from 'react';
import { GitFork, Plus } from 'lucide-react';
import { appLines, can, childLines, lineById, lineVersions, useStore, versionById, type Application, type Line, type Version } from '../../data';
import { useT } from '../../i18n';
import { setParam, useLocation } from '../../router';
import type { Ctx } from './detail';
import { StatusDialog, versionHref } from './dialogs';
import { A, Btn, Empty, Mono, Panel, Status } from './ui';

// Geometry. Rows have fixed heights so one SVG can draw the whole graph.
const HEAD_H = 56;
const TICK_H = 28;
const X0 = 16;
const STEP = 24;
const GUTTER = 12;
const GREY_MAIN = '#6b6862';
const GREY = '#a9a49d';
const LEADER = '#d9d6d0';
const ACCENT = '#1f6f8b';
const xOf = (depth: number) => X0 + depth * STEP;

type Row = { kind: 'head'; line: Line; depth: number; y: number } | { kind: 'tick'; line: Line; depth: number; ver: Version; y: number; fork: boolean };

function build(s: ReturnType<typeof useStore>, appKey: string) {
  const rows: Row[] = [];
  const headY: Record<string, number> = {};
  const lastY: Record<string, number> = {};
  const tickY: Record<string, number> = {};
  const depthOf: Record<string, number> = {};
  let y = 0;
  const main = appLines(s, appKey).find((l) => l.isMain);
  const walk = (line: Line, depth: number) => {
    depthOf[line.id] = depth;
    rows.push({ kind: 'head', line, depth, y });
    headY[line.id] = y + HEAD_H / 2;
    lastY[line.id] = headY[line.id];
    y += HEAD_H;
    for (const ver of lineVersions(s, line.id)) {
      const kids = childLines(s, ver.id);
      rows.push({ kind: 'tick', line, depth, ver, y, fork: kids.length > 0 });
      tickY[ver.id] = y + TICK_H / 2;
      lastY[line.id] = tickY[ver.id];
      y += TICK_H;
      kids.forEach((c) => walk(c, depth + 1));
    }
  };
  if (main) walk(main, 0);
  const maxDepth = Math.max(0, ...Object.values(depthOf));
  return { rows, headY, lastY, tickY, depthOf, height: y, maxDepth };
}

export function Lineage({ app, ctx }: { app: Application; ctx: Ctx }) {
  const { t } = useT();
  const s = useStore();
  const { params } = useLocation();
  const model = useMemo(() => build(s, app.key), [s, app.key]);
  const selId = params.get('line');
  const sel = selId ? lineById(selId) : undefined;
  const selected = sel && sel.appKey === app.key ? sel : undefined;
  const select = (id: string) => setParam('line', id === selected?.id ? null : id);

  const { rows, headY, lastY, tickY, depthOf, height, maxDepth } = model;
  const G = X0 + maxDepth * STEP + 36;

  // Path back to the mainline for the selected line: only this is drawn in the accent colour.
  const chain: Line[] = [];
  for (let l: Line | undefined = selected; l; ) {
    chain.push(l);
    const src = l.sourceVersionId ? versionById(l.sourceVersionId) : undefined;
    l = src ? lineById(src.lineId) : undefined;
  }
  const accentRange: Record<string, [number, number]> = {};
  chain.forEach((l, i) => {
    const below = chain[i - 1];
    accentRange[l.id] = [headY[l.id], below ? tickY[below.sourceVersionId!] : lastY[l.id]];
  });
  const onPath = new Set(chain.map((l) => l.id));
  const accentTick = (l: Line, y: number) => !!accentRange[l.id] && y >= accentRange[l.id][0] && y <= accentRange[l.id][1];

  const elbow = (c: Line) => {
    const src = versionById(c.sourceVersionId!)!;
    const pd = depthOf[src.lineId];
    const cd = depthOf[c.id];
    return `M${xOf(pd)} ${tickY[src.id]} H${xOf(pd) + GUTTER} V${headY[c.id]} H${xOf(cd)}`;
  };
  const lines = Object.keys(depthOf).map((id) => lineById(id)!);

  return (
    <div className="cg-lin-wrap" data-cg="lineage">
      <Panel title={t('term.lineage')} bodyClass={null} actions={<span className="cg-meta">{t('catalog.lineageLegend')}</span>}>
        <div className="cg-lin" style={{ height }} data-cg="lineage-graph">
          <svg width={G} height={height} aria-hidden="true">
            {/* leaders from node to the text column */}
            {rows.map((r) => (
              <line key={`ld-${r.kind}-${r.kind === 'tick' ? r.ver.id : r.line.id}`} x1={xOf(r.depth) + 8} x2={G - 8} y1={r.kind === 'tick' ? r.y + TICK_H / 2 : headY[r.line.id]} y2={r.kind === 'tick' ? r.y + TICK_H / 2 : headY[r.line.id]} stroke={LEADER} strokeWidth={1} strokeDasharray="1 3" />
            ))}
            {/* neutral spines and elbows */}
            {lines.map((l) => (
              <g key={`g-${l.id}`}>
                <line x1={xOf(depthOf[l.id])} x2={xOf(depthOf[l.id])} y1={headY[l.id]} y2={lastY[l.id]} stroke={l.isMain ? GREY_MAIN : GREY} strokeWidth={2} />
                {!l.isMain && <path d={elbow(l)} fill="none" stroke={GREY} strokeWidth={2} strokeLinejoin="miter" />}
              </g>
            ))}
            {/* accent: selected line and its path back to the mainline */}
            {chain.map((l) => (
              <g key={`a-${l.id}`}>
                <line x1={xOf(depthOf[l.id])} x2={xOf(depthOf[l.id])} y1={accentRange[l.id][0]} y2={accentRange[l.id][1]} stroke={ACCENT} strokeWidth={2} />
                {!l.isMain && <path d={elbow(l)} fill="none" stroke={ACCENT} strokeWidth={2} strokeLinejoin="miter" />}
              </g>
            ))}
            {/* nodes */}
            {rows.map((r) => {
              const x = xOf(r.depth);
              if (r.kind === 'head') {
                const acc = onPath.has(r.line.id);
                return <circle key={`n-${r.line.id}`} cx={x} cy={headY[r.line.id]} r={5} fill="#fff" stroke={acc ? ACCENT : r.line.isMain ? GREY_MAIN : GREY} strokeWidth={2} />;
              }
              const cy = r.y + TICK_H / 2;
              const acc = accentTick(r.line, cy);
              const col = acc ? ACCENT : r.line.isMain ? GREY_MAIN : GREY;
              return r.fork ? <circle key={`n-${r.ver.id}`} cx={x} cy={cy} r={4} fill={col} /> : <line key={`n-${r.ver.id}`} x1={x - 5} x2={x + 5} y1={cy} y2={cy} stroke={col} strokeWidth={2} />;
            })}
          </svg>
          {rows.map((r) => {
            if (r.kind === 'tick') {
              return (
                <div key={r.ver.id} className="cg-lin-row cg-lin-tick" style={{ paddingLeft: G }} data-cg="tick" data-sel={selected?.id === r.line.id ? 1 : undefined}>
                  <A to={versionHref(app.key, r.ver.id)} title={`${t(`ref.${r.ver.ref.kind}`)} ${r.ver.ref.value}`}>
                    {r.ver.number}
                  </A>
                  <VersionDate iso={r.ver.releasedAt} />
                </div>
              );
            }
            return <HeadRow key={r.line.id} app={app} line={r.line} G={G} selected={selected?.id === r.line.id} onSelect={() => select(r.line.id)} />;
          })}
        </div>
      </Panel>

      <div className="cg-sticky" style={{ minWidth: 0 }}>
        {selected ? <LineDetail app={app} line={selected} ctx={ctx} onSelectLine={select} /> : <Hint />}
      </div>
    </div>
  );
}

function VersionDate({ iso }: { iso: string }) {
  const { fmtDate } = useT();
  return <span className="d">{fmtDate(iso)}</span>;
}

function HeadRow({ app, line, G, selected, onSelect }: { app: Application; line: Line; G: number; selected: boolean; onSelect: () => void }) {
  const { t, tx } = useT();
  const s = useStore();
  const vs = lineVersions(s, line.id);
  const latest = vs.at(-1);
  const src = line.sourceVersionId ? versionById(line.sourceVersionId) : undefined;
  const parts: string[] = [];
  if (src) parts.push(`${t('term.sourceVersion')} ${src.number}`);
  if (latest) parts.push(`${t('catalog.latestShort')} ${latest.number}`);
  parts.push(vs.length ? t('n.versions', { n: vs.length }) : t('catalog.noVersionsYet'));
  const reason = line.reason ? tx(line.reason) : '';
  if (reason) parts.push(`${t('term.forkReason')}: ${reason}`);
  const branch = line.isMain ? app.mainBranch : line.branch;
  return (
    <button type="button" className="cg-lin-head" style={{ paddingLeft: G }} aria-pressed={selected} onClick={onSelect} data-cg="line-row" data-line={line.id}>
      <div className="l1">
        <b>{line.isMain ? t('term.mainline') : tx(line.name)}</b>
        <Status status={line.status} />
        <span className="cg-branch" title={branch}>
          {branch}
        </span>
      </div>
      <div className="l2" title={parts.join(' · ')}>
        {parts.join(' · ')}
      </div>
    </button>
  );
}

function Hint() {
  const { t } = useT();
  return (
    <Panel bodyClass={null} data-cg="line-hint">
      <Empty title={t('catalog.selectLine')}>
        <p>{t('catalog.selectLineBody')}</p>
      </Empty>
    </Panel>
  );
}

function LineDetail({ app, line, ctx, onSelectLine }: { app: Application; line: Line; ctx: Ctx; onSelectLine: (id: string) => void }) {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const perms = can(s.role);
  const [statusTo, setStatusTo] = useState<'Frozen' | 'Retired' | null>(null);
  const vs = lineVersions(s, line.id).slice().reverse();
  const src = line.sourceVersionId ? versionById(line.sourceVersionId) : undefined;
  const srcLine = src ? lineById(src.lineId) : undefined;
  const retired = line.status === 'Retired';
  const forkFrom = vs[0] ?? src ?? s.versions.find((v) => lineById(v.lineId)?.appKey === app.key);
  const next = line.status === 'Active' ? 'Frozen' : line.status === 'Frozen' ? 'Retired' : null;
  const branch = line.isMain ? app.mainBranch : line.branch;
  const title = line.isMain ? t('term.mainline') : tx(line.name);

  return (
    <Panel
      title={title}
      bodyClass={null}
      actions={
        <button type="button" className="cg-linkbtn" onClick={() => onSelectLine(line.id)} data-cg="line-close">
          {t('common.close')}
        </button>
      }
      data-cg="line-detail"
    >
      <div className="cg-panel-b">
        {perms.manageLines && (
          <div className="cg-actions" style={{ flexWrap: 'wrap', marginBottom: 16 }}>
            <Btn onClick={() => forkFrom && ctx.openFork(forkFrom.id)} disabled={!forkFrom} data-cg="line-fork">
              <GitFork size={15} aria-hidden />
              {t('term.fork')}
            </Btn>
            <Btn onClick={() => ctx.openRegister(line.id)} disabled={retired} title={retired ? t('catalog.retiredNoRegister') : undefined} data-cg="line-register">
              <Plus size={15} aria-hidden />
              {t('catalog.registerVersion')}
            </Btn>
            {next && !line.isMain && (
              <Btn onClick={() => setStatusTo(next)} data-cg={`set-${next}`}>
                {t(`catalog.setStatus.${next}`)}
              </Btn>
            )}
          </div>
        )}
        {retired && perms.manageLines && <p className="cg-hint" style={{ marginTop: -8, marginBottom: 14 }}>{t('catalog.retiredNoRegister')}</p>}
        <dl className="cg-kv-rows">
          <dt>{t('term.status')}</dt>
          <dd>
            <Status status={line.status} />
            <div className="cg-meta">{t(`status.hint.${line.status}`)}</div>
          </dd>
          <dt>{t('term.branch')}</dt>
          <dd>
            <span className="cg-branch">{branch}</span>
          </dd>
          {!line.isMain && (
            <>
              <dt>{t('term.sourceVersion')}</dt>
              <dd>
                {src ? (
                  <>
                    <A to={versionHref(app.key, src.id)}>
                      <Mono>{src.number}</Mono>
                    </A>{' '}
                    <span className="cg-meta">· {srcLine?.isMain ? t('term.mainline') : tx(srcLine?.name)}</span>
                  </>
                ) : (
                  '—'
                )}
              </dd>
              <dt>{t('term.forkReason')}</dt>
              <dd className="cg-pre">{line.reason ? tx(line.reason) : '—'}</dd>
              <dt>{t('term.note')}</dt>
              <dd className="cg-pre">{line.note ? tx(line.note) : '—'}</dd>
              <dt>{t('term.plannedEnd')}</dt>
              <dd data-cg="planned-end">{line.plannedEnd ? fmtDate(line.plannedEnd) : '—'}</dd>
            </>
          )}
        </dl>
      </div>
      <div style={{ borderTop: '1px solid var(--cg-hair)' }}>
        <div className="cg-panel-h" style={{ borderBottom: '1px solid var(--cg-hair)', minHeight: 40 }}>
          <h2 style={{ fontSize: 13 }}>
            {t('term.versions')} <span className="cg-count">({vs.length})</span>
          </h2>
        </div>
        {vs.length === 0 ? (
          <p className="cg-note" style={{ padding: 16 }}>{t('catalog.noVersionsYet')}</p>
        ) : (
          vs.map((v) => (
            <A key={v.id} to={versionHref(app.key, v.id)} className="cg-vrow" data-cg="line-version">
              <Mono>{v.number}</Mono>
              <span className="cg-meta" style={{ display: 'inline-flex', gap: 12, alignItems: 'baseline' }}>
                <span className="cg-mono" style={{ fontSize: 12 }}>{v.ref.value}</span>
                <span className="cg-nowrap">{fmtDate(v.releasedAt)}</span>
              </span>
            </A>
          ))
        )}
      </div>
      {statusTo && <StatusDialog line={line} to={statusTo} onClose={() => setStatusTo(null)} />}
    </Panel>
  );
}
