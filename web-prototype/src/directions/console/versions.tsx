import { GitFork, PackagePlus, Search, X } from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { allVersions, appVersions, can, childLines, lineById, useStore, type Application, type Version } from '../../data';
import { useT } from '../../i18n';
import { appPath, navigate, setParam } from '../../router';
import { Shell } from './shell';
import { Btn, Container, Empty, Input, KV, LinkBtn, Mono, Select, StatusText, useLineName } from './ui';
import { showing } from './list';

type Kind = '' | 'tag' | 'commit';

function Filters({ q, setQ, kind, setKind, children, onClear, active }: { q: string; setQ: (v: string) => void; kind: Kind; setKind: (k: Kind) => void; children?: ReactNode; onClear: () => void; active: boolean }) {
  const { t } = useT();
  return (
    <div className="cs-filters">
      <div className="cs-search">
        <Search size={16} aria-hidden />
        <Input aria-label={t('common.search')} placeholder={t('console.versions.search')} value={q} onChange={(e) => setQ(e.target.value)} />
      </div>
      {children}
      <Select className="cs-select-auto" aria-label={t('term.ref')} value={kind} onChange={(e) => setKind(e.target.value as Kind)}>
        <option value="">{t('console.filter.anyRef')}</option>
        <option value="tag">{t('ref.tag')}</option>
        <option value="commit">{t('ref.commit')}</option>
      </Select>
      {active && (
        <LinkBtn className="cs-small" onClick={onClear}>
          {t('console.filter.clear')}
        </LinkBtn>
      )}
    </div>
  );
}

// ---------------------------------------------------------------- Versions tab (per application)

export function VersionsTab({ app, selectedId, onRegister }: { app: Application; selectedId?: string; onRegister: () => void }) {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const lineName = useLineName();
  const [q, setQ] = useState('');
  const [kind, setKind] = useState<Kind>('');
  const [lineId, setLineId] = useState('');
  const lines = s.lines.filter((l) => l.appKey === app.key);
  const all = appVersions(s, app.key).slice().reverse();
  const needle = q.trim().toLowerCase();
  const rows = all.filter((v) => {
    if (lineId && v.lineId !== lineId) return false;
    if (kind && v.ref.kind !== kind) return false;
    if (!needle) return true;
    const l = lineById(v.lineId);
    return [v.number, v.ref.value, tx(v.note), l ? lineName(l) : ''].some((x) => x.toLowerCase().includes(needle));
  });
  const clear = () => {
    setQ('');
    setKind('');
    setLineId('');
  };
  const manage = can(s.role).manageLines;
  return (
    <Container
      flush
      title={`${t('term.versions')} (${all.length})`}
      actions={
        manage ? (
          <Btn kind="primary" onClick={onRegister}>
            <PackagePlus size={16} aria-hidden /> {t('console.registerVersion')}
          </Btn>
        ) : undefined
      }
    >
      <div style={{ padding: '0 20px' }}>
        <Filters q={q} setQ={setQ} kind={kind} setKind={setKind} onClear={clear} active={!!(q || kind || lineId)}>
          <Select className="cs-select-auto" aria-label={t('term.line')} value={lineId} onChange={(e) => setLineId(e.target.value)}>
            <option value="">{t('console.filter.allLines')}</option>
            {lines.map((l) => (
              <option key={l.id} value={l.id}>
                {lineName(l)}
              </option>
            ))}
          </Select>
        </Filters>
        <div className="cs-showing" role="status">
          {showing(t, rows.length, all.length)}
        </div>
      </div>
      <div style={{ overflowX: 'auto', borderTop: '1px solid var(--c-line)' }}>
        <table className="cs-table">
          <thead>
            <tr>
              <th style={{ minWidth: 130 }}>{t('term.version')}</th>
              <th style={{ minWidth: 180 }}>{t('term.line')}</th>
              <th style={{ minWidth: 170 }}>{t('term.ref')}</th>
              <th style={{ minWidth: 120 }}>{t('term.releasedAt')}</th>
              <th style={{ minWidth: 200 }}>{t('term.note')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((v) => {
              const l = lineById(v.lineId)!;
              return (
                <tr key={v.id} aria-selected={v.id === selectedId}>
                  <td className="nw">
                    <LinkBtn className="cs-mono" onClick={() => setParam('v', v.id)}>
                      {v.number}
                    </LinkBtn>
                  </td>
                  <td className="nw">{lineName(l)}</td>
                  <td className="nw">
                    <span className="cs-muted">{t(`ref.${v.ref.kind}`)}</span> <Mono>{v.ref.value}</Mono>
                  </td>
                  <td className="nw">{fmtDate(v.releasedAt)}</td>
                  <td style={{ maxWidth: 320 }}>
                    <span className="cs-clip" title={tx(v.note)}>
                      {tx(v.note) || '—'}
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {rows.length === 0 && (
          <Empty title={all.length === 0 ? t('console.versions.empty') : t('console.versions.noMatch')}>
            {all.length === 0 ? (manage ? t('console.versions.emptyHint') : null) : <LinkBtn onClick={clear}>{t('console.filter.clear')}</LinkBtn>}
          </Empty>
        )}
      </div>
    </Container>
  );
}

export function VersionPanel({ app, version, onFork }: { app: Application; version: Version; onFork: (id: string) => void }) {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const lineName = useLineName();
  const line = lineById(version.lineId);
  const children = childLines(s, version.id);
  const manage = can(s.role).manageLines;
  return (
    <aside className="cs-panel" aria-label={t('console.versionDetails')}>
      <div className="cs-panel-head">
        <div style={{ minWidth: 0 }}>
          <h2 className="cs-h2">
            <Mono className="cs-h2">{version.number}</Mono>
          </h2>
          <div className="cs-muted cs-small">{tx(app.name)}</div>
        </div>
        <button type="button" className="cs-iconbtn" aria-label={t('common.close')} onClick={() => setParam('v', null)}>
          <X size={18} aria-hidden />
        </button>
      </div>
      {manage && (
        <div className="cs-panel-sec">
          <Btn small onClick={() => onFork(version.id)}>
            <GitFork size={14} aria-hidden /> {t('console.forkFromVersion')}
          </Btn>
        </div>
      )}
      <div className="cs-panel-sec">
        <h3 className="cs-h3">{t('console.details')}</h3>
        <KV
          single
          rows={[
            { label: t('term.line'), value: line ? <LinkBtn onClick={() => navigate(`${appPath(app.key, 'lineage')}?line=${line.id}`)}>{lineName(line)}</LinkBtn> : '—' },
            { label: t('term.status'), value: line ? <StatusText status={line.status} /> : '—' },
            { label: t('term.ref'), value: <><span className="cs-muted">{t(`ref.${version.ref.kind}`)}</span> <Mono>{version.ref.value}</Mono></> },
            { label: t('term.releasedAt'), value: fmtDate(version.releasedAt) },
            { label: t('term.note'), value: tx(version.note) || '—' },
          ]}
        />
      </div>
      <div className="cs-panel-sec">
        <h3 className="cs-h3">{t('console.forkedFrom')}</h3>
        {children.length === 0 ? (
          <div className="cs-muted">{t('console.noForks')}</div>
        ) : (
          <ul className="cs-people">
            {children.map((c) => (
              <li key={c.id}>
                <LinkBtn onClick={() => navigate(`${appPath(app.key, 'lineage')}?line=${c.id}`)}>{lineName(c)}</LinkBtn> <Mono className="cs-muted">{c.branch}</Mono>
              </li>
            ))}
          </ul>
        )}
      </div>
    </aside>
  );
}

// ---------------------------------------------------------------- global Versions page

export function Versions() {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const lineName = useLineName();
  const [q, setQ] = useState('');
  const [kind, setKind] = useState<Kind>('');
  const [appKey, setAppKey] = useState('');
  const [lineKind, setLineKind] = useState<'' | 'main' | 'maint'>('');
  const all = allVersions(s);
  const needle = q.trim().toLowerCase();
  const rows = all.filter(({ version: v, line, app }) => {
    if (appKey && app.key !== appKey) return false;
    if (lineKind === 'main' && !line.isMain) return false;
    if (lineKind === 'maint' && line.isMain) return false;
    if (kind && v.ref.kind !== kind) return false;
    if (!needle) return true;
    return [v.number, v.ref.value, tx(v.note), lineName(line), tx(app.name), app.key].some((x) => x.toLowerCase().includes(needle));
  });
  const clear = () => {
    setQ('');
    setKind('');
    setAppKey('');
    setLineKind('');
  };
  return (
    <Shell>
      <div className="cs-titlebar">
        <h1 className="cs-h1">
          {t('term.versions')} <span className="cs-count">({all.length})</span>
        </h1>
      </div>
      <div>
        <Filters q={q} setQ={setQ} kind={kind} setKind={setKind} onClear={clear} active={!!(q || kind || appKey || lineKind)}>
          <Select className="cs-select-auto" aria-label={t('term.application')} value={appKey} onChange={(e) => setAppKey(e.target.value)}>
            <option value="">{t('console.filter.allApps')}</option>
            {s.apps.map((a) => (
              <option key={a.key} value={a.key}>
                {tx(a.name)}
              </option>
            ))}
          </Select>
          <Select className="cs-select-auto" aria-label={t('term.line')} value={lineKind} onChange={(e) => setLineKind(e.target.value as '' | 'main' | 'maint')}>
            <option value="">{t('console.filter.allLines')}</option>
            <option value="main">{t('term.mainline')}</option>
            <option value="maint">{t('term.lines')}</option>
          </Select>
        </Filters>
        <div className="cs-showing" role="status">
          {showing(t, rows.length, all.length)}
        </div>
        <div className="cs-tablewrap">
          <table className="cs-table">
            <thead>
              <tr>
                <th style={{ minWidth: 150 }}>{t('term.application')}</th>
                <th style={{ minWidth: 130 }}>{t('term.version')}</th>
                <th style={{ minWidth: 180 }}>{t('term.line')}</th>
                <th style={{ minWidth: 170 }}>{t('term.ref')}</th>
                <th style={{ minWidth: 120 }}>{t('term.releasedAt')}</th>
                <th style={{ minWidth: 200 }}>{t('term.note')}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map(({ version: v, line, app }) => (
                <tr key={v.id}>
                  <td className="nw">{tx(app.name)}</td>
                  <td className="nw">
                    <LinkBtn className="cs-mono" title={t('console.openInApp', { name: tx(app.name) })} onClick={() => navigate(`${appPath(app.key, 'versions')}?v=${v.id}`)}>
                      {v.number}
                    </LinkBtn>
                  </td>
                  <td className="nw">{lineName(line)}</td>
                  <td className="nw">
                    <span className="cs-muted">{t(`ref.${v.ref.kind}`)}</span> <Mono>{v.ref.value}</Mono>
                  </td>
                  <td className="nw">{fmtDate(v.releasedAt)}</td>
                  <td style={{ maxWidth: 320 }}>
                    <span className="cs-clip" title={tx(v.note)}>
                      {tx(v.note) || '—'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {rows.length === 0 && (
            <Empty title={all.length === 0 ? t('console.versions.emptyGlobal') : t('console.versions.noMatch')}>
              {all.length > 0 && <LinkBtn onClick={clear}>{t('console.filter.clear')}</LinkBtn>}
            </Empty>
          )}
        </div>
      </div>
    </Shell>
  );
}
