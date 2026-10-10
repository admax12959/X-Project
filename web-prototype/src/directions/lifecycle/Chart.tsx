import { useLayoutEffect, useMemo, useRef, useState } from 'react';
import { appVersions, daysUntil, isExpiring, lineageOrder, lineVersions, TODAY, useStore, versionById, type Line } from '../../data';
import { addDays, daysBetween, dueText, fmtMonthDay, lineSolidEnd, lineStart, monthStart, months, nextMonth } from './time';
import { Seg, Swatch } from './ui';

type Zoom = '1y' | '3y' | 'all';
const ZOOMS: { value: Zoom; label: string }[] = [
  { value: '1y', label: '1 年' },
  { value: '3y', label: '3 年' },
  { value: 'all', label: '全部' },
];
const SPAN: Record<Zoom, number> = { '1y': 365, '3y': 1095, all: 0 };

const LANE = 64; // keep in sync with --lc-lane-h
const AXIS = 44;
const BAR_Y = 34; // bar centre inside a lane: label row 4–24, tick 24–44, label row 44–64

const labelWidth = (t: string) => t.length * 6.9 + 8;

/**
 * The lifecycle chart. Real calendar on x, one lane per line in lineage order on y.
 * Solid = lived (fork date → today). Hatched = planned (today → 计划终止日期). Red = inside the 30-day window.
 */
export function LifecycleChart({ appKey, mainBranch, selected, selectedVer, onSelect }: { appKey: string; mainBranch: string; selected: string; selectedVer: string | null; onSelect: (lineId: string, verId?: string) => void }) {
  const s = useStore();
  const rows = lineageOrder(s, appKey);
  const scroller = useRef<HTMLDivElement>(null);
  const labelCol = useRef<HTMLDivElement>(null);
  const [box, setBox] = useState({ w: 900, label: 248 });
  const [zoom, setZoom] = useState<Zoom>('all');

  useLayoutEffect(() => {
    const el = scroller.current;
    if (!el) return;
    const measure = () => setBox({ w: el.clientWidth, label: labelCol.current?.offsetWidth ?? 248 });
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Calendar domain: a month before the first thing that happened, two months past the last planned end.
  const { from, to } = useMemo(() => {
    const dates = [TODAY, ...appVersions(s, appKey).map((v) => v.releasedAt), ...rows.flatMap(({ line }) => (line.plannedEnd ? [line.plannedEnd] : []))].sort();
    return { from: monthStart(addDays(dates[0], -20)), to: nextMonth(addDays(dates.at(-1)!, 80)) };
  }, [s, appKey, rows]);

  const total = daysBetween(from, to);
  const viewW = Math.max(240, box.w - box.label);
  const fit = viewW / total;
  // Floor keeps 全部 legible on narrow screens: it scrolls inside the chart instead of crushing.
  const ppd = Math.max(0.6, zoom === 'all' ? fit : Math.max(fit, viewW / SPAN[zoom]));
  const chartW = Math.round(total * ppd);
  const x = (d: string) => daysBetween(from, d) * ppd;

  useLayoutEffect(() => {
    const el = scroller.current;
    if (!el) return;
    el.scrollLeft = Math.max(0, x(TODAY) - viewW * 0.72); // 0 whenever the whole calendar fits
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [zoom, viewW]);

  const monthList = months(from, addDays(to, -1));
  const pxMonth = ppd * 30.4;
  const monthLabel = (m: string) => {
    const n = Number(m.slice(5, 7));
    if (n === 1) return null;
    if (pxMonth >= 44) return `${n}月`;
    if (pxMonth >= 20 && n % 3 === 1) return `${n}月`;
    return null;
  };

  const rowIndex = new Map(rows.map(({ line }, i) => [line.id, i]));
  const height = AXIS + rows.length * LANE;
  const expiring = rows.filter(({ line }) => isExpiring(line));

  return (
    <figure className="m-0">
      <div className="mb-3 flex flex-wrap items-end justify-between gap-4">
        <figcaption className="text-[var(--lc-ink-2)]">
          {rows.length} 条线，横轴是日历。
          {expiring.length > 0 ? (
            <span className="lc-signal-text">
              {expiring.map(({ line }) => `「${line.name}」${dueText(line.plannedEnd!)}`).join('，')}。
            </span>
          ) : (
            <span>30 天内没有到期的维护线。</span>
          )}
        </figcaption>
        <Seg label="时间范围" value={zoom} options={ZOOMS} onChange={setZoom} />
      </div>

      <div ref={scroller} className="relative overflow-x-auto overflow-y-hidden border-y-2 border-[var(--lc-ink)]" tabIndex={-1}>
        <div className="relative" style={{ width: box.label + chartW, height }}>
          {/* ground layer: selection band, month grid, years */}
          <div aria-hidden className="pointer-events-none absolute top-0 z-0" style={{ left: box.label, width: chartW, height }}>
            {rowIndex.has(selected) && <div className="absolute inset-x-0 bg-[var(--lc-paper-3)]" style={{ top: AXIS + rowIndex.get(selected)! * LANE, height: LANE }} />}
            {pxMonth >= 8 &&
              monthList.map((m) => (
                <div key={m} className={`absolute top-0 bottom-0 w-px ${m.endsWith('-01-01') ? 'bg-[var(--lc-rule-2)]' : 'bg-[var(--lc-rule)]'}`} style={{ left: x(m) }} />
              ))}
            {pxMonth < 8 && monthList.filter((m) => m.endsWith('-01-01')).map((m) => <div key={m} className="absolute top-0 bottom-0 w-px bg-[var(--lc-rule-2)]" style={{ left: x(m) }} />)}
          </div>

          {/* axis */}
          <div className="relative z-[1] flex border-b border-[var(--lc-ink)]" style={{ height: AXIS }}>
            <div ref={labelCol} className="sticky left-0 z-[6] flex w-[var(--lc-label-w)] shrink-0 items-end border-r border-[var(--lc-ink)] bg-[var(--lc-paper)] px-3 pb-2">
              <span className="lc-label">线</span>
            </div>
            <div className="relative" style={{ width: chartW }}>
              {monthList.map((m) => {
                const year = m.endsWith('-01-01') || m === monthList[0];
                const nearToday = Math.abs(x(m) - x(TODAY)) < 44;
                const ml = nearToday ? null : monthLabel(m);
                return (
                  <span key={m} className="absolute bottom-2 pl-1.5 whitespace-nowrap" style={{ left: x(m) }}>
                    {year ? <span className="lc-num text-[length:var(--lc-text-sm)] font-bold">{m.slice(0, 4)}</span> : ml && <span className="lc-num text-[length:var(--lc-text-xs)] text-[var(--lc-muted)]">{ml}</span>}
                  </span>
                );
              })}
              <span className="absolute top-1.5 z-[4] -translate-x-1/2 bg-[var(--lc-paper)] px-1 text-[length:var(--lc-text-xs)] font-bold whitespace-nowrap text-[var(--lc-signal)]" style={{ left: x(TODAY) }}>
                今天 <span className="lc-num">{fmtMonthDay(TODAY)}</span>
              </span>
            </div>
          </div>

          {/* fork connectors under the bars; the source-version square above them */}
          {(['lines', 'marks'] as const).map((layer) => (
            <div key={layer} aria-hidden className={`pointer-events-none absolute top-0 ${layer === 'lines' ? 'z-[1]' : 'z-[3]'}`} style={{ left: box.label, width: chartW, height }}>
              {rows.map(({ line }) => {
                const src = line.sourceVersionId ? versionById(line.sourceVersionId) : undefined;
                const pi = src ? rowIndex.get(src.lineId) : undefined;
                if (!src || pi === undefined) return null;
                const y1 = AXIS + pi * LANE + BAR_Y;
                const y2 = AXIS + rowIndex.get(line.id)! * LANE + BAR_Y;
                return layer === 'lines' ? (
                  <div key={line.id} className="absolute w-px bg-[var(--lc-ink)]" style={{ left: x(src.releasedAt), top: y1, height: y2 - y1 }} />
                ) : (
                  <div key={line.id} className="absolute h-2.5 w-2.5 -translate-x-1/2 -translate-y-1/2 border-2 border-[var(--lc-ink)] bg-[var(--lc-paper)]" style={{ left: x(src.releasedAt) + 0.5, top: y1 }} />
                );
              })}
            </div>
          ))}

          {/* lanes */}
          {rows.map(({ line, depth }) => (
            <Lane key={line.id} line={line} depth={depth} mainBranch={mainBranch} x={x} chartW={chartW} selected={selected === line.id} selectedVer={selectedVer} onSelect={onSelect} />
          ))}

          {/* today */}
          <div aria-hidden className="pointer-events-none absolute z-[4] w-0.5 -translate-x-1/2 bg-[var(--lc-signal)]" style={{ left: box.label + x(TODAY), top: 26, bottom: 0 }} />
        </div>
      </div>

      <Legend />
    </figure>
  );
}

function Lane({ line, depth, mainBranch, x, chartW, selected, selectedVer, onSelect }: { line: Line; depth: number; mainBranch: string; x: (d: string) => number; chartW: number; selected: boolean; selectedVer: string | null; onSelect: (lineId: string, verId?: string) => void }) {
  const s = useStore();
  const vers = lineVersions(s, line.id);
  const start = lineStart(s, line);
  const solidEnd = lineSolidEnd(s, line);
  const retired = line.status === '终止';
  const exp = isExpiring(line);
  const overdue = !!line.plannedEnd && !retired && daysUntil(line.plannedEnd) < 0;
  const name = line.isMain ? '主线' : line.name;

  // Greedy label placement: above the bar, else below, else tick only (still focusable and named).
  let above = -Infinity;
  let below = -Infinity;
  const endX = line.plannedEnd && !retired ? x(line.plannedEnd) : null;
  const placed = vers.map((v) => {
    const cx = x(v.releasedAt);
    const w = labelWidth(v.number);
    let slot: 'above' | 'below' | null = null;
    if (cx - w / 2 > above + 6) (slot = 'above'), (above = cx + w / 2);
    else if (cx - w / 2 > below + 6 && (endX === null || cx + w / 2 < endX - 4)) (slot = 'below'), (below = cx + w / 2);
    return { v, cx, slot };
  });

  // End-date label sits right of the cap; flips to the left when it would run off the calendar.
  const endLabelX = line.plannedEnd ? Math.max(x(line.plannedEnd), overdue ? x(TODAY) : 0) : 0;
  const endLabelFlip = endLabelX + (exp || overdue ? 150 : 84) > chartW;
  const barClass = retired ? 'lc-bar-retired' : line.status === '冻结' ? 'lc-bar-frozen' : 'lc-bar-active';
  const barH = retired ? 4 : 12;

  return (
    <div className="relative flex" style={{ height: LANE }}>
      <button
        type="button"
        onClick={() => onSelect(line.id)}
        aria-pressed={selected}
        className={`lc-row sticky left-0 z-[6] flex w-[var(--lc-label-w)] shrink-0 flex-col justify-center border-r border-b border-[var(--lc-ink)] border-b-[var(--lc-rule)] pr-3 text-left ${selected ? '!bg-[var(--lc-paper-3)]' : 'bg-[var(--lc-paper)]'}`}
        style={{ paddingLeft: 12 + depth * 16 }}
      >
        {depth > 0 && <span aria-hidden className="absolute top-0 h-[34px] w-2 border-b border-l border-[var(--lc-muted)]" style={{ left: 12 + (depth - 1) * 16 + 3 }} />}
        <span className="flex items-baseline gap-2">
          <span className={`flex min-w-0 flex-1 items-center gap-1.5 text-[length:var(--lc-text-base)] leading-tight ${selected ? 'font-extrabold' : 'font-semibold'} ${retired ? 'text-[var(--lc-muted)]' : ''}`}>
            {selected && <span aria-hidden className="inline-block h-2 w-2 shrink-0 bg-[var(--lc-ink)]" />}
            <span className="truncate">{name}</span>
          </span>
          {!line.isMain && (
            <span className={`shrink-0 text-[length:var(--lc-text-xs)] ${exp ? 'font-bold text-[var(--lc-signal)]' : 'text-[var(--lc-muted)]'}`}>
              {exp ? (overdue ? '已过期' : `剩 ${daysUntil(line.plannedEnd!)} 天`) : line.status}
            </span>
          )}
        </span>
        <span className="lc-code mt-1 truncate text-[length:var(--lc-text-sm)] leading-tight text-[var(--lc-muted)]">{line.isMain ? mainBranch : line.branch}</span>
      </button>

      <div className="relative border-b border-[var(--lc-rule)]" style={{ width: chartW }}>
        {/* lived */}
        <div
          onClick={() => onSelect(line.id)}
          className={`absolute z-[2] cursor-pointer ${barClass}`}
          style={{ left: x(start), width: Math.max(2, x(solidEnd) - x(start)), top: BAR_Y - barH / 2, height: barH }}
        />
        {retired && <div aria-hidden className="absolute z-[2] h-3 w-0.5 bg-[var(--lc-muted)]" style={{ left: x(solidEnd), top: BAR_Y - 6 }} />}

        {/* planned */}
        {!retired && line.plannedEnd && line.plannedEnd > TODAY && (
          <div onClick={() => onSelect(line.id)} className={`absolute z-[2] cursor-pointer ${exp ? 'lc-hatch--signal' : 'lc-hatch'}`} style={{ left: x(TODAY), width: x(line.plannedEnd) - x(TODAY), top: BAR_Y - 6, height: 12 }} />
        )}
        {!retired && line.plannedEnd && (
          <>
            <div aria-hidden className={`absolute z-[3] w-[3px] -translate-x-1/2 ${exp ? 'bg-[var(--lc-signal)]' : 'bg-[var(--lc-ink)]'}`} style={{ left: x(line.plannedEnd), top: BAR_Y - 12, height: 24 }} />
            <span
              className={`absolute z-[3] px-2 text-[length:var(--lc-text-xs)] whitespace-nowrap ${exp ? 'font-bold text-[var(--lc-signal)]' : 'text-[var(--lc-ink-2)]'}`}
              style={endLabelFlip ? { right: chartW - endLabelX, top: BAR_Y + 8 } : { left: endLabelX, top: BAR_Y + 8 }}
            >
              <span className="lc-num">{line.plannedEnd}</span>
              {(exp || overdue) && <> · {dueText(line.plannedEnd)}</>}
            </span>
          </>
        )}
        {!retired && !line.isMain && !line.plannedEnd && (
          <span className="absolute z-[3] pl-3 text-[length:var(--lc-text-xs)] whitespace-nowrap text-[var(--lc-muted)]" style={{ left: x(TODAY), top: BAR_Y - 8 }}>
            未定计划终止日期
          </span>
        )}
        {line.isMain && (
          <span aria-hidden className="absolute z-[3] h-2.5 w-2.5 rotate-45 border-t-2 border-r-2 border-[var(--lc-ink)]" style={{ left: x(TODAY) + 4, top: BAR_Y - 5 }} />
        )}
        {!line.isMain && vers.length === 0 && (
          <span className="absolute z-[3] text-[length:var(--lc-text-xs)] whitespace-nowrap text-[var(--lc-muted)]" style={{ left: x(start) + 6, top: 6 }}>
            还没有版本
          </span>
        )}

        {/* versions */}
        {placed.map(({ v, cx, slot }) => {
          const on = selectedVer === v.id;
          return (
            <button
              key={v.id}
              type="button"
              aria-pressed={on}
              aria-label={`${name} 版本 ${v.number}，${v.releasedAt}`}
              title={`${v.number} · ${v.releasedAt}`}
              data-retired={retired ? '' : undefined}
              onClick={() => onSelect(line.id, v.id)}
              className="lc-tick z-[5] min-w-4"
              style={{ left: cx, top: BAR_Y - 10 - (slot === 'above' ? 20 : 0) }}
            >
              {slot === 'above' && <Tag on={on} retired={retired}>{v.number}</Tag>}
              <i />
              {slot === 'below' && <Tag on={on} retired={retired}>{v.number}</Tag>}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function Tag({ children, on, retired }: { children: string; on: boolean; retired: boolean }) {
  return (
    <span className={`lc-num block h-5 px-0.5 text-[length:var(--lc-text-xs)] leading-5 whitespace-nowrap ${on ? 'bg-[var(--lc-ink)] font-bold text-[var(--lc-paper)]' : retired ? 'bg-[var(--lc-paper)] text-[var(--lc-muted)]' : 'bg-[var(--lc-paper)] font-semibold text-[var(--lc-ink)]'}`}>
      {children}
    </span>
  );
}

function Legend() {
  const items: [string, string][] = [
    ['lc-bar-active', '活跃，已走过的时间'],
    ['lc-bar-frozen', '冻结'],
    ['lc-hatch', '今天到计划终止日期'],
    ['lc-hatch--signal', '30 天内到期'],
    ['lc-bar-retired !h-1', '终止'],
  ];
  return (
    <ul className="mt-3 flex flex-wrap gap-x-6 gap-y-1 text-[length:var(--lc-text-sm)] text-[var(--lc-ink-2)]">
      {items.map(([c, t]) => (
        <li key={t} className="flex items-center gap-2">
          <Swatch className={c} />
          {t}
        </li>
      ))}
      <li className="flex items-center gap-2">
        <span aria-hidden className="inline-block h-4 w-0.5 bg-[var(--lc-signal)]" />
        今天
      </li>
      <li className="flex items-center gap-2">
        <span aria-hidden className="inline-block h-2.5 w-2.5 border-2 border-[var(--lc-ink)]" />
        从这个版本分叉
      </li>
    </ul>
  );
}
