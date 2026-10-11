import { useMemo, useState } from 'react';
import { CircleCheck, CircleMinus, CirclePause, FolderGit, Plus } from 'lucide-react';
import { appLines, can, latestVersion, lineVersions, useStore, userName, type Application, type Line, type LineStatus } from '../../data';
import { useT } from '../../i18n';
import { appPath } from '../../router';
import { CreateAppDialog, DeleteAppDialog, lineHref } from './dialogs';
import { Shell } from './shell';
import { A, Btn, Empty, Mono, OverflowMenu, Panel, SearchBox, Select } from './ui';

const ICON = { Active: CircleCheck, Frozen: CirclePause, Retired: CircleMinus } as const;
const STATUSES: LineStatus[] = ['Active', 'Frozen', 'Retired'];

export function AppList() {
  const { t, tx } = useT();
  const s = useStore();
  const admin = can(s.role).admin;
  const [q, setQ] = useState('');
  const [owned, setOwned] = useState(false);
  const [platform, setPlatform] = useState('');
  const [owner, setOwner] = useState('');
  const [status, setStatus] = useState('');
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<Application | null>(null);

  const ownedCount = s.apps.filter((a) => a.owners.includes(s.me)).length;
  const owners = [...new Set(s.apps.flatMap((a) => a.owners))];

  const rows = useMemo(() => {
    const needle = q.trim().toLowerCase();
    return s.apps
      .filter((a) => !owned || a.owners.includes(s.me))
      .filter((a) => !platform || a.repo.platformId === platform)
      .filter((a) => !owner || a.owners.includes(owner))
      .filter((a) => !status || appLines(s, a.key).some((l) => !l.isMain && l.status === status))
      .filter((a) => !needle || [a.key, tx(a.name), tx(a.description), ...a.owners.map(userName)].some((x) => x.toLowerCase().includes(needle)))
      .map((a) => {
        const lines = appLines(s, a.key);
        const main = lines.find((l) => l.isMain)!;
        const maint = lines.filter((l) => !l.isMain);
        return { app: a, main, latest: latestVersion(s, main.id), maint, lines };
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [s, q, owned, platform, owner, status, t]);

  // Frozen lines, and active/frozen lines with no versions registered yet. No dates, no countdowns.
  type Attn = { app: Application; line: Line; why: 'frozen' | 'empty' };
  const attention: Attn[] = rows.flatMap(({ app, maint }) =>
    maint.flatMap((l): Attn[] => {
      const empty = lineVersions(s, l.id).length === 0;
      if (l.status === 'Frozen') return [{ app, line: l, why: 'frozen' as const }];
      if (empty && l.status !== 'Retired') return [{ app, line: l, why: 'empty' as const }];
      return [];
    }),
  );

  const filtersActive = !!(q || owned || platform || owner || status);
  const reset = () => {
    setQ('');
    setOwned(false);
    setPlatform('');
    setOwner('');
    setStatus('');
  };

  return (
    <Shell crumbs={[{ label: t('term.applications') }]}>
      <div className="cg-page-head">
        <h1 className="cg-h1">{t('catalog.catalogTitle')}</h1>
        {admin && (
          <Btn kind="primary" onClick={() => setCreating(true)} data-cg="create-app-open">
            <Plus size={15} aria-hidden />
            {t('catalog.createApp')}
          </Btn>
        )}
      </div>

      <div className="cg-list-grid">
        <aside className="cg-rail" aria-label={t('catalog.filters')}>
          <div className="cg-rail-group" role="group" aria-label={t('catalog.quickFilters')}>
            <button type="button" aria-pressed={owned} onClick={() => setOwned(true)} data-cg="filter-owned">
              <span>{t('catalog.ownedByMe')}</span>
              <span className="cg-count">{ownedCount}</span>
            </button>
            <button type="button" aria-pressed={!owned} onClick={() => setOwned(false)} data-cg="filter-all">
              <span>{t('catalog.allApplications')}</span>
              <span className="cg-count">{s.apps.length}</span>
            </button>
          </div>
          <div className="cg-panel" style={{ padding: 12, display: 'grid', gap: 12 }}>
            <label className="cg-field">
              <span className="cg-label">{t('term.codePlatform')}</span>
              <Select value={platform} onChange={(e) => setPlatform(e.target.value)} data-cg="filter-platform">
                <option value="">{t('common.all')}</option>
                {s.platforms.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.type} · {p.url.replace(/^https?:\/\//, '')}
                  </option>
                ))}
              </Select>
            </label>
            <label className="cg-field">
              <span className="cg-label">{t('catalog.owner')}</span>
              <Select value={owner} onChange={(e) => setOwner(e.target.value)} data-cg="filter-owner">
                <option value="">{t('common.all')}</option>
                {owners.map((u) => (
                  <option key={u} value={u}>
                    {userName(u)}
                  </option>
                ))}
              </Select>
            </label>
            <label className="cg-field">
              <span className="cg-label">{t('catalog.lineStatus')}</span>
              <Select value={status} onChange={(e) => setStatus(e.target.value)} data-cg="filter-status">
                <option value="">{t('common.all')}</option>
                {STATUSES.map((x) => (
                  <option key={x} value={x}>
                    {t(`status.${x}`)}
                  </option>
                ))}
              </Select>
              <span className="cg-hint">{t('catalog.lineStatusHint')}</span>
            </label>
          </div>
        </aside>

        <div style={{ minWidth: 0 }}>
          <Panel
            title={`${t('term.applications')} (${rows.length})`}
            actions={<SearchBox value={q} onChange={setQ} placeholder={t('catalog.searchApps')} />}
            bodyClass={null}
            data-cg="app-list"
          >
            {rows.length === 0 ? (
              <Empty title={filtersActive ? t('catalog.noMatch') : t('catalog.noApps')}>
                {filtersActive ? (
                  <Btn sm onClick={reset} data-cg="clear-filters">
                    {t('catalog.clearFilters')}
                  </Btn>
                ) : (
                  <p>{t('catalog.noAppsBody')}</p>
                )}
              </Empty>
            ) : (
              <table className="cg-table cg-fixed">
                <colgroup>
                  <col />
                  <col style={{ width: 126 }} />
                  <col style={{ width: 92 }} />
                  <col style={{ width: 124 }} />
                  <col style={{ width: 252 }} />
                  {admin && <col style={{ width: 52 }} />}
                </colgroup>
                <thead>
                  <tr>
                    <th>{t('term.name')}</th>
                    <th>{t('term.owners')}</th>
                    <th>{t('catalog.latestVersion')}</th>
                    <th>{t('term.lines')}</th>
                    <th>{t('term.repository')}</th>
                    {admin && (
                      <th>
                        <span className="sr-only">{t('catalog.actions')}</span>
                      </th>
                    )}
                  </tr>
                </thead>
                <tbody>
                  {rows.map(({ app, latest, maint }) => {
                    const p = s.platforms.find((x) => x.id === app.repo.platformId);
                    return (
                      <tr key={app.key} data-cg="app-row" data-key={app.key}>
                        <td>
                          <div className="cg-name">
                            <A to={appPath(app.key)}>{tx(app.name)}</A>
                            <Mono className="cg-grey">{app.key}</Mono>
                          </div>
                          <div className="cg-meta cg-ellipsis" title={tx(app.description)}>
                            {tx(app.description) || <span className="cg-grey">—</span>}
                          </div>
                        </td>
                        <td className="cg-note" style={{ color: 'var(--cg-text)' }}>
                          {app.owners.map(userName).join(', ')}
                        </td>
                        <td>{latest ? <Mono>{latest.number}</Mono> : <span className="cg-grey">—</span>}</td>
                        <td>
                          <div>{maint.length}</div>
                          {maint.length > 0 && <Summary lines={maint} />}
                        </td>
                        <td>
                          <div className="cg-ellipsis" title={app.repo.path} style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                            <FolderGit size={14} aria-hidden style={{ flex: 'none', color: 'var(--cg-text-3)' }} />
                            <Mono className="cg-ellipsis" title={app.repo.path}>
                              {app.repo.path}
                            </Mono>
                          </div>
                          <div className="cg-meta">{p?.type}</div>
                        </td>
                        {admin && (
                          <td style={{ textAlign: 'right' }}>
                            <OverflowMenu label={t('catalog.moreActions')} dataCg="row-menu" items={[{ label: t('catalog.deleteAppShort'), danger: true, onClick: () => setDeleting(app), dataCg: 'row-delete' }]} />
                          </td>
                        )}
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </Panel>

          {attention.length > 0 && (
            <Panel title={t('catalog.needsAttention')} bodyClass={null} data-cg="attention">
              <div>
                {attention.map(({ app, line, why }) => (
                  <div key={line.id} className="cg-attn-row" data-cg="attention-row">
                    <div>
                      <A to={appPath(app.key)} style={{ fontWeight: 600 }}>
                        {tx(app.name)}
                      </A>
                    </div>
                    <div className="cg-name">
                      <span>{tx(line.name)}</span>
                      <Mono className="cg-grey">{line.branch}</Mono>
                    </div>
                    <div className="cg-note">{why === 'frozen' ? <WhyFrozen line={line} /> : t('catalog.whyEmpty')}</div>
                    <A to={lineHref(app.key, line.id)} className="cg-nowrap" data-cg="attention-link">
                      {t('catalog.openInLineage')}
                    </A>
                  </div>
                ))}
              </div>
            </Panel>
          )}
        </div>
      </div>

      {creating && <CreateAppDialog onClose={() => setCreating(false)} />}
      {deleting && <DeleteAppDialog app={deleting} onClose={() => setDeleting(null)} />}
    </Shell>
  );
}

function WhyFrozen({ line }: { line: Line }) {
  const { t } = useT();
  return (
    <span className="cg-status" data-s={line.status}>
      <CirclePause size={15} aria-hidden />
      {t('catalog.whyFrozen')}
    </span>
  );
}

function Summary({ lines }: { lines: Line[] }) {
  const { t } = useT();
  return (
    <span className="cg-sum">
      {STATUSES.map((st) => {
        const n = lines.filter((l) => l.status === st).length;
        if (!n) return null;
        const Icon = ICON[st];
        return (
          <span key={st} className="cg-status" data-s={st} title={`${t(`status.${st}`)}: ${n}`} aria-label={`${t(`status.${st}`)}: ${n}`} style={{ fontSize: 12 }}>
            <Icon size={13} aria-hidden />
            {n}
          </span>
        );
      })}
    </span>
  );
}
