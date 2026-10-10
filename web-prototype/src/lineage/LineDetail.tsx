import { actions, can, lineById, lineVersions, useStore, versionById, type Line } from '../data';
import { Button, ExpiryText, Mono, StatusText } from '../ui';

export type LineageProps = {
  appKey: string;
  selected: string | null;
  onSelect: (lineId: string) => void;
  onFork: (sourceVersionId: string) => void;
  onRegister: (lineId: string) => void;
};

const STATUS_MEANING: Record<Line['status'], string> = {
  活跃: '可以正常发版',
  冻结: '只允许紧急修复',
  终止: '不再发版，只读',
};

/** Every field of the selected line, so a variant switch never hides state. */
export function LineDetail({ lineId, onSelect, onFork, onRegister, layout = 'stack', showVersions = true, showTitle = true }: { lineId: string; layout?: 'stack' | 'wide'; showVersions?: boolean; showTitle?: boolean } & Pick<LineageProps, 'onSelect' | 'onFork' | 'onRegister'>) {
  const s = useStore();
  const line = s.lines.find((l) => l.id === lineId);
  if (!line) return null;
  const perm = can(s.role);
  const src = line.sourceVersionId ? versionById(line.sourceVersionId) : undefined;
  const srcLine = src ? lineById(src.lineId) : undefined;
  const vers = lineVersions(s, line.id).slice().reverse();
  const retired = line.status === '终止';

  const fields = (
    <dl className="grid grid-cols-[88px_1fr] gap-x-3 gap-y-2.5 text-[13px] [&>dt]:text-ink-2">
      <dt>Git 分支名</dt>
      <dd><Mono>{line.branch}</Mono></dd>
      {!line.isMain && (
        <>
          <dt>源版本</dt>
          <dd>
            {src?.number}
            <span className="text-ink-2"> · 位于</span>
            <button className="ml-1 text-accent hover:underline" onClick={() => srcLine && onSelect(srcLine.id)}>
              {srcLine?.isMain ? '主线' : srcLine?.name}
            </button>
          </dd>
          <dt>分叉原因</dt>
          <dd className="leading-relaxed">{line.reason}</dd>
          <dt>状态</dt>
          <dd>
            <StatusText line={line} />
            <span className="text-ink-3">，{STATUS_MEANING[line.status]}</span>
          </dd>
          <dt>计划终止日期</dt>
          <dd><ExpiryText line={line} /></dd>
          <dt>备注</dt>
          <dd className="text-ink-2">{line.note || '—'}</dd>
          <dt>Owner / PE</dt>
          <dd className="text-ink-2">沿用应用的设置</dd>
        </>
      )}
    </dl>
  );

  const lifecycle = perm.manageLines && !line.isMain && !retired && (
    <div className="flex gap-2">
      {line.status === '活跃' && <Button onClick={() => actions.setLineStatus(line.id, '冻结')}>冻结</Button>}
      <Button kind="danger" onClick={() => confirm(`终止「${line.name}」后只读，不能再登记版本，也不能删除。确定？`) && actions.setLineStatus(line.id, '终止')}>
        终止
      </Button>
    </div>
  );

  const versionList = showVersions && (
    <div>
      <div className="mb-2 flex items-baseline justify-between">
        <h4 className="font-semibold">版本 <span className="font-normal text-ink-3">{vers.length}</span></h4>
        {perm.manageLines && !retired && <Button kind="quiet" onClick={() => onRegister(line.id)}>登记版本</Button>}
      </div>
      {vers.length === 0 ? (
        <p className="text-[13px] text-ink-3">分叉后还没有登记版本。</p>
      ) : (
        <ul className="divide-y divide-line-2 border-y border-line-2 text-[13px]">
          {vers.map((ver) => (
            <li key={ver.id} className="group py-2">
              <div className="flex items-baseline gap-3">
                <span className="min-w-28 font-medium">{ver.number}</span>
                <span className="flex-1 text-ink-2">{ver.releasedAt}</span>
                {perm.manageLines && (
                  <button className="shrink-0 text-accent opacity-0 group-hover:opacity-100 focus:opacity-100" onClick={() => onFork(ver.id)}>
                    从此版本分叉
                  </button>
                )}
              </div>
              <div className="mt-0.5 text-[12px] text-ink-2">
                <span className="text-ink-3">{ver.ref.kind} </span>
                <Mono className="text-[12px]">{ver.ref.value}</Mono>
                {ver.note && <span className="ml-2">{ver.note}</span>}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );

  const head = (
    <div className={`mb-4 flex items-start gap-4 ${showTitle ? 'justify-between' : 'justify-end'}`}>
      {showTitle && <h3 className="text-[17px] font-semibold">{line.isMain ? '主线' : line.name}</h3>}
      {lifecycle}
    </div>
  );

  if (layout === 'wide')
    return (
      <div>
        {head}
        <div className="grid grid-cols-[minmax(0,5fr)_minmax(0,6fr)] gap-10">
          {fields}
          {versionList}
        </div>
      </div>
    );
  return (
    <div>
      {head}
      {fields}
      {showVersions && <div className="mt-6">{versionList}</div>}
    </div>
  );
}
