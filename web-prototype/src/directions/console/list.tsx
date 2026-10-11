import { Plus, Search, X } from 'lucide-react';
import { useState } from 'react';
import { can, latestVersion, useStore, userName, type Application } from '../../data';
import { useT } from '../../i18n';
import { appPath, navigate } from '../../router';
import { CreateAppDialog, DeleteAppDialog } from './dialogs';
import { Shell } from './shell';
import { Btn, Container, Empty, LinkBtn, Mono } from './ui';

export function showing(t: (k: string, p?: Record<string, string | number>) => string, shown: number, total: number) {
  return shown === 0 ? t('console.showingNone', { total }) : t('console.showing', { from: 1, to: shown, total });
}

export function RecentActivity() {
  const { t, tx, tm, fmtDateTime } = useT();
  const s = useStore();
  const kinds = new Set(['audit.fork', 'audit.registerVersion', 'audit.registerVersionMain']);
  const rows = s.audit
    .filter((a) => kinds.has(a.event.key))
    .sort((a, b) => b.at.localeCompare(a.at))
    .slice(0, 8);
  return (
    <Container title={t('console.recent')} flush>
      {rows.length === 0 ? (
        <Empty title={t('console.recent.empty')} />
      ) : (
        <div className="cs-tablewrap" style={{ border: 0, borderRadius: 0 }}>
          <table className="cs-table cs-recent">
            <thead>
              <tr>
                <th style={{ width: 170 }}>{t('console.time')}</th>
                <th style={{ width: 220 }}>{t('term.application')}</th>
                <th>{t('console.event')}</th>
                <th style={{ width: 160 }}>{t('console.by')}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((a, i) => {
                const app = s.apps.find((x) => x.key === a.appKey);
                return (
                  <tr key={i}>
                    <td className="nw">{fmtDateTime(a.at)}</td>
                    <td className="nw">
                      {app ? (
                        <LinkBtn onClick={() => navigate(appPath(app.key))}>{tx(app.name)}</LinkBtn>
                      ) : (
                        <Mono>{a.appKey}</Mono>
                      )}
                    </td>
                    <td>{tm(a.event)}</td>
                    <td className="nw">{userName(a.actor)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </Container>
  );
}

export function AppList() {
  const { t, tx } = useT();
  const s = useStore();
  const admin = can(s.role).admin;
  const [q, setQ] = useState('');
  const [mine, setMine] = useState(false);
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<Application | null>(null);

  const needle = q.trim().toLowerCase();
  const apps = s.apps.filter((a) => {
    if (mine && !a.owners.includes(s.me)) return false;
    if (!needle) return true;
    const platform = s.platforms.find((p) => p.id === a.repo.platformId);
    return [a.key, tx(a.name), tx(a.description), a.repo.path, platform?.type ?? '', ...a.owners.map(userName)].some((x) => x.toLowerCase().includes(needle));
  });

  return (
    <Shell>
      <div className="cs-titlebar">
        <h1 className="cs-h1">
          {t('term.applications')} <span className="cs-count">({s.apps.length})</span>
        </h1>
        {admin && (
          <div className="cs-actions">
            <Btn kind="primary" onClick={() => setCreating(true)}>
              <Plus size={16} aria-hidden /> {t('console.createApp.title')}
            </Btn>
          </div>
        )}
      </div>

      <div>
        <div className="cs-filters">
          <div className="cs-search">
            <Search size={16} aria-hidden />
            <input className="cs-input" aria-label={t('common.search')} placeholder={t('console.list.search')} value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          {mine ? (
            <span className="cs-chip">
              {t('console.filter.ownerMe')}
              <button type="button" aria-label={t('console.filter.remove', { name: t('console.filter.ownerMe') })} onClick={() => setMine(false)}>
                <X size={14} aria-hidden />
              </button>
            </span>
          ) : (
            <button type="button" className="cs-chip-add" onClick={() => setMine(true)}>
              <Plus size={14} aria-hidden /> {t('console.filter.add')}: {t('console.filter.ownerMe')}
            </button>
          )}
          {(mine || q) && (
            <LinkBtn
              className="cs-small"
              onClick={() => {
                setMine(false);
                setQ('');
              }}
            >
              {t('console.filter.clear')}
            </LinkBtn>
          )}
        </div>
        <div className="cs-showing" role="status">
          {showing(t, apps.length, s.apps.length)}
        </div>
        <div className="cs-tablewrap">
          <table className="cs-table">
            <thead>
              <tr>
                <th style={{ minWidth: 220 }}>{t('term.name')}</th>
                <th style={{ minWidth: 200 }}>{t('term.description')}</th>
                <th>{t('console.latestMainline')}</th>
                <th className="num" style={{ whiteSpace: 'nowrap' }}>{t('term.lines')}</th>
                <th style={{ minWidth: 130 }}>{t('term.owners')}</th>
                <th style={{ minWidth: 200 }}>{t('term.repository')}</th>
                {admin && <th />}
              </tr>
            </thead>
            <tbody>
              {apps.map((a) => {
                const main = s.lines.find((l) => l.appKey === a.key && l.isMain);
                const lv = main ? latestVersion(s, main.id) : undefined;
                const platform = s.platforms.find((p) => p.id === a.repo.platformId);
                const owners = a.owners.map(userName).join(', ');
                return (
                  <tr key={a.key}>
                    <td className="nw">
                      <LinkBtn style={{ fontWeight: 600 }} onClick={() => navigate(appPath(a.key))}>
                        {tx(a.name)}
                      </LinkBtn>{' '}
                      <Mono className="cs-muted">{a.key}</Mono>
                    </td>
                    <td style={{ maxWidth: 280 }}>
                      <span className="cs-clip" title={tx(a.description)}>
                        {tx(a.description)}
                      </span>
                    </td>
                    <td className="mono">{lv ? lv.number : '—'}</td>
                    <td className="num">{s.lines.filter((l) => l.appKey === a.key && !l.isMain).length}</td>
                    <td style={{ maxWidth: 190 }}>
                      <span className="cs-clip" title={owners}>
                        {owners}
                      </span>
                    </td>
                    <td className="nw">
                      {platform?.type} · <Mono>{a.repo.path}</Mono>
                    </td>
                    {admin && (
                      <td className="nw">
                        <LinkBtn onClick={() => setDeleting(a)}>{t('common.delete')}</LinkBtn>
                      </td>
                    )}
                  </tr>
                );
              })}
            </tbody>
          </table>
          {apps.length === 0 && (
            <Empty title={s.apps.length === 0 ? t('console.list.empty') : t('console.list.noMatch')}>
              {s.apps.length > 0 && (
                <LinkBtn
                  onClick={() => {
                    setMine(false);
                    setQ('');
                  }}
                >
                  {t('console.filter.clear')}
                </LinkBtn>
              )}
            </Empty>
          )}
        </div>
      </div>

      <RecentActivity />
      {creating && <CreateAppDialog onClose={() => setCreating(false)} />}
      {deleting && <DeleteAppDialog app={deleting} onClose={() => setDeleting(null)} />}
    </Shell>
  );
}
