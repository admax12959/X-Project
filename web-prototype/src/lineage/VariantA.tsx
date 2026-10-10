// Variant A — 泳道时间轴: one lane per line, x = release order, fork edges drawn from the source version.
import { useEffect, useRef, useState } from 'react';
import { appVersions, lineageOrder, lineVersions, useStore } from '../data';
import { ExpiryText, Mono, StatusText } from '../ui';
import { LineDetail, type LineageProps } from './LineDetail';

const ROW = 60;
const COL = 92;
const PAD_X = 36;
const PAD_TOP = 34;
const LABEL_W = 232;

export function VariantA({ appKey, selected, onSelect, onFork, onRegister }: LineageProps) {
  const s = useStore();
  const [hover, setHover] = useState<string | null>(null);
  const scroller = useRef<HTMLDivElement>(null);
  const rows = lineageOrder(s, appKey);
  const all = appVersions(s, appKey);
  const col = new Map(all.map((v, i) => [v.id, i]));
  const x = (i: number) => PAD_X + i * COL;
  const y = (r: number) => PAD_TOP + r * ROW + ROW / 2;
  const rowOf = new Map(rows.map((r, i) => [r.line.id, i]));
  const todayCol = all.length;
  const width = x(todayCol) + PAD_X;
  const height = PAD_TOP + rows.length * ROW + 8;

  // Recent history matters most: keep the selected line's newest version (or today) in view.
  const selVers = selected ? lineVersions(s, selected) : [];
  const focusX = selVers.length ? x(col.get(selVers[selVers.length - 1].id)!) : x(todayCol);
  useEffect(() => {
    const el = scroller.current;
    if (el && (focusX < el.scrollLeft + 40 || focusX > el.scrollLeft + el.clientWidth - 40)) el.scrollLeft = focusX - el.clientWidth + 120;
  }, [focusX]);

  // year ticks where the year changes
  const years = all.map((v, i) => ({ i, y: v.releasedAt.slice(0, 4) })).filter((t, i, a) => i === 0 || a[i - 1].y !== t.y);

  return (
    <div>
      <div className="flex border border-line bg-paper">
        <div className="shrink-0 border-r border-line" style={{ width: LABEL_W }}>
          <div style={{ height: PAD_TOP }} className="border-b border-line-2" />
          {rows.map(({ line, depth }) => (
            <button
              key={line.id}
              onClick={() => onSelect(line.id)}
              style={{ height: ROW, paddingLeft: 12 + depth * 12 }}
              className={`block w-full border-b border-line-2 pr-3 text-left ${selected === line.id ? 'bg-accent-wash' : 'hover:bg-wash'}`}
            >
              <div className={`truncate font-medium ${line.status === '终止' ? 'text-ink-3' : ''}`}>{line.isMain ? '主线' : line.name}</div>
              <div className="mt-0.5 flex gap-2 truncate text-[12px] text-ink-2">
                <StatusText line={line} />
                {line.isMain ? <Mono>{line.branch}</Mono> : line.plannedEnd && <ExpiryText line={line} />}
              </div>
            </button>
          ))}
        </div>
        <div ref={scroller} className="min-w-0 flex-1 overflow-x-auto">
          <svg width={width} height={height} className="block select-none">
            {years.map((t) => (
              <g key={t.y}>
                <line x1={x(t.i) - COL / 2} x2={x(t.i) - COL / 2} y1={0} y2={height} stroke="var(--color-line-2)" />
                <text x={Math.max(6, x(t.i) - COL / 2 + 6)} y={20} fontSize={12} fill="var(--color-ink-3)">{t.y}</text>
              </g>
            ))}
            <line x1={x(todayCol)} x2={x(todayCol)} y1={PAD_TOP - 6} y2={height} stroke="var(--color-ink-3)" strokeDasharray="2 3" />
            <text x={x(todayCol)} y={20} fontSize={12} textAnchor="middle" fill="var(--color-ink-2)">今天</text>

            {rows.map(({ line }, r) => {
              const vs = lineVersions(s, line.id);
              const retired = line.status === '终止';
              const sel = selected === line.id;
              const stroke = sel ? 'var(--color-accent)' : retired ? 'var(--color-ink-3)' : 'var(--color-ink-2)';
              const dash = retired ? '4 4' : undefined;
              const ly = y(r);
              const srcCol = line.sourceVersionId ? col.get(line.sourceVersionId)! : undefined;
              const startX = srcCol !== undefined ? x(srcCol) : x(col.get(vs[0]?.id) ?? 0);
              const endX = retired ? x(col.get(vs[vs.length - 1]?.id) ?? srcCol ?? 0) : x(todayCol);
              const srcLine = line.sourceVersionId ? s.versions.find((v) => v.id === line.sourceVersionId)!.lineId : undefined;
              const sy = srcLine ? y(rowOf.get(srcLine)!) : 0;
              return (
                <g key={line.id} onClick={() => onSelect(line.id)} className="cursor-pointer">
                  <rect x={0} y={ly - ROW / 2} width={width} height={ROW} fill={sel ? 'var(--color-accent-wash)' : 'transparent'} />
                  <line x1={0} x2={width} y1={ly + ROW / 2} y2={ly + ROW / 2} stroke="var(--color-line-2)" />
                  {srcCol !== undefined && (
                    <path d={`M ${startX} ${sy + 6} V ${ly - 10} Q ${startX} ${ly} ${startX + 10} ${ly}`} fill="none" stroke={stroke} strokeWidth={sel ? 2 : 1.25} strokeDasharray={dash} />
                  )}
                  <line x1={srcCol !== undefined ? startX + 10 : startX} x2={endX} y1={ly} y2={ly} stroke={stroke} strokeWidth={sel ? 2 : 1.25} strokeDasharray={dash} />
                  {vs.length === 0 && (
                    <text x={startX + 18} y={ly - 8} fontSize={11.5} fill="var(--color-ink-3)">尚无版本</text>
                  )}
                  {vs.map((v) => {
                    const cx = x(col.get(v.id)!);
                    const isHover = hover === v.id;
                    return (
                      <g key={v.id} onMouseEnter={() => setHover(v.id)} onMouseLeave={() => setHover(null)}>
                        <circle cx={cx} cy={ly} r={isHover ? 6 : 4.5} fill="var(--color-paper)" stroke={stroke} strokeWidth={sel ? 2 : 1.5} />
                        <text x={cx} y={ly - 11} fontSize={11.5} textAnchor="middle" fill={retired ? 'var(--color-ink-3)' : 'var(--color-ink)'} fontWeight={isHover ? 600 : 400}>
                          {v.number}
                        </text>
                        {isHover && (
                          <text x={cx} y={ly + 20} fontSize={11} textAnchor="middle" fill="var(--color-ink-2)">
                            {v.releasedAt} · {v.ref.value}
                          </text>
                        )}
                      </g>
                    );
                  })}
                </g>
              );
            })}
          </svg>
        </div>
      </div>
      <p className="mt-2 text-[12px] text-ink-3">横向按发布先后排列，不按真实时间间距。虚线为已终止的维护线。悬停版本查看日期和 tag/commit，点击一行查看详情。</p>
      {selected && (
        <div className="mt-6 border-t border-line pt-6">
          <LineDetail lineId={selected} layout="wide" onSelect={onSelect} onFork={onFork} onRegister={onRegister} />
        </div>
      )}
    </div>
  );
}
