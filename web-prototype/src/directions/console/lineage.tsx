import { ChevronDown, ChevronRight, GitFork, PackagePlus, X } from 'lucide-react';
import { useMemo, useState, type KeyboardEvent } from 'react';
import { can, latestVersion, lineageOrder, lineById, lineVersions, useStore, versionById, type Application, type Line, type Version } from '../../data';
import { useT } from '../../i18n';
import { appPath, navigate, setParam } from '../../router';
import { nextStatus } from './dialogs';
import { Btn, Container, KV, LinkBtn, Mono, StatusText, useLineName } from './ui';

const GUTTER = 36;
const ROW = 40;
const gx = (depth: number) => 8 + depth * 8;

type Row = { line: Line; depth: number; parentId?: string; hasChildren: boolean };

export function LineageTab({ app, selectedId, onSelect }: { app: Application; selectedId?: string; onSelect: (lineId: string, versionId?: string) => void }) {
  const { t } = useT();
  const s = useStore();
  const lineName = useLineName();
  const [graph, setGraph] = useState(false);
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());

  const { rows, all } = useMemo(() => {
    const order = lineageOrder(s, app.key);
    const parentOf = (l: Line) => {
      const v = l.sourceVersionId ? versionById(l.sourceVersionId) : undefined;
      return v?.lineId;
    };
    const base: Row[] = order.map(({ line, depth }) => ({ line, depth, parentId: parentOf(line), hasChildren: order.some((o) => parentOf(o.line) === line.id) }));
    const visible: Row[] = [];
    let hideBelow: number | null = null;
    for (const r of base) {
      if (hideBelow !== null && r.depth > hideBelow) continue;
      hideBelow = null;
      visible.push(r);
      if (collapsed.has(r.line.id)) hideBelow = r.depth;
    }
    return { rows: visible, all: base };
  }, [s, app.key, collapsed]);

  const toggle = (id: string) =>
    setCollapsed((c) => {
      const n = new Set(c);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });

  const cols = `${graph ? `${GUTTER}px ` : ''}minmax(200px,2fr) 92px minmax(140px,1.2fr) 110px 108px 76px`;
  const minW = (graph ? GUTTER : 0) + 200 + 92 + 140 + 110 + 108 + 76 + 8;

  const onKey = (e: KeyboardEvent, r: Row) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onSelect(r.line.id);
    } else if (e.key === 'ArrowRight' && r.hasChildren && collapsed.has(r.line.id)) toggle(r.line.id);
    else if (e.key === 'ArrowLeft' && r.hasChildren && !collapsed.has(r.line.id)) toggle(r.line.id);
    else if (e.key === 'ArrowDown') ((e.currentTarget as HTMLElement).nextElementSibling as HTMLElement | null)?.focus();
    else if (e.key === 'ArrowUp') ((e.currentTarget as HTMLElement).previousElementSibling as HTMLElement | null)?.focus();
  };

  // graph geometry over visible rows only
  const index = new Map(rows.map((r, i) => [r.line.id, i]));
  const gray = 'var(--c-graph)';
  const accent = 'var(--c-accent)';

  return (
    <Container flush title={`${t('term.lineage')} (${all.length})`}>
      <div className="cs-toolbar" style={{ borderTop: '1px solid var(--c-sep)' }}>
        <button type="button" role="switch" aria-checked={graph} className="cs-switch" onClick={() => setGraph(!graph)}>
          <i aria-hidden />
          {t('console.graph')}
        </button>
        <span style={{ flex: 1 }} />
        <LinkBtn className="cs-small" onClick={() => setCollapsed(new Set())}>
          {t('console.expandAll')}
        </LinkBtn>
        <LinkBtn className="cs-small" onClick={() => setCollapsed(new Set(all.filter((r) => r.hasChildren).map((r) => r.line.id)))}>
          {t('console.collapseAll')}
        </LinkBtn>
      </div>
      <div className="cs-tree">
        <div className="cs-tree-inner" role="treegrid" aria-label={t('term.lineage')} style={{ minWidth: minW }}>
          <div className="cs-tree-row cs-tree-head" role="row" style={{ gridTemplateColumns: cols }}>
            {graph && <div role="columnheader" aria-hidden />}
            <div role="columnheader">{t('term.name')}</div>
            <div role="columnheader">{t('term.status')}</div>
            <div role="columnheader">{t('term.branch')}</div>
            <div role="columnheader">{t('term.sourceVersion')}</div>
            <div role="columnheader">{t('console.latestVersion')}</div>
            <div role="columnheader">{t('term.versions')}</div>
          </div>
          {graph && (
            <svg className="cs-graph-svg" width={GUTTER} height={rows.length * ROW} aria-hidden>
              {rows.map((r, i) => {
                const y = i * ROW + ROW / 2;
                const x = gx(r.depth);
                const sel = r.line.id === selectedId;
                const stroke = sel ? accent : gray;
                const pi = r.parentId !== undefined ? index.get(r.parentId) : undefined;
                const px = gx(r.depth - 1);
                return (
                  <g key={r.line.id}>
                    {pi !== undefined && <path d={`M ${px} ${pi * ROW + ROW / 2} V ${y - 4} Q ${px} ${y} ${px + 4} ${y} H ${x}`} fill="none" stroke={stroke} strokeWidth={1.5} />}
                    <circle cx={x} cy={y} r={r.line.isMain ? 4 : 3.5} fill={r.line.status === 'Retired' && !sel ? '#fff' : stroke} stroke={stroke} strokeWidth={1.5} />
                  </g>
                );
              })}
            </svg>
          )}
          {rows.map((r) => {
            const l = r.line;
            const lv = latestVersion(s, l.id);
            const src = l.sourceVersionId ? versionById(l.sourceVersionId) : undefined;
            const open = !collapsed.has(l.id);
            return (
              <div
                key={l.id}
                role="row"
                aria-level={r.depth + 1}
                aria-expanded={r.hasChildren ? open : undefined}
                aria-selected={l.id === selectedId}
                tabIndex={0}
                className={`cs-tree-row ${l.status === 'Retired' ? 'retired' : ''}`}
                style={{ gridTemplateColumns: cols }}
                onClick={() => onSelect(l.id)}
                onKeyDown={(e) => onKey(e, r)}
              >
                {graph && <div role="gridcell" />}
                <div role="gridcell" className="cs-tree-name" style={{ paddingLeft: 8 + r.depth * 20 }} title={lineName(l)}>
                  {r.hasChildren ? (
                    <button
                      type="button"
                      className="cs-twisty"
                      tabIndex={-1}
                      aria-label={`${open ? t('console.collapse') : t('console.expand')} ${lineName(l)}`}
                      onClick={(e) => {
                        e.stopPropagation();
                        toggle(l.id);
                      }}
                    >
                      {open ? <ChevronDown size={16} aria-hidden /> : <ChevronRight size={16} aria-hidden />}
                    </button>
                  ) : (
                    <span className="cs-twisty-gap" />
                  )}
                  <span>{lineName(l)}</span>
                </div>
                <div role="gridcell">
                  <StatusText status={l.status} />
                </div>
                <div role="gridcell" className="cs-mono" title={l.isMain ? app.mainBranch : l.branch}>
                  {l.isMain ? app.mainBranch : l.branch}
                </div>
                <div role="gridcell" className="cs-mono">
                  {src ? (
                    <LinkBtn
                      className="cs-mono"
                      title={`${t('console.openSource')} ${src.number}`}
                      onClick={(e) => {
                        e.stopPropagation();
                        onSelect(src.lineId, src.id);
                      }}
                    >
                      {src.number}
                    </LinkBtn>
                  ) : (
                    '—'
                  )}
                </div>
                <div role="gridcell" className="cs-mono">
                  {lv ? lv.number : '—'}
                </div>
                <div role="gridcell">{lineVersions(s, l.id).length}</div>
              </div>
            );
          })}
        </div>
      </div>
    </Container>
  );
}

// ---------------------------------------------------------------- split panel

export function LinePanel({
  app,
  line,
  highlight,
  onFork,
  onRegister,
  onStatus,
}: {
  app: Application;
  line: Line;
  highlight?: string;
  onFork: (versionId: string) => void;
  onRegister: (lineId: string) => void;
  onStatus: (line: Line, to: Line['status']) => void;
}) {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const lineName = useLineName();
  const manage = can(s.role).manageLines;
  const vs = lineVersions(s, line.id).slice().reverse();
  const src = line.sourceVersionId ? versionById(line.sourceVersionId) : undefined;
  const srcLine = src ? lineById(src.lineId) : undefined;
  const next = nextStatus(line);
  const retired = line.status === 'Retired';
  return (
    <aside className="cs-panel" aria-label={t('console.lineDetails')}>
      <div className="cs-panel-head">
        <div style={{ minWidth: 0 }}>
          <h2 className="cs-h2" style={{ overflowWrap: 'anywhere' }}>
            {lineName(line)}
          </h2>
          <div className="cs-muted cs-small">{line.isMain ? t('term.mainline') : t('term.line')}</div>
        </div>
        <button type="button" className="cs-iconbtn" aria-label={t('common.close')} onClick={() => setParam('line', null)}>
          <X size={18} aria-hidden />
        </button>
      </div>
      {manage && (
        <div className="cs-panel-sec" style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <Btn small disabled={vs.length === 0} title={vs.length === 0 ? t('console.noSourceVersion') : undefined} onClick={() => onFork(vs[0].id)}>
            <GitFork size={14} aria-hidden /> {t('term.fork')}
          </Btn>
          <Btn small disabled={retired} title={retired ? t('console.retiredNoRegister') : undefined} onClick={() => onRegister(line.id)}>
            <PackagePlus size={14} aria-hidden /> {t('console.registerVersion')}
          </Btn>
        </div>
      )}
      <div className="cs-panel-sec">
        <h3 className="cs-h3">{t('term.status')}</h3>
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8, flexWrap: 'wrap' }}>
          <StatusText status={line.status} />
          {manage && next && (
            <Btn small onClick={() => onStatus(line, next)}>
              {t('console.changeTo', { status: t(`status.${next}`) })}
            </Btn>
          )}
        </div>
        <div className="cs-muted cs-small" style={{ marginTop: 4 }}>
          {t(`status.hint.${line.status}`)}
          {!line.isMain && retired ? ` · ${t('console.statusFinal')}` : ''}
        </div>
      </div>
      <div className="cs-panel-sec">
        <h3 className="cs-h3">{t('console.details')}</h3>
        <KV
          single
          rows={[
            { label: t('term.branch'), value: <Mono>{line.isMain ? app.mainBranch : line.branch}</Mono> },
            ...(line.isMain
              ? []
              : [
                  {
                    label: t('term.sourceVersion'),
                    value:
                      src && srcLine ? (
                        <>
                          <Mono>{src.number}</Mono> <span className="cs-muted cs-small">{t('console.onLine', { line: lineName(srcLine) })}</span>
                        </>
                      ) : (
                        '—'
                      ),
                  },
                  { label: t('term.forkReason'), value: line.reason ? tx(line.reason) : '—' },
                  { label: t('term.note'), value: line.note ? tx(line.note) : '—' },
                  { label: t('term.plannedEnd'), value: line.plannedEnd ? fmtDate(line.plannedEnd) : '—' },
                ]),
          ]}
        />
      </div>
      <div className="cs-panel-sec">
        <h3 className="cs-h3">
          {t('term.versions')} ({vs.length})
        </h3>
        {vs.length === 0 ? (
          <div className="cs-muted">{t('console.noVersionsOnLine')}</div>
        ) : (
          <ul className="cs-version-list">
            {vs.map((v) => (
              <VersionItem key={v.id} v={v} hl={v.id === highlight} appKey={app.key} manage={manage} onFork={onFork} />
            ))}
          </ul>
        )}
      </div>
    </aside>
  );
}

function VersionItem({ v, hl, appKey, manage, onFork }: { v: Version; hl: boolean; appKey: string; manage: boolean; onFork: (id: string) => void }) {
  const { t, fmtDate } = useT();
  return (
    <li className={hl ? 'hl' : ''}>
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, alignItems: 'baseline' }}>
        <LinkBtn className="cs-mono" onClick={() => navigate(`${appPath(appKey, 'versions')}?v=${v.id}`)}>
          {v.number}
        </LinkBtn>
        <span className="cs-muted cs-small">{fmtDate(v.releasedAt)}</span>
      </div>
      <div className="cs-small cs-muted">
        {t(`ref.${v.ref.kind}`)} <Mono>{v.ref.value}</Mono>
      </div>
      {manage && (
        <LinkBtn className="cs-small" onClick={() => onFork(v.id)}>
          {t('console.forkFromVersion')}
        </LinkBtn>
      )}
    </li>
  );
}
