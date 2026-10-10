import { useLayoutEffect, useRef, useState } from 'react';
import { daysUntil, isExpiring, lineageOrder, lineVersions, TODAY, useStore, type Line, type Version } from '../../data';
import { dueText, Mono, RailGlyph } from './ui';

/*
 * 分支谱系 as a time-scaled rail chart.
 * - One row per line, in lineage order (parent before children). Left column names the line; the canvas
 *   places every version at its 发布时间, so fork points, line lifetimes and 计划终止 sit on one shared time axis.
 * - A maintenance line's rail starts exactly under its 源版本: a connector drops from the source dot and
 *   turns into the child rail with a rounded elbow. Where a connector passes over another line's rail,
 *   the rail is cut for a few pixels (a hop), so a crossing never looks like a junction.
 * - Status is drawn in the stroke itself, not in a colour: 活跃 heavy single rail, 冻结 hollow double rail,
 *   终止 thin grey rail ending in a bar. 主线 is the heaviest rail. Beyond 今天 a dashed tail runs to
 *   计划终止; inside 30 days the tail and its label turn 朱 (the one accent).
 * - Version labels are placed greedily above the rail, falling back below when they would collide.
 */

const DAY = 86400000;
const t = (d: string) => Date.parse(d) / DAY;
const AXIS = 36;
const ROW = 60;
const RAIL = 33; // rail y within a row
const R = 9; // elbow radius
const NAME_W = 184;
const TAIL_PAD = 120;

function textW(s: string) {
  let w = 0;
  for (const c of s) w += /[　-鿿]/.test(c) ? 12.5 : /[A-Z]/.test(c) ? 8 : 6.9;
  return w;
}

export function LineageGraph({ appKey, mainBranch, selLine, selVer, onLine, onVer }: { appKey: string; mainBranch: string; selLine: string | null; selVer: string | null; onLine: (id: string) => void; onVer: (v: Version) => void }) {
  const s = useStore();
  const rows = lineageOrder(s, appKey);
  const vers = new Map(rows.map(({ line }) => [line.id, lineVersions(s, line.id)]));
  const allVers = rows.flatMap(({ line }) => vers.get(line.id)!);
  const verById = new Map(allVers.map((v) => [v.id, v]));
  const rowOfLine = new Map(rows.map(({ line }, i) => [line.id, i]));

  const today = t(TODAY);
  const dates = [today, ...allVers.map((v) => t(v.releasedAt)), ...rows.filter(({ line }) => line.plannedEnd).map(({ line }) => t(line.plannedEnd!))];
  let lo = Math.min(...dates) - 45;
  const hi = Math.max(...dates) + 30;
  if (hi - lo < 400) lo = hi - 400;

  const wrap = useRef<HTMLDivElement>(null);
  const [avail, setAvail] = useState(640);
  useLayoutEffect(() => {
    const el = wrap.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setAvail(el.clientWidth));
    ro.observe(el);
    setAvail(el.clientWidth);
    return () => ro.disconnect();
  }, []);
  // fit the whole history to the width; only below ~11px a month does the canvas scroll sideways
  const ppd = Math.max((avail - TAIL_PAD) / (hi - lo), 0.36);
  const W = Math.ceil((hi - lo) * ppd + TAIL_PAD);
  const lonely = rows.length === 1;
  const H = AXIS + (rows.length + (lonely ? 1 : 0)) * ROW + 8;
  const X = (d: number) => (d - lo) * ppd;
  const yOf = (i: number) => AXIS + i * ROW + RAIL;

  const didScroll = useRef(false);
  useLayoutEffect(() => {
    const el = wrap.current;
    if (el && !didScroll.current && el.scrollWidth > el.clientWidth) {
      el.scrollLeft = el.scrollWidth;
      didScroll.current = true;
    }
  });

  // version x: by date, but versions on one line released close together (or the same day) are pushed
  // at least MIN_GAP apart so every dot stays separately clickable
  const MIN_GAP = 12;
  const verX = new Map<string, number>();
  rows.forEach(({ line }) => {
    let prev = -Infinity;
    vers.get(line.id)!.forEach((v) => {
      const x = Math.max(X(t(v.releasedAt)), prev + MIN_GAP);
      verX.set(v.id, x);
      prev = x;
    });
  });

  // ---- geometry per row
  const geo = rows.map(({ line }, i) => {
    const vs = vers.get(line.id)!;
    const src = line.sourceVersionId ? verById.get(line.sourceVersionId) : undefined;
    const y = yOf(i);
    const forkX = src ? verX.get(src.id) : undefined;
    const start = forkX !== undefined ? forkX + R : vs.length ? verX.get(vs[0].id)! - 14 : X(today) - 40;
    const lastX = vs.length ? verX.get(vs[vs.length - 1].id)! : start;
    const retired = line.status === '终止';
    const solidEnd = retired ? Math.max(lastX + 14, start + 14) : Math.max(X(today), lastX + 8);
    const endX = line.plannedEnd ? X(t(line.plannedEnd)) : undefined;
    const parentRow = src ? rowOfLine.get(src.lineId) : undefined;
    return { line, vs, y, forkX, start, solidEnd, endX, parentRow, retired };
  });

  // ---- label placement: versions that are a 源版本 keep their label above (a connector drops below them);
  // the rest go above when free, else below, else wherever overlaps least.
  const parentVer = new Set(rows.map(({ line }) => line.sourceVersionId).filter(Boolean) as string[]);
  type Lab = { v: Version; x: number; y: number };
  const labels: Lab[] = [];
  geo.forEach((g) => {
    const taken: Record<'above' | 'below', [number, number][]> = { above: [], below: [] };
    const overlap = (slot: 'above' | 'below', a: number, b: number) => taken[slot].reduce((m, [l, r]) => m + Math.max(0, Math.min(b + 6, r) - Math.max(a - 6, l)), 0);
    const items = g.vs.map((v) => ({ v, x: verX.get(v.id)!, w: textW(v.number) }));
    const order = [...items.filter((i) => parentVer.has(i.v.id)), ...items.filter((i) => !parentVer.has(i.v.id))];
    order.forEach(({ v, x, w }) => {
      const a = x - w / 2;
      const b = x + w / 2;
      let slot: 'above' | 'below';
      if (parentVer.has(v.id) || overlap('above', a, b) === 0) slot = 'above';
      else if (overlap('below', a, b) === 0) slot = 'below';
      else if (selVer === v.id) slot = overlap('above', a, b) <= overlap('below', a, b) ? 'above' : 'below';
      else return; // no room: the dot stays (hover shows it), the ledger below lists it
      taken[slot].push([a, b]);
      labels.push({ v, x, y: slot === 'above' ? g.y - 11 : g.y + 21 });
    });
  });

  // where a fork connector passes over another line's rail, cut a gap in that rail (a hop)
  const rowBg = (i: number) => (selLine === geo[i].line.id ? 'var(--wb-sel-row)' : 'var(--color-paper)');
  const hops: { x: number; y: number; i: number }[] = [];
  geo.forEach((g, ci) => {
    if (g.forkX === undefined || g.parentRow === undefined) return;
    for (let r = g.parentRow + 1; r < ci; r++) {
      const o = geo[r];
      const right = !o.retired && o.endX !== undefined ? Math.max(o.solidEnd, o.endX) : o.solidEnd;
      if (g.forkX >= o.start - 2 && g.forkX <= right + 2) hops.push({ x: g.forkX, y: o.y, i: r });
    }
  });

  // ---- axis ticks
  const y0 = new Date(lo * DAY).getUTCFullYear();
  const y1 = new Date(hi * DAY).getUTCFullYear();
  const years: number[] = [];
  for (let y = y0; y <= y1; y++) years.push(y);
  const monthPx = 30.4 * ppd;

  const strokeOf = (l: Line) => (l.status === '终止' ? 'var(--wb-retired)' : 'var(--color-ink)');

  return (
    <div className="flex border-y border-ink">
      {/* name column */}
      <div className="shrink-0 border-r border-line bg-paper" style={{ width: NAME_W }}>
        <div className="flex items-end px-3 pb-1.5 text-[length:var(--wb-text-xs)] font-semibold text-ink-3" style={{ height: AXIS }}>
          线 · {rows.length}
        </div>
        {geo.map(({ line }, i) => {
          const sel = selLine === line.id;
          const d = line.plannedEnd ? daysUntil(line.plannedEnd) : 0;
          const due = isExpiring(line) ? (d < 0 ? `过期 ${-d} 天` : `剩 ${d} 天`) : null;
          return (
            <button
              key={line.id}
              type="button"
              onClick={() => onLine(line.id)}
              aria-pressed={sel}
              className={`flex w-full flex-col justify-center border-t border-line-2 pr-2 text-left ${sel ? 'bg-ink text-white' : 'hover:bg-[var(--wb-hover-row)]'}`}
              style={{ height: ROW, paddingLeft: 12 + rows[i].depth * 13 }}
            >
              <span className="flex w-full items-baseline gap-1.5">
                <span className={`min-w-0 truncate font-semibold ${line.status === '终止' && !sel ? 'text-ink-3' : ''}`}>{line.isMain ? '主线' : line.name}</span>
                {!line.isMain && <span className={`shrink-0 text-[length:var(--wb-text-xs)] ${sel ? 'text-white/75' : 'text-ink-2'}`}>{line.status}</span>}
              </span>
              <span className="flex w-full items-baseline gap-2">
                <Mono className={`min-w-0 truncate text-[length:var(--wb-text-xs)] ${sel ? 'text-white/70' : 'text-ink-3'}`}>{line.isMain ? mainBranch : line.branch}</Mono>
                {due && <span className={`ml-auto shrink-0 text-[length:var(--wb-text-xs)] font-semibold ${sel ? 'text-white' : 'text-accent'}`}>{due}</span>}
              </span>
            </button>
          );
        })}
        {lonely && (
          <div className="flex flex-col justify-center border-t border-line-2 px-3 text-ink-3" style={{ height: ROW }}>
            还没有维护线
          </div>
        )}
      </div>

      {/* canvas */}
      <div ref={wrap} className="wb-scroll min-w-0 flex-1 overflow-x-auto">
        <svg className="wb-graph block" width={W} height={H} role="img" aria-label="分支谱系">
          {/* row backgrounds = click targets for the line */}
          {geo.map(({ line }, i) => (
            <rect
              key={line.id}
              className="wb-row-hit"
              x={0}
              y={AXIS + i * ROW}
              width={W}
              height={ROW}
              fill={selLine === line.id ? 'var(--wb-sel-row)' : 'transparent'}
              onClick={() => onLine(line.id)}
            />
          ))}
          {lonely && (
            <text x={X(today) - 12} y={AXIS + ROW + RAIL + 4} textAnchor="end" fontSize={12} fill="var(--color-ink-3)">
              从主线任一版本分叉后，维护线画在这一行，起点对齐它的源版本
            </text>
          )}
          {geo.map((_, i) => (
            <line key={i} x1={0} x2={W} y1={AXIS + i * ROW + 0.5} y2={AXIS + i * ROW + 0.5} stroke="var(--color-line-2)" />
          ))}

          {/* axis */}
          <line x1={0} x2={W} y1={AXIS - 0.5} y2={AXIS - 0.5} stroke="var(--color-line)" />
          {years.map((y) => {
            const x = X(t(`${y}-01-01`));
            return (
              <g key={y}>
                {x >= 0 && x <= W && <line x1={x} x2={x} y1={10} y2={H} stroke="var(--color-line)" />}
                {[3, 6, 9].map((m) => {
                  const mx = X(t(`${y}-${String(m + 1).padStart(2, '0')}-01`));
                  if (mx < 0 || mx > W) return null;
                  return (
                    <g key={m}>
                      <line x1={mx} x2={mx} y1={AXIS - 6} y2={AXIS} stroke="var(--color-line)" />
                      {monthPx * 3 > 34 && Math.abs(mx - X(today)) > 34 && (
                        <text x={mx + 3} y={AXIS - 8} fontSize={10.5} fill="var(--color-ink-3)">{m + 1}月</text>
                      )}
                    </g>
                  );
                })}
                {x + 4 >= 0 && x < W && (
                  <text x={Math.max(x + 4, 4)} y={AXIS - 8} fontSize={12} fontWeight={700} fill="var(--color-ink)">{y}</text>
                )}
              </g>
            );
          })}

          {/* today */}
          <line x1={X(today)} x2={X(today)} y1={AXIS} y2={H} stroke="var(--color-ink)" strokeWidth={1} strokeDasharray="2 3" />
          <rect x={X(today) - 22} y={4} width={44} height={17} fill="var(--color-ink)" />
          <text x={X(today)} y={16.5} fontSize={11} fill="#fff" textAnchor="middle" fontWeight={600}>今天</text>

          {/* rails */}
          {geo.map((g) => {
            const { line, y, start, solidEnd, endX } = g;
            const c = strokeOf(line);
            const due = isExpiring(line);
            return (
              <g key={line.id} pointerEvents="none">
                {line.status === '冻结' ? (
                  <>
                    <line x1={start} x2={solidEnd} y1={y - 2.25} y2={y - 2.25} stroke={c} strokeWidth={1.5} />
                    <line x1={start} x2={solidEnd} y1={y + 2.25} y2={y + 2.25} stroke={c} strokeWidth={1.5} />
                  </>
                ) : (
                  <line x1={start} x2={solidEnd} y1={y} y2={y} stroke={c} strokeWidth={line.isMain ? 5 : line.status === '终止' ? 1.75 : 3} />
                )}
                {g.retired && <line x1={solidEnd} x2={solidEnd} y1={y - 6} y2={y + 6} stroke={c} strokeWidth={2.5} />}
                {!g.retired && line.isMain && <path d={`M${solidEnd} ${y - 5} L${solidEnd + 8} ${y} L${solidEnd} ${y + 5}Z`} fill={c} />}
                {!g.retired && endX !== undefined && (
                  <>
                    {endX > solidEnd && <line x1={solidEnd} x2={endX} y1={y} y2={y} stroke={due ? 'var(--color-accent)' : 'var(--color-ink-3)'} strokeWidth={1.5} strokeDasharray="4 3" />}
                    <line x1={endX} x2={endX} y1={y - 7} y2={y + 7} stroke={due ? 'var(--color-accent)' : 'var(--color-ink-2)'} strokeWidth={2.5} />
                    <text x={endX + 6} y={y + 4} fontSize={11.5} fontWeight={due ? 700 : 400} fill={due ? 'var(--color-accent)' : 'var(--color-ink-2)'}>
                      {due ? dueText(line) : `计划终止 ${line.plannedEnd!.slice(0, 7)}`}
                    </text>
                  </>
                )}
                {g.vs.length === 0 && (
                  <>
                    <rect x={start + 14} y={y - 21} width={52} height={14} fill={rowBg(rows.findIndex((r) => r.line.id === line.id))} />
                    <text x={start + 16} y={y - 10} fontSize={11.5} fill="var(--color-ink-3)">尚无版本</text>
                  </>
                )}
              </g>
            );
          })}

          {/* hops, then fork connectors */}
          {hops.map((h, k) => (
            <rect key={`hop-${k}`} x={h.x - 4.5} y={h.y - 6} width={9} height={12} fill={rowBg(h.i)} pointerEvents="none" />
          ))}
          {geo.map((g) =>
            g.forkX !== undefined && g.parentRow !== undefined ? (
              <path key={`c-${g.line.id}`} d={`M${g.forkX} ${yOf(g.parentRow) + 5} V${g.y - R} Q${g.forkX} ${g.y} ${g.forkX + R} ${g.y}`} fill="none" stroke={strokeOf(g.line)} strokeWidth={g.retired ? 1.5 : 2} pointerEvents="none" />
            ) : null,
          )}

          {/* versions */}
          {geo.map((g) =>
            g.vs.map((v) => {
              const x = verX.get(v.id)!;
              const sel = selVer === v.id;
              const fill = g.retired ? 'var(--color-paper)' : 'var(--color-ink)';
              const stroke = g.retired ? 'var(--wb-retired)' : 'var(--color-paper)';
              return (
                <g key={v.id} className="wb-ver" onClick={() => onVer(v)}>
                  <title>{`${v.number} · ${v.releasedAt} · ${v.ref.kind} ${v.ref.value}`}</title>
                  <circle cx={x} cy={g.y} r={12} fill="transparent" />
                  {sel && <circle cx={x} cy={g.y} r={9} fill="var(--color-paper)" stroke="var(--color-ink)" strokeWidth={2} />}
                  {v.ref.kind === 'tag' ? (
                    <circle className="wb-ver-dot" cx={x} cy={g.y} r={5} fill={fill} stroke={stroke} strokeWidth={g.retired ? 1.5 : 2} />
                  ) : (
                    <rect x={x - 4.5} y={g.y - 4.5} width={9} height={9} fill={fill} stroke={stroke} strokeWidth={g.retired ? 1.5 : 2} />
                  )}
                </g>
              );
            }),
          )}
          {labels.map(({ v, x, y }) => {
            const sel = selVer === v.id;
            const retired = geo.find((g) => g.line.id === v.lineId)?.retired;
            const w = textW(v.number) + (sel ? 6 : 4);
            return (
              <g key={`l-${v.id}`} className="wb-ver" onClick={() => onVer(v)}>
                <rect x={x - w / 2} y={y - 11} width={w} height={14} fill={rowBg(rowOfLine.get(v.lineId)!)} />
                <text x={x} y={y} textAnchor="middle" fontSize={12} fontWeight={sel ? 700 : 500} fill={retired ? 'var(--color-ink-3)' : 'var(--color-ink)'} textDecoration={sel ? 'underline' : undefined}>
                  {v.number}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    </div>
  );
}

export function Legend() {
  const item = 'flex items-center gap-1.5';
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[length:var(--wb-text-xs)] text-ink-2">
      <span className={item}><RailGlyph status="活跃" width={20} />活跃</span>
      <span className={item}><RailGlyph status="冻结" width={20} />冻结</span>
      <span className={item}><RailGlyph status="终止" width={20} />终止</span>
      <span className={item}>
        <svg width="22" height="10" aria-hidden><line x1={0} x2={16} y1={5} y2={5} stroke="var(--color-ink-3)" strokeWidth={1.5} strokeDasharray="4 3" /><line x1={19} x2={19} y1={0} y2={10} stroke="var(--color-ink-2)" strokeWidth={2.5} /></svg>
        计划终止
      </span>
      <span className={item}><svg width="10" height="10" aria-hidden><circle cx={5} cy={5} r={4} fill="var(--color-ink)" /></svg>tag</span>
      <span className={item}><svg width="10" height="10" aria-hidden><rect x={1} y={1} width={8} height={8} fill="var(--color-ink)" /></svg>commit</span>
    </div>
  );
}
