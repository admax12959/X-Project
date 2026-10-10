// Variant B — 按源版本缩进: each line's versions read top-down; a forked line sits directly under the version it came from.
import { useState } from 'react';
import { appLines, can, childLines, lineVersions, useStore, type Line } from '../data';
import { ExpiryText, Mono, StatusText } from '../ui';
import { LineDetail, type LineageProps } from './LineDetail';

export function VariantB(props: LineageProps) {
  const s = useStore();
  const main = appLines(s, props.appKey).find((l) => l.isMain);
  // 终止 lines start collapsed: their history matters less than what is still shipping.
  const [open, setOpen] = useState<Record<string, boolean>>({});
  const isOpen = (l: Line) => open[l.id] ?? l.status !== '终止';
  if (!main) return null;
  return (
    <div className="max-w-4xl">
      <LineBlock line={main} depth={0} isOpen={isOpen} toggle={(id, v) => setOpen((o) => ({ ...o, [id]: v }))} {...props} />
      <p className="mt-3 text-[12px] text-ink-3">版本自上而下按发布先后排列；维护线缩进在它的源版本下面。已终止的维护线默认收起。</p>
    </div>
  );
}

function LineBlock({ line, depth, isOpen, toggle, ...props }: LineageProps & { line: Line; depth: number; isOpen: (l: Line) => boolean; toggle: (id: string, v: boolean) => void }) {
  const s = useStore();
  const vers = lineVersions(s, line.id);
  const sel = props.selected === line.id;
  const open = isOpen(line);
  const retired = line.status === '终止';
  const perm = can(s.role);

  return (
    <section className={depth === 0 ? 'border border-line bg-paper' : ''}>
      <header className={`flex items-baseline gap-3 px-4 py-2.5 ${sel ? 'bg-accent-wash' : ''} ${depth === 0 ? 'border-b border-line-2' : ''}`}>
        <button className="w-4 shrink-0 text-ink-3" onClick={() => toggle(line.id, !open)} aria-label={open ? '收起' : '展开'}>
          {open ? '▾' : '▸'}
        </button>
        <button className="min-w-0 flex-1 text-left" onClick={() => props.onSelect(line.id)}>
          <span className={`text-[15px] font-semibold ${retired ? 'text-ink-3' : ''}`}>{line.isMain ? '主线' : line.name}</span>
          <Mono className="ml-2 text-ink-2">{line.branch}</Mono>
          {!line.isMain && (
            <span className="ml-3 text-[13px]">
              <StatusText line={line} />
              {line.plannedEnd && (
                <span className="ml-3 text-ink-2">
                  计划终止 <ExpiryText line={line} />
                </span>
              )}
            </span>
          )}
        </button>
        {!open && <span className="text-[12px] text-ink-3">{vers.length} 个版本</span>}
      </header>

      {sel && !line.isMain && (
        <div className="border-b border-line-2 bg-accent-wash/40 px-4 pt-1 pb-4 pl-11">
          <LineDetail lineId={line.id} showVersions={false} showTitle={false} onSelect={props.onSelect} onFork={props.onFork} onRegister={props.onRegister} />
        </div>
      )}

      {open && (
        <ol className="relative ml-[23px] border-l border-line">
          {vers.map((v) => {
            const kids = childLines(s, v.id);
            return (
              <li key={v.id}>
                <div className="group flex items-baseline gap-3 py-1.5 pr-4 pl-4 text-[13px] hover:bg-wash">
                  <span className={`w-28 shrink-0 ${retired ? 'text-ink-3' : 'font-medium'}`}>{v.number}</span>
                  <span className="w-24 shrink-0 text-ink-2">{v.releasedAt}</span>
                  <span className="min-w-0 flex-1 truncate">
                    <Mono className="text-ink-2">{v.ref.value}</Mono>
                    {v.note && <span className="ml-2 text-ink-2">{v.note}</span>}
                  </span>
                  {perm.manageLines && (
                    <button className="shrink-0 text-accent opacity-0 group-hover:opacity-100 focus:opacity-100" onClick={() => props.onFork(v.id)}>
                      从此版本分叉
                    </button>
                  )}
                </div>
                {kids.length > 0 && (
                  <div className="ml-4 border-l-2 border-ink-3/40 pb-1">
                    {kids.map((k) => (
                      <LineBlock key={k.id} line={k} depth={depth + 1} isOpen={isOpen} toggle={toggle} {...props} />
                    ))}
                  </div>
                )}
              </li>
            );
          })}
          {vers.length === 0 && <li className="py-1.5 pl-4 text-[13px] text-ink-3">分叉后还没有登记版本</li>}
          {perm.manageLines && !retired && (
            <li className="py-1.5 pl-4 text-[13px]">
              <button className="text-accent hover:underline" onClick={() => props.onRegister(line.id)}>登记版本</button>
            </li>
          )}
        </ol>
      )}
    </section>
  );
}
