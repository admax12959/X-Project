// Variant C — 表格 + 详情栏: lines as flat rows you can filter and sort; lineage shown as a 源版本 column, detail in a side pane.
import { useState } from 'react';
import { daysUntil, lineageOrder, lineById, lineVersions, useStore, versionById, type LineStatus } from '../../../data';
import { ExpiryText, Mono, StatusText } from '../ui';
import { LineDetail, type LineageProps } from './LineDetail';

type Filter = '全部' | LineStatus;
const FILTERS: Filter[] = ['全部', '活跃', '冻结', '终止'];

export function VariantC({ appKey, selected, onSelect, onFork, onRegister }: LineageProps) {
  const s = useStore();
  const [filter, setFilter] = useState<Filter>('全部');
  const [sort, setSort] = useState<'lineage' | 'expiry'>('lineage');

  let rows = lineageOrder(s, appKey).filter(({ line }) => filter === '全部' || line.isMain || line.status === filter);
  if (sort === 'expiry')
    rows = rows.slice().sort((a, b) => (a.line.plannedEnd ? daysUntil(a.line.plannedEnd) : 1e9) - (b.line.plannedEnd ? daysUntil(b.line.plannedEnd) : 1e9));

  return (
    <div className="grid grid-cols-[minmax(0,1fr)_380px] items-start gap-6">
      <div>
        <div className="mb-2 flex items-center justify-between text-[13px]">
          <div className="flex gap-3">
            {FILTERS.map((f) => (
              <button key={f} onClick={() => setFilter(f)} className={filter === f ? 'font-semibold text-ink' : 'text-ink-2 hover:text-ink'}>
                {f}
              </button>
            ))}
          </div>
          <div className="flex gap-3">
            <span className="text-ink-3">排序</span>
            <button onClick={() => setSort('lineage')} className={sort === 'lineage' ? 'font-semibold' : 'text-ink-2'}>按谱系</button>
            <button onClick={() => setSort('expiry')} className={sort === 'expiry' ? 'font-semibold' : 'text-ink-2'}>按计划终止日期</button>
          </div>
        </div>
        <table className="w-full border-collapse border border-line bg-paper text-left text-[13px]">
          <thead>
            <tr className="border-b border-line text-[12.5px] text-ink-2 [&>th]:px-3 [&>th]:py-2 [&>th]:font-normal">
              <th>线</th>
              <th>源版本</th>
              <th>状态</th>
              <th className="text-right">版本</th>
              <th>最新版本</th>
              <th>计划终止日期</th>
            </tr>
          </thead>
          <tbody>
            {rows.map(({ line, depth }) => {
              const vs = lineVersions(s, line.id);
              const src = line.sourceVersionId ? versionById(line.sourceVersionId) : undefined;
              const srcLine = src ? lineById(src.lineId) : undefined;
              return (
                <tr
                  key={line.id}
                  onClick={() => onSelect(line.id)}
                  className={`cursor-pointer border-b border-line-2 [&>td]:px-3 [&>td]:py-2.5 ${selected === line.id ? 'bg-accent-wash' : 'hover:bg-wash'}`}
                >
                  <td style={{ paddingLeft: 12 + (sort === 'lineage' ? depth * 14 : 0) }}>
                    <div className={`font-medium ${line.status === '终止' ? 'text-ink-3' : ''}`}>{line.isMain ? '主线' : line.name}</div>
                    <Mono className="text-ink-2">{line.branch}</Mono>
                  </td>
                  <td>
                    {src ? (
                      <>
                        {src.number}
                        <div className="text-[12px] text-ink-3">{srcLine?.isMain ? '主线' : srcLine?.name}</div>
                      </>
                    ) : (
                      <span className="text-ink-3">—</span>
                    )}
                  </td>
                  <td>{line.isMain ? <span className="text-ink-3">—</span> : <StatusText line={line} />}</td>
                  <td className="text-right">{vs.length}</td>
                  <td>{vs.at(-1)?.number ?? <span className="text-ink-3">尚无</span>}</td>
                  <td>{line.isMain ? <span className="text-ink-3">—</span> : <ExpiryText line={line} />}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <aside className="sticky top-16 border border-line bg-paper p-5">
        {selected ? (
          <LineDetail lineId={selected} onSelect={onSelect} onFork={onFork} onRegister={onRegister} />
        ) : (
          <p className="text-ink-3">选择一条线查看详情。</p>
        )}
      </aside>
    </div>
  );
}
