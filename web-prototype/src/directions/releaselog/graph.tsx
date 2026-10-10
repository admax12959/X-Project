import type { ReactNode } from 'react';
import { appVersions, childLines, lineageOrder, lineById, useStore, type Line, type Version } from '../../data';
import { expiryWords, Mono } from './ui';

/*
 * A `git log --graph` for one 应用: one row per 版本 (newest on top), one rail per 线.
 * - A 维护线's rail leaves its 源版本 node sideways, bends up and runs to the top.
 * - 活跃 rail: solid ink, green nodes. 冻结 rail: a thin double rule, hollow nodes.
 * - 终止 rail: stops at its last 版本 with a cap; above the cap only a dotted guide.
 * - Sticky header is a staircase: each rail climbs to its own label row.
 */

const LANE = 24;
const PAD = 16;
const ROW = 40;
const YEAR = 32;
const HROW = 22;
const COLH = 32;
const R = 7; // elbow radius
const STAGGER = 10;

type Item = { kind: 'year'; year: string; y: number } | { kind: 'ver'; v: Version; y: number };
type Lane = { line: Line; i: number; x: number };

export type GraphSelection = { line: string | null; ver: string | null };

export function VersionGraph({ appKey, mainBranch, sel, onLine, onVersion, empty }: { appKey: string; mainBranch: string; sel: GraphSelection; empty?: ReactNode; onLine: (id: string) => void; onVersion: (id: string) => void }) {
  const s = useStore();
  const order = lineageOrder(s, appKey);
  const lanes: Lane[] = order.map(({ line }, i) => ({ line, i, x: PAD + i * LANE + LANE / 2 }));
  const laneOf = new Map(lanes.map((l) => [l.line.id, l]));
  const versions = appVersions(s, appKey)
    .filter((v) => laneOf.has(v.lineId))
    .slice()
    .reverse();
  const graphW = PAD * 2 + lanes.length * LANE;

  // rows, newest first, with a ruled year divider whenever the year changes
  const items: Item[] = [];
  let y = 0;
  let lastYear = '';
  for (const v of versions) {
    const yr = v.releasedAt.slice(0, 4);
    if (yr !== lastYear) {
      items.push({ kind: 'year', year: yr, y });
      y += YEAR;
      lastYear = yr;
    }
    items.push({ kind: 'ver', v, y });
    y += ROW;
  }
  const bodyH = Math.max(y + 12, 80);
  const cy = new Map<string, number>();
  for (const it of items) if (it.kind === 'ver') cy.set(it.v.id, it.y + ROW / 2);

  // geometry per lane
  type Geo = { lane: Lane; top: number; bottom: number; elbow?: { fromX: number; ySrc: number; yH: number }; capY?: number };
  const geo: Geo[] = lanes.map((lane) => {
    const { line } = lane;
    const own = versions.filter((v) => v.lineId === line.id); // newest first
    const retired = line.status === '终止';
    if (line.isMain) {
      const bottom = own.length ? cy.get(own[own.length - 1].id)! : 24;
      return { lane, top: retired && own.length ? cy.get(own[0].id)! - 8 : 0, bottom, capY: retired && own.length ? cy.get(own[0].id) : undefined };
    }
    const srcLane = line.sourceVersionId ? laneOf.get(s.versions.find((v) => v.id === line.sourceVersionId)?.lineId ?? '') : undefined;
    const ySrc = line.sourceVersionId ? cy.get(line.sourceVersionId) : undefined;
    if (!srcLane || ySrc === undefined) return { lane, top: 0, bottom: 24 };
    // siblings forked from the same 源版本: the farthest lane runs lowest, nearer ones step up
    const sibs = childLines(s, line.sourceVersionId!)
      .map((c) => laneOf.get(c.id))
      .filter((l): l is Lane => !!l)
      .sort((a, b) => b.i - a.i);
    const k = sibs.findIndex((l) => l.line.id === line.id);
    const yH = ySrc - k * STAGGER;
    const bottom = yH - R;
    const newest = own[0] ? cy.get(own[0].id)! : bottom - 10;
    return { lane, top: retired ? newest - 8 : 0, bottom, elbow: { fromX: srcLane.x, ySrc, yH }, capY: retired ? newest : undefined };
  });

  // which lanes have a drawn vertical at height y (for bridging horizontals underneath)
  const verticalAt = (lane: Lane, yy: number) => {
    const g = geo[lane.i];
    return yy >= g.top && yy <= g.bottom;
  };
  const hSeg = (x2: number, yy: number, fromI: number, toI: number) => {
    let d = '';
    for (const l of lanes) if (l.i > fromI && l.i < toI && verticalAt(l, yy)) d += ` H${l.x - 3.5} M${l.x + 3.5},${yy}`;
    return d + ` H${x2}`;
  };
  const srcIndex = (line: Line) => laneOf.get(s.versions.find((v) => v.id === line.sourceVersionId)!.lineId)!.i;

  const selLine = sel.ver ? null : sel.line;

  return (
    <div style={{ minWidth: graphW + ROW_MIN }}>
      {/* sticky staircase header */}
      <div className="rl-graph-head sticky top-14 z-10" style={{ borderBottom: '1px solid var(--rl-ink)' }}>
        <div className="relative" style={{ height: lanes.length * HROW + 8 + COLH }}>
          <svg className="absolute inset-0" width={graphW} height={lanes.length * HROW + 8 + COLH} aria-hidden>
            {lanes.map((lane) => {
              const H = lanes.length * HROW + 8 + COLH;
              const ly = lane.i * HROW + HROW / 2;
              const st = lane.line.status;
              if (st === '终止') return <path key={lane.line.id} className="rl-g-ghost" d={`M${lane.x},${H} V${ly} H${lane.x + 9}`} />;
              const d = `M${lane.x},${H} V${ly + R} Q${lane.x},${ly} ${lane.x + R},${ly} H${lane.x + 10}`;
              if (st === '冻结')
                return (
                  <g key={lane.line.id}>
                    <path className="rl-g-rail rl-g-rail--frozen" d={`M${lane.x - 2},${H} V${ly + R}`} />
                    <path className="rl-g-rail rl-g-rail--frozen" d={`M${lane.x + 2},${H} V${ly + R}`} />
                    <path className="rl-g-rail rl-g-rail--frozen" d={`M${lane.x - 2},${ly + R} Q${lane.x - 2},${ly} ${lane.x + R},${ly} H${lane.x + 10}`} />
                  </g>
                );
              return <path key={lane.line.id} className="rl-g-rail" d={d} />;
            })}
          </svg>
          {lanes.map(({ line, i, x }) => {
            const due = expiryWords(line);
            const active = selLine === line.id;
            return (
              <button
                key={line.id}
                onClick={() => onLine(line.id)}
                aria-pressed={active}
                className="rl-row absolute flex items-center gap-2 rounded-sm pr-2 pl-1"
                style={{ left: x + 12, top: i * HROW + 1, height: HROW - 2, width: 'auto', maxWidth: `calc(100% - ${x + 12}px)` }}
              >
                <span className={`truncate ${active ? 'font-semibold underline underline-offset-4' : 'font-semibold'} ${line.status === '终止' ? 'rl-ink-3' : 'rl-ink'}`}>{line.isMain ? '主线' : line.name}</span>
                {line.isMain ? (
                  <Mono className="rl-ink-2">{mainBranch}</Mono>
                ) : (
                  <>
                    <span className={`shrink-0 font-medium ${line.status === '活跃' ? 'rl-green' : line.status === '冻结' ? 'rl-ink-2' : 'rl-ink-3'}`}>{line.status}</span>
                    {line.plannedEnd && <span className="shrink-0 rl-ink-3">至 {line.plannedEnd}</span>}
                    {due && <span className="rl-stamp shrink-0 rl-ink">{due}</span>}
                  </>
                )}
              </button>
            );
          })}
          <ColumnHeads graphW={graphW} />
        </div>
      </div>

      {/* rows + rails */}
      <div className="relative" style={{ height: bodyH }}>
        {versions.length === 0 && (
          <div className="absolute right-0 py-6" style={{ left: graphW }}>
            {empty}
          </div>
        )}
        {items.map((it) =>
          it.kind === 'year' ? (
            <div key={`y${it.year}`} className="absolute right-0 flex items-end" style={{ top: it.y, left: graphW, height: YEAR }}>
              <span className="pb-1 font-semibold rl-ink-2">{it.year}</span>
              <span className="mb-3 ml-3 flex-1" style={{ borderTop: '1px solid var(--rl-rule-2)' }} />
            </div>
          ) : (
            <VersionRow key={it.v.id} v={it.v} y={it.y} graphW={graphW} pressed={sel.ver === it.v.id} lit={selLine === it.v.lineId} onClick={() => onVersion(it.v.id)} />
          ),
        )}

        <svg className="pointer-events-none absolute top-0 left-0" width={graphW} height={bodyH} aria-hidden>
          {/* horizontals of fork connectors, broken where they pass under another rail */}
          {geo.map((g) =>
            g.elbow ? (
              <path
                key={`h${g.lane.line.id}`}
                className={`rl-g-rail ${g.lane.line.status === '冻结' ? 'rl-g-rail--frozen' : g.lane.line.status === '终止' ? 'rl-g-rail--retired' : ''}`}
                d={`M${g.elbow.fromX},${g.elbow.ySrc} V${g.elbow.yH}${hSeg(g.lane.x - R, g.elbow.yH, srcIndex(g.lane.line), g.lane.i)} Q${g.lane.x},${g.elbow.yH} ${g.lane.x},${g.elbow.yH - R}`}
              />
            ) : null,
          )}
          {/* verticals */}
          {geo.map((g) => {
            const { x, line } = g.lane;
            const st = line.status;
            const parts = [];
            if (g.capY !== undefined) parts.push(<path key="ghost" className="rl-g-ghost" d={`M${x},${g.capY - 8} V0`} />);
            if (st === '冻结')
              parts.push(
                <path key="a" className="rl-g-rail rl-g-rail--frozen" d={`M${x - 2},${g.bottom} V${g.top}`} />,
                <path key="b" className="rl-g-rail rl-g-rail--frozen" d={`M${x + 2},${g.bottom} V${g.top}`} />,
              );
            else parts.push(<path key="r" className={`rl-g-rail ${st === '终止' ? 'rl-g-rail--retired' : ''}`} d={`M${x},${g.bottom} V${g.top}`} />);
            if (g.capY !== undefined) parts.push(<path key="cap" className="rl-g-cap" d={`M${x - 6},${g.capY - 8} H${x + 6}`} />);
            return <g key={line.id}>{parts}</g>;
          })}
          {/* nodes */}
          {versions.map((v) => {
            const lane = laneOf.get(v.lineId)!;
            const c = cy.get(v.id)!;
            const st = lane.line.status;
            return (
              <g key={v.id}>
                {sel.ver === v.id && <circle className="rl-g-sel" cx={lane.x} cy={c} r={9} />}
                <circle className={st === '活跃' ? 'rl-g-node--live' : st === '冻结' ? 'rl-g-node--frozen' : 'rl-g-node--retired'} cx={lane.x} cy={c} r={st === '终止' ? 4 : 5} />
              </g>
            );
          })}
        </svg>
      </div>
    </div>
  );
}

/** column minimums sum to ROW_MIN; the graph never gets narrower than graphW + ROW_MIN */
export const ROW_MIN = 96 + 72 + 92 + 88 + 120;
export const ROW_COLS = (graphW: number) => `${graphW}px minmax(96px,136px) minmax(72px,116px) 92px minmax(88px,168px) minmax(120px,1fr)`;

function ColumnHeads({ graphW }: { graphW: number }) {
  return (
    <div className="absolute right-0 bottom-0 left-0 grid items-center rl-label" style={{ height: COLH, gridTemplateColumns: ROW_COLS(graphW) }}>
      <span />
      <span>版本号</span>
      <span>线</span>
      <span>发布时间</span>
      <span>tag / commit</span>
      <span>说明</span>
    </div>
  );
}

function VersionRow({ v, y, graphW, pressed, lit, onClick }: { v: Version; y: number; graphW: number; pressed: boolean; lit: boolean; onClick: () => void }) {
  const s = useStore();
  const line = lineById(v.lineId)!;
  const forks = childLines(s, v.id);
  const retired = line.status === '终止';
  return (
    <button
      onClick={onClick}
      aria-pressed={pressed}
      data-lit={lit || undefined}
      aria-label={`版本 ${v.number}，${line.isMain ? '主线' : line.name}，${v.releasedAt}`}
      className="rl-row absolute left-0 right-0"
      style={{ top: y, height: ROW, gridTemplateColumns: ROW_COLS(graphW) }}
    >
      <span aria-hidden className="absolute right-0 bottom-0" style={{ left: graphW, borderBottom: '1px solid var(--rl-rule)' }} />
      <span />
      <span className={`truncate rl-t-md rl-mono font-medium ${retired ? 'rl-ink-2' : 'rl-ink'}`}>{v.number}</span>
      <span className="truncate pr-3 rl-ink-2">{line.isMain ? '主线' : line.name}</span>
      <span className="rl-ink-2">{v.releasedAt}</span>
      <span className="truncate pr-3 rl-ink-2">
        <span className="rl-t-xs rl-ink-3">{v.ref.kind} </span>
        <Mono>{v.ref.value}</Mono>
      </span>
      <span className="truncate pr-4" title={[forks.length ? `分叉出${forks.map((f) => `「${f.name}」`).join('')}` : '', v.note].filter(Boolean).join('  ') || undefined}>
        {forks.length > 0 && (
          <span className="mr-3 font-medium rl-ink">
            分叉出{forks.map((f) => `「${f.name}」`).join('')}
          </span>
        )}
        <span className="rl-ink-2">{v.note}</span>
      </span>
    </button>
  );
}
