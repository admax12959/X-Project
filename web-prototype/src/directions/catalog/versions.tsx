import { useState } from 'react';
import { GitFork, Plus } from 'lucide-react';
import { allVersions, can, childLines, lineageOrder, lineById, lineVersions, useStore, type Application, type Line, type Version } from '../../data';
import { useT } from '../../i18n';
import { setParam, useLocation } from '../../router';
import type { Ctx } from './detail';
import { lineHref, versionHref } from './dialogs';
import { Shell } from './shell';
import { A, Btn, CopyButton, Empty, Mono, Panel, SearchBox, Select, Status } from './ui';

type Kind = '' | 'tag' | 'commit';

const matches = (v: Version, kind: Kind, q: string, extra: string[] = []) => {
  if (kind && v.ref.kind !== kind) return false;
  const needle = q.trim().toLowerCase();
  return !needle || [v.number, v.ref.value, ...extra].some((x) => x.toLowerCase().includes(needle));
};

// ---------------------------------------------------------------- application Versions tab

export function VersionsTab({ app, ctx }: { app: Application; ctx: Ctx }) {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const perms = can(s.role);
  const { params } = useLocation();
  const [lineId, setLineId] = useState('');
  const [kind, setKind] = useState<Kind>('');
  const [q, setQ] = useState('');

  const order = lineageOrder(s, app.key);
  const groups = order
    .map(({ line }) => ({ line, items: lineVersions(s, line.id).filter((v) => (!lineId || lineId === line.id) && matches(v, kind, q, [tx(v.note)])).reverse() }))
    .filter((g) => g.items.length > 0);
  const flat = groups.flatMap((g) => g.items);
  const total = order.reduce((n, { line }) => n + lineVersions(s, line.id).length, 0);
  const wanted = params.get('v');
  const selected = flat.find((v) => v.id === wanted) ?? flat[0];
  const filtersActive = !!(lineId || kind || q);
  const reset = () => {
    setLineId('');
    setKind('');
    setQ('');
  };
  const name = (l: Line) => (l.isMain ? t('term.mainline') : tx(l.name));

  return (
    <div data-cg="versions-tab">
      <Panel bodyClass={null}>
        <div className="cg-filterbar" style={{ borderBottom: 0 }}>
          <Select aria-label={t('term.line')} value={lineId} onChange={(e) => setLineId(e.target.value)} data-cg="vf-line">
            <option value="">{t('catalog.allLines')}</option>
            {order.map(({ line }) => (
              <option key={line.id} value={line.id}>
                {name(line)}
              </option>
            ))}
          </Select>
          <Select aria-label={t('term.ref')} value={kind} onChange={(e) => setKind(e.target.value as Kind)} data-cg="vf-kind">
            <option value="">{t('catalog.tagOrCommit')}</option>
            <option value="tag">{t('ref.tag')}</option>
            <option value="commit">{t('ref.commit')}</option>
          </Select>
          <SearchBox value={q} onChange={setQ} placeholder={t('catalog.searchVersions')} dataCg="vf-search" />
          <span className="cg-meta" style={{ marginLeft: 'auto' }} data-cg="vf-count">
            {t('n.versions', { n: flat.length })}
            {filtersActive && ` / ${total}`}
          </span>
          {perms.manageLines && (
            <Btn kind="primary" onClick={() => ctx.openRegister(selected ? selected.lineId : order[0].line.id)} data-cg="register-tab">
              <Plus size={15} aria-hidden />
              {t('catalog.registerVersion')}
            </Btn>
          )}
        </div>
      </Panel>

      {flat.length === 0 ? (
        <Panel bodyClass={null} data-cg="versions-empty">
          <Empty title={total === 0 ? t('catalog.noVersions') : t('catalog.noMatch')}>
            {filtersActive && (
              <Btn sm onClick={reset} data-cg="clear-filters">
                {t('catalog.clearFilters')}
              </Btn>
            )}
          </Empty>
        </Panel>
      ) : (
        <div style={{ display: 'grid', gridTemplateColumns: '300px minmax(0, 1fr)', gap: 16, alignItems: 'start', marginTop: 16 }}>
          <Panel bodyClass={null} className="cg-sticky" data-cg="version-index">
            <div className="cg-vlist">
              {groups.map((g) => (
                <div key={g.line.id}>
                  <div className="cg-vgroup">
                    <span>{name(g.line)}</span>
                    {!g.line.isMain && <Status status={g.line.status} size={13} />}
                  </div>
                  {g.items.map((v) => (
                    <button key={v.id} type="button" className="cg-vitem" aria-current={selected?.id === v.id} onClick={() => setParam('v', v.id)} data-cg="version-item" data-version={v.number}>
                      <Mono>{v.number}</Mono>
                      <span className="cg-meta">{fmtDate(v.releasedAt)}</span>
                    </button>
                  ))}
                </div>
              ))}
            </div>
          </Panel>
          {selected && <VersionCard app={app} version={selected} ctx={ctx} />}
        </div>
      )}
    </div>
  );
}

function VersionCard({ app, version, ctx }: { app: Application; version: Version; ctx: Ctx }) {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const perms = can(s.role);
  const line = lineById(version.lineId)!;
  const kids = childLines(s, version.id);
  const branch = line.isMain ? app.mainBranch : line.branch;
  return (
    <Panel bodyClass={null} data-cg="version-card" data-version={version.number}>
      <div className="cg-panel-b">
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, alignItems: 'flex-start' }}>
          <div style={{ minWidth: 0 }}>
            <div className="cg-vbig" data-cg="version-number">{version.number}</div>
            <div className="cg-meta" style={{ marginTop: 4, display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
              <A to={lineHref(app.key, line.id)} data-cg="version-line-link">
                {line.isMain ? t('term.mainline') : tx(line.name)}
              </A>
              {!line.isMain && <Status status={line.status} size={13} />}
            </div>
          </div>
          {perms.manageLines && (
            <Btn onClick={() => ctx.openFork(version.id)} data-cg="fork-from-version">
              <GitFork size={15} aria-hidden />
              {t('catalog.forkFromVersion')}
            </Btn>
          )}
        </div>

        <dl className="cg-kv-rows" style={{ marginTop: 20 }}>
          <dt>{t('term.ref')}</dt>
          <dd>
            <span className="cg-ref">
              <span className="cg-meta">{t(`ref.${version.ref.kind}`)}</span>
              <Mono>{version.ref.value}</Mono>
              <CopyButton text={version.ref.value} label={`${t('common.copy')} ${t(`ref.${version.ref.kind}`)}`} />
            </span>
          </dd>
          <dt>{t('term.releasedAt')}</dt>
          <dd>{fmtDate(version.releasedAt)}</dd>
          <dt>{t('term.branch')}</dt>
          <dd>
            <span className="cg-branch">{branch}</span>
          </dd>
          <dt>{t('term.note')}</dt>
          <dd className="cg-pre">{tx(version.note) || '—'}</dd>
        </dl>

        <div className="cg-sec" data-cg="forked-from">
          <h3>{t('catalog.linesForked')}</h3>
          {kids.length === 0 ? (
            <p className="cg-note">{t('catalog.noForks')}</p>
          ) : (
            <div>
              {kids.map((c) => (
                <div key={c.id} style={{ display: 'flex', gap: 12, alignItems: 'center', padding: '4px 0' }}>
                  <A to={lineHref(app.key, c.id)} data-cg="forked-line">
                    {tx(c.name)}
                  </A>
                  <Status status={c.status} size={13} />
                  <span className="cg-branch">{c.branch}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </Panel>
  );
}

// ---------------------------------------------------------------- global Versions page

export function Versions() {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const [appKey, setAppKey] = useState('');
  const [lineKind, setLineKind] = useState<'' | 'main' | 'maint'>('');
  const [kind, setKind] = useState<Kind>('');
  const [q, setQ] = useState('');

  const all = allVersions(s);
  const rows = all.filter(({ version, line, app }) => (!appKey || app.key === appKey) && (!lineKind || (lineKind === 'main') === line.isMain) && matches(version, kind, q, [tx(app.name), app.key, tx(line.name)]));
  const filtersActive = !!(appKey || lineKind || kind || q);
  const reset = () => {
    setAppKey('');
    setLineKind('');
    setKind('');
    setQ('');
  };

  return (
    <Shell crumbs={[{ label: t('term.versions') }]}>
      <div className="cg-page-head">
        <h1 className="cg-h1">{t('term.versions')}</h1>
      </div>
      <Panel title={`${t('catalog.allVersions')} (${rows.length})`} bodyClass={null} data-cg="global-versions">
        <div className="cg-filterbar">
          <Select aria-label={t('term.application')} value={appKey} onChange={(e) => setAppKey(e.target.value)} data-cg="gv-app">
            <option value="">{t('catalog.allApplications')}</option>
            {s.apps.map((a) => (
              <option key={a.key} value={a.key}>
                {tx(a.name)}
              </option>
            ))}
          </Select>
          <Select aria-label={t('term.line')} value={lineKind} onChange={(e) => setLineKind(e.target.value as '' | 'main' | 'maint')} data-cg="gv-line">
            <option value="">{t('catalog.mainlineAndLines')}</option>
            <option value="main">{t('term.mainline')}</option>
            <option value="maint">{t('term.lines')}</option>
          </Select>
          <Select aria-label={t('term.ref')} value={kind} onChange={(e) => setKind(e.target.value as Kind)} data-cg="gv-kind">
            <option value="">{t('catalog.tagOrCommit')}</option>
            <option value="tag">{t('ref.tag')}</option>
            <option value="commit">{t('ref.commit')}</option>
          </Select>
          <SearchBox value={q} onChange={setQ} placeholder={t('catalog.searchVersions')} dataCg="gv-search" />
        </div>
        {rows.length === 0 ? (
          <Empty title={all.length === 0 ? t('catalog.noVersions') : t('catalog.noMatch')}>
            {filtersActive && (
              <Btn sm onClick={reset} data-cg="clear-filters">
                {t('catalog.clearFilters')}
              </Btn>
            )}
          </Empty>
        ) : (
          <div>
            {rows.map(({ version, line, app }) => (
              <A key={version.id} to={versionHref(app.key, version.id)} className="cg-vrow" style={{ padding: '8px 16px', minHeight: 52 }} data-cg="global-version-row" data-version={version.number} data-app={app.key}>
                <span style={{ minWidth: 0 }}>
                  <span style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
                    <Mono className="">{version.number}</Mono>
                    <span style={{ color: 'var(--cg-text-2)' }}>{tx(app.name)}</span>
                  </span>
                  <span className="cg-meta" style={{ display: 'flex', gap: 8, flexWrap: 'wrap', alignItems: 'baseline' }}>
                    <span>{line.isMain ? t('term.mainline') : tx(line.name)}</span>
                    <span>·</span>
                    <span className="cg-mono" style={{ fontSize: 12 }}>
                      {t(`ref.${version.ref.kind}`)} {version.ref.value}
                    </span>
                    <span>·</span>
                    <span>{fmtDate(version.releasedAt)}</span>
                  </span>
                </span>
                {!line.isMain && <Status status={line.status} size={13} />}
              </A>
            ))}
          </div>
        )}
      </Panel>
    </Shell>
  );
}

