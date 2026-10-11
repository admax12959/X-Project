// Lineage tab: every line of the application as an expandable tree table (ARIA treegrid).
// The mainline is the root; maintenance lines nest under the line that holds their source
// version, to any depth. Selecting a row opens the line's panel on the right; the selection
// lives in the URL (?line=<id>) so a panel can be linked to. Owners can fork from the panel
// (any of the line's versions), register a version on the selected line, edit it and move
// it along the status state machine (confirmation dialogs; see lineLifecycle.ts).
//
// Layout: the panel sits next to the tree inside the tab. With a line selected the "Source
// version" column is dropped (the panel shows it), and the other columns shrink, so the tree
// fits at 1280px without scrolling sideways.
//
// Graph column (#11): an optional first column that draws the forks (layout in lineageGraph.ts,
// drawing in LineageGraphCell.tsx). Off by default; the choice is kept in localStorage.
// To extend the tree: add a column to `template` and a cell in each row; `rows` is the
// depth-first list of visible nodes.
import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { useSearchParams } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { LineDetail, LineStatus, LineStatusChange, LineageNode, Version } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Empty } from '../ui/controls.tsx';
import { LineStatusLabel } from '../ui/LineStatusLabel.tsx';
import { useApplication } from './ApplicationPage.tsx';
import { EditLineDialog } from './EditLineDialog.tsx';
import { ForkDialog } from './ForkDialog.tsx';
import { LinePanel } from './LinePanel.tsx';
import { LineStatusDialog } from './LineStatusDialog.tsx';
import { RegisterVersionDialog } from './RegisterVersionDialog.tsx';
import { graphLanes, layoutGraph } from './lineageGraph.ts';
import { graphWidth, LineageGraphCell, ROW_H } from './LineageGraphCell.tsx';

const GRAPH_KEY = 'xp.lineage.graph';

/** The graph switch, remembered across visits. Storage may be unavailable: then it is just off. */
function useGraphSwitch(): [boolean, (on: boolean) => void] {
  const [on, setOn] = useState(() => {
    try {
      return localStorage.getItem(GRAPH_KEY) === '1';
    } catch {
      return false;
    }
  });
  const set = (v: boolean) => {
    setOn(v);
    try {
      localStorage.setItem(GRAPH_KEY, v ? '1' : '0');
    } catch {
      // not remembered; the switch still works for this visit
    }
  };
  return [on, set];
}

type Data = { root: LineageNode; lines: LineDetail[]; versions: Version[] };
type Load = { status: 'loading' } | { status: 'error'; error: unknown } | ({ status: 'ok' } & Data);
type Row = { node: LineageNode; depth: number };

/** Depth-first visible rows; collapsed nodes hide their descendants. */
function visibleRows(node: LineageNode, collapsed: Set<string>, depth = 0, out: Row[] = []): Row[] {
  out.push({ node, depth });
  if (!collapsed.has(node.id)) for (const c of node.children) visibleRows(c, collapsed, depth + 1, out);
  return out;
}

/** IDs of every node that has children (what "Collapse all" collapses). */
function parentIds(node: LineageNode, out: string[] = []): string[] {
  if (node.children.length) out.push(node.id);
  for (const c of node.children) parentIds(c, out);
  return out;
}

export function ApplicationLineage() {
  const { t } = useT();
  const app = useApplication();
  const user = useCurrentUser();
  const [params, setParams] = useSearchParams();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const [forkFrom, setForkFrom] = useState<string | null>(null);
  const [registering, setRegistering] = useState(false);
  const [editing, setEditing] = useState(false);
  const [statusTarget, setStatusTarget] = useState<LineStatus | null>(null);
  // The status log of the selected line (null while loading); `logKey` bumps to refetch.
  const [log, setLog] = useState<{ lineId: string; items: LineStatusChange[] | 'error' } | null>(null);
  const [logKey, setLogKey] = useState(0);
  const [showGraph, setShowGraph] = useGraphSwitch();
  const rowRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const reload = useCallback(() => {
    let live = true;
    Promise.all([api.applications.lineage(app.key), api.applications.lines(app.key), api.applications.versions(app.key)]).then(
      ([root, l, v]) => live && setLoad({ status: 'ok', root, lines: l.items, versions: v.items }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, [app.key]);
  useEffect(() => reload(), [reload]);

  const isOwner = app.owners.some((o) => o.id === user.id); // display only; the server decides
  const data = load.status === 'ok' ? load : null;
  const rows = useMemo(() => (data ? visibleRows(data.root, collapsed) : []), [data, collapsed]);
  const graph = useMemo(() => (data && showGraph ? { rows: layoutGraph(rows), lanes: graphLanes(data.root) } : null), [data, rows, showGraph]);
  const selectedId = params.get('line') ?? '';
  const selected = data?.lines.find((l) => l.id === selectedId);

  const selectedIsLine = !!selected && !selected.isMain;
  useEffect(() => {
    if (!selectedIsLine) return;
    let live = true;
    api.applications.line(app.key, selectedId).then(
      (d) => live && setLog({ lineId: selectedId, items: d.statusChanges }),
      () => live && setLog({ lineId: selectedId, items: 'error' }),
    );
    return () => {
      live = false;
    };
  }, [app.key, selectedId, selectedIsLine, logKey]);
  /** After a status change or an edit: reload the tree, the panel and the status log. */
  const changed = () => {
    setStatusTarget(null);
    setEditing(false);
    setLogKey((k) => k + 1);
    reload();
  };

  const select = (id: string) => setParams(id ? { line: id } : {}, { replace: true });
  const toggle = (id: string, open: boolean) =>
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (open) next.delete(id);
      else next.add(id);
      return next;
    });

  // 5 columns; the source version is dropped while the panel is open.
  const withSource = !selected;
  const columns = withSource
    ? 'minmax(160px,2fr) 100px minmax(110px,1.5fr) minmax(80px,1fr) minmax(80px,1fr)'
    : 'minmax(130px,2fr) 96px minmax(100px,1.5fr) minmax(70px,1fr)';
  const template = graph ? `${graphWidth(graph.lanes)}px ${columns}` : columns;
  const cell = 'min-w-0 truncate px-2.5';
  const headCell = `${cell} whitespace-nowrap text-[13px] font-semibold`;

  const onKey = (e: KeyboardEvent, i: number) => {
    const { node } = rows[i];
    const move = (to: number) => {
      const target = rows[to];
      if (target) {
        e.preventDefault();
        rowRefs.current.get(target.node.id)?.focus();
      }
    };
    switch (e.key) {
      case 'ArrowDown':
        return move(i + 1);
      case 'ArrowUp':
        return move(i - 1);
      case 'ArrowRight':
        if (node.children.length && collapsed.has(node.id)) {
          e.preventDefault();
          toggle(node.id, true);
        }
        return;
      case 'ArrowLeft':
        if (node.children.length && !collapsed.has(node.id)) {
          e.preventDefault();
          toggle(node.id, false);
        }
        return;
      case 'Enter':
      case ' ':
        e.preventDefault();
        select(node.id);
    }
  };

  if (load.status === 'loading') return <Empty title={t('common.loading')} />;
  if (load.status === 'error') {
    return (
      <Empty title={errorsByField(load.error)[''] ?? t('lineage.loadFailed')}>
        <Button small onClick={reload}>
          {t('common.retry')}
        </Button>
      </Empty>
    );
  }

  const panelVersions = selected ? load.versions.filter((v) => v.lineId === selected.id) : [];
  const lineLabel = (n: LineDetail) => (n.isMain ? t('term.mainline') : n.name);

  return (
    <div className={`grid items-start gap-4 ${selected ? 'grid-cols-[minmax(0,1fr)_340px]' : 'grid-cols-1'}`}>
      <section className="min-w-0 rounded border border-line bg-white">
        <div className="flex items-center gap-2 border-b border-line px-4 py-2.5">
          <h2 className="text-base font-bold">{t('lineage.tree')}</h2>
          <span className="text-[13px] text-text-3">{t('lineage.count', { count: load.lines.length })}</span>
          <span className="flex-1" />
          <button
            type="button"
            role="switch"
            aria-checked={showGraph}
            onClick={() => setShowGraph(!showGraph)}
            className="inline-flex items-center gap-2 rounded px-1.5 py-1 text-[13px] text-text-2 hover:bg-subtle"
          >
            <span aria-hidden className={`relative inline-block h-4 w-7 rounded-full border ${showGraph ? 'border-accent bg-accent' : 'border-text-3 bg-white'}`}>
              <span className={`absolute top-px h-3 w-3 rounded-full ${showGraph ? 'left-[13px] bg-white' : 'left-px bg-text-3'}`} />
            </span>
            {t('lineage.graph')}
          </button>
          <Button small onClick={() => setCollapsed(new Set())}>
            {t('lineage.expandAll')}
          </Button>
          <Button small onClick={() => setCollapsed(new Set(parentIds(load.root)))}>
            {t('lineage.collapseAll')}
          </Button>
        </div>
        <div role="treegrid" aria-label={t('lineage.tree')} className="text-sm">
          <div role="row" className="grid items-center border-b border-line bg-thead" style={{ gridTemplateColumns: template, height: ROW_H }}>
            {graph && (
              <div role="columnheader">
                <span className="sr-only">{t('lineage.graph')}</span>
              </div>
            )}
            <div role="columnheader" className={headCell}>
              {t('lineage.col.name')}
            </div>
            <div role="columnheader" className={headCell}>
              {t('lineage.col.status')}
            </div>
            <div role="columnheader" className={headCell}>
              {t('lineage.col.branch')}
            </div>
            {withSource && (
              <div role="columnheader" className={headCell}>
                {t('lineage.col.source')}
              </div>
            )}
            <div role="columnheader" className={headCell}>
              {t('lineage.col.latest')}
            </div>
          </div>
          {rows.map(({ node, depth }, i) => {
            const open = !collapsed.has(node.id);
            const hasChildren = node.children.length > 0;
            const isSelected = node.id === selectedId;
            return (
              <div
                key={node.id}
                role="row"
                aria-level={depth + 1}
                aria-selected={isSelected}
                aria-expanded={hasChildren ? open : undefined}
                tabIndex={i === 0 || isSelected ? 0 : -1}
                ref={(el) => {
                  if (el) rowRefs.current.set(node.id, el);
                  else rowRefs.current.delete(node.id);
                }}
                onClick={() => select(node.id)}
                onKeyDown={(e) => onKey(e, i)}
                className={`grid cursor-pointer items-center border-b border-sep ${isSelected ? 'bg-sel' : 'hover:bg-subtle'} ${node.status === 'retired' ? 'text-text-3' : ''}`}
                style={{ gridTemplateColumns: template, height: ROW_H }}
              >
                {graph && (
                  <div role="gridcell" className="relative self-stretch">
                    <LineageGraphCell row={graph.rows[i]} lanes={graph.lanes} selected={isSelected} />
                  </div>
                )}
                <div role="gridcell" className={`${cell} flex items-center`} style={{ paddingLeft: 10 + depth * 20 }}>
                  {hasChildren ? (
                    <button
                      type="button"
                      tabIndex={-1}
                      aria-label={t(open ? 'lineage.collapse' : 'lineage.expand')}
                      className="mr-1 inline-flex h-5 w-5 shrink-0 items-center justify-center rounded text-text-2 hover:bg-sep"
                      onClick={(e) => {
                        e.stopPropagation();
                        toggle(node.id, !open);
                      }}
                    >
                      <svg width="12" height="12" viewBox="0 0 12 12" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                        <path d={open ? 'M2.5 4.5L6 8l3.5-3.5' : 'M4.5 2.5L8 6 4.5 9.5'} />
                      </svg>
                    </button>
                  ) : (
                    <span className="mr-1 inline-block w-5 shrink-0" />
                  )}
                  <span className={`truncate ${node.isMain ? 'font-semibold' : ''}`}>{lineLabel(node)}</span>
                </div>
                <div role="gridcell" className={cell}>
                  {node.isMain ? <span className="text-text-3">—</span> : <LineStatusLabel status={node.status} />}
                </div>
                <div role="gridcell" className={`${cell} font-mono text-[12.5px]`}>
                  {node.branch}
                </div>
                {withSource && (
                  <div role="gridcell" className={`${cell} font-mono text-[12.5px]`}>
                    {node.sourceVersion ? (
                      <button
                        type="button"
                        tabIndex={-1}
                        className="text-accent hover:underline"
                        onClick={(e) => {
                          e.stopPropagation();
                          select(node.sourceVersion!.line.id);
                        }}
                      >
                        {node.sourceVersion.number}
                      </button>
                    ) : (
                      <span className="text-text-3">—</span>
                    )}
                  </div>
                )}
                <div role="gridcell" className={`${cell} font-mono text-[12.5px]`}>
                  {node.latestVersion ? node.latestVersion.number : <span className="font-sans text-text-3">{t('lineage.noVersions')}</span>}
                </div>
              </div>
            );
          })}
        </div>
      </section>

      {selected && (
        <LinePanel
          line={selected}
          versions={panelVersions}
          log={log?.lineId === selected.id ? log.items : null}
          isOwner={isOwner}
          onClose={() => select('')}
          onSelectLine={select}
          onFork={setForkFrom}
          onRegister={() => setRegistering(true)}
          onEdit={() => setEditing(true)}
          onStatus={setStatusTarget}
        />
      )}

      {editing && selected && <EditLineDialog appKey={app.key} line={selected} onClose={() => setEditing(false)} onSaved={changed} />}
      {statusTarget && selected && (
        <LineStatusDialog appKey={app.key} line={selected} target={statusTarget} onClose={() => setStatusTarget(null)} onChanged={changed} />
      )}

      {forkFrom && (
        <ForkDialog
          appKey={app.key}
          versions={load.versions}
          lines={load.lines}
          sourceId={forkFrom}
          onClose={() => setForkFrom(null)}
          onForked={(line) => {
            setForkFrom(null);
            select(line.id);
            reload();
          }}
        />
      )}
      {registering && selected && (
        <RegisterVersionDialog
          appKey={app.key}
          line={selected}
          // Latest release date first: the first version of the line is its previous one.
          previous={panelVersions[0]}
          onClose={() => setRegistering(false)}
          onRegistered={() => {
            setRegistering(false);
            reload();
          }}
        />
      )}
    </div>
  );
}
