import { Ellipsis, GitFork, PackagePlus, Pencil, Trash2 } from 'lucide-react';
import { Menu } from '@base-ui/react/menu';
import { useState } from 'react';
import { appVersions, can, latestVersion, lineageOrder, lineById, lineVersions, useStore, versionById, type Line, type LineStatus } from '../../data';
import { useT } from '../../i18n';
import { appPath, navigate, useLocation, TABS, type Tab } from '../../router';
import { DeleteAppDialog, DeployDialog, EditAppDialog, ForkDialog, RegisterVersionDialog, StatusDialog } from './dialogs';
import { LinePanel, LineageTab } from './lineage';
import { Shell } from './shell';
import { Cmd, Container, CopyButton, Empty, KV, LinkBtn, Mono, StatusText, useLineName } from './ui';
import { VersionPanel, VersionsTab } from './versions';

type Dlg = { kind: 'edit' | 'delete' | 'deploy' } | { kind: 'fork'; version: string } | { kind: 'register'; line: string } | { kind: 'status'; line: Line; to: LineStatus };

export function AppDetail({ appKey, tab }: { appKey: string; tab: Tab }) {
  const { t, tx } = useT();
  const s = useStore();
  const { params } = useLocation();
  const app = s.apps.find((a) => a.key === appKey);
  const [dlg, setDlg] = useState<Dlg | null>(null);

  if (!app)
    return (
      <Shell>
        <div className="cs-container">
          <Empty title={t('console.notFound')}>
            <LinkBtn onClick={() => navigate('/')}>{t('console.backToList')}</LinkBtn>
          </Empty>
        </div>
      </Shell>
    );

  const role = can(s.role);
  const lineParam = params.get('line');
  const vParam = params.get('v');
  const hl = params.get('hv') ?? undefined;
  const selLine = tab === 'lineage' && lineParam ? s.lines.find((l) => l.id === lineParam && l.appKey === app.key) : undefined;
  const selVersion = tab === 'versions' && vParam ? versionById(vParam) : undefined;
  const selVersionOk = selVersion && lineById(selVersion.lineId)?.appKey === app.key ? selVersion : undefined;

  const main = s.lines.find((l) => l.appKey === app.key && l.isMain)!;
  const all = appVersions(s, app.key);
  const defaultSource = (selLine ? lineVersions(s, selLine.id).at(-1) : undefined) ?? latestVersion(s, main.id) ?? all.at(-1);
  const defaultLine = selLine && selLine.status !== 'Retired' ? selLine.id : main.id;

  const select = (lineId: string, versionId?: string) => {
    const url = new URL(location.href);
    url.searchParams.set('line', lineId);
    if (versionId) url.searchParams.set('hv', versionId);
    else url.searchParams.delete('hv');
    navigate(url.pathname + url.search, { replace: true });
  };

  const panel = selLine ? (
    <LinePanel
      key={selLine.id}
      app={app}
      line={selLine}
      highlight={hl}
      onFork={(version) => setDlg({ kind: 'fork', version })}
      onRegister={(line) => setDlg({ kind: 'register', line })}
      onStatus={(line, to) => setDlg({ kind: 'status', line, to })}
    />
  ) : selVersionOk ? (
    <VersionPanel key={selVersionOk.id} app={app} version={selVersionOk} onFork={(version) => setDlg({ kind: 'fork', version })} />
  ) : undefined;

  const tabLabel: Record<Tab, string> = { overview: t('term.overview'), lineage: t('term.lineage'), versions: t('term.versions'), history: t('term.changeHistory') };
  const hasCmd = role.manageLines || role.editApp || role.admin;

  return (
    <Shell panel={panel}>
      <div>
        <nav className="cs-breadcrumb" aria-label="breadcrumb">
          <LinkBtn onClick={() => navigate('/')}>{t('term.applications')}</LinkBtn>
          <span aria-hidden>/</span>
          <span>{tx(app.name)}</span>
        </nav>
        <h1 className="cs-h1">
          {tx(app.name)} <span className="cs-key">{app.key}</span>
        </h1>
        {hasCmd && (
          <div className="cs-cmdbar" role="toolbar" aria-label={t('console.actions')}>
            {role.manageLines && (
              <>
                <Cmd
                  icon={<GitFork size={16} aria-hidden />}
                  disabled={!defaultSource}
                  title={!defaultSource ? t('console.noSourceVersion') : undefined}
                  onClick={() => defaultSource && setDlg({ kind: 'fork', version: defaultSource.id })}
                >
                  {t('term.fork')}
                </Cmd>
                <Cmd icon={<PackagePlus size={16} aria-hidden />} onClick={() => setDlg({ kind: 'register', line: defaultLine })}>
                  {t('console.registerVersion')}
                </Cmd>
              </>
            )}
            {role.editApp && (
              <Cmd icon={<Pencil size={16} aria-hidden />} onClick={() => setDlg({ kind: 'edit' })}>
                {t('common.edit')}
              </Cmd>
            )}
            {role.admin && (
              <Menu.Root>
                <Menu.Trigger className="cs-cmd" aria-label={t('console.moreActions')}>
                  <Ellipsis size={16} aria-hidden /> {t('console.more')}
                </Menu.Trigger>
                <Menu.Portal>
                  <Menu.Positioner align="start" sideOffset={4} style={{ zIndex: 60 }}>
                    <Menu.Popup className="cs-menu">
                      <Menu.Item className="cs-menu-item danger" onClick={() => setDlg({ kind: 'delete' })}>
                        <Trash2 size={16} aria-hidden /> {t('console.deleteApp.menu')}
                      </Menu.Item>
                    </Menu.Popup>
                  </Menu.Positioner>
                </Menu.Portal>
              </Menu.Root>
            )}
          </div>
        )}
      </div>

      <div className="cs-tabs" role="tablist" aria-label={tx(app.name)} style={{ marginTop: 20 }}>
        {TABS.map((x) => (
          <button key={x} type="button" role="tab" id={`tab-${x}`} aria-selected={tab === x} aria-controls="tabpanel" className="cs-tab" onClick={() => navigate(appPath(app.key, x))}>
            {tabLabel[x]}
          </button>
        ))}
      </div>

      <div id="tabpanel" className="cs-stack" role="tabpanel" aria-labelledby={`tab-${tab}`} style={{ marginTop: 20 }}>
        {tab === 'overview' && <Overview appKey={app.key} onDeploy={() => setDlg({ kind: 'deploy' })} />}
        {tab === 'lineage' && <LineageTab app={app} selectedId={selLine?.id} onSelect={select} />}
        {tab === 'versions' && <VersionsTab app={app} selectedId={selVersionOk?.id} onRegister={() => setDlg({ kind: 'register', line: main.id })} />}
        {tab === 'history' && <History appKey={app.key} />}
      </div>

      {dlg?.kind === 'edit' && <EditAppDialog app={app} onClose={() => setDlg(null)} />}
      {dlg?.kind === 'delete' && <DeleteAppDialog app={app} onClose={() => setDlg(null)} />}
      {dlg?.kind === 'deploy' && <DeployDialog app={app} onClose={() => setDlg(null)} />}
      {dlg?.kind === 'fork' && (
        <ForkDialog
          appKey={app.key}
          sourceVersionId={dlg.version}
          onClose={() => setDlg(null)}
          onCreated={(id) => navigate(`${appPath(app.key, 'lineage')}?line=${encodeURIComponent(id)}`)}
        />
      )}
      {dlg?.kind === 'register' && <RegisterVersionDialog appKey={app.key} initialLineId={dlg.line} onClose={() => setDlg(null)} />}
      {dlg?.kind === 'status' && <StatusDialog line={dlg.line} to={dlg.to} onClose={() => setDlg(null)} />}
    </Shell>
  );
}

// ---------------------------------------------------------------- Overview

function Overview({ appKey, onDeploy }: { appKey: string; onDeploy: () => void }) {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const lineName = useLineName();
  const app = s.apps.find((a) => a.key === appKey)!;
  const role = can(s.role);
  const platform = s.platforms.find((p) => p.id === app.repo.platformId);
  const lines = lineageOrder(s, app.key).map((x) => x.line);
  const counts: Record<LineStatus, number> = { Active: 0, Frozen: 0, Retired: 0 };
  lines.filter((l) => !l.isMain).forEach((l) => counts[l.status]++);
  const people = (names: string[]) =>
    names.length === 0 ? (
      <span className="cs-muted">{t('common.none')}</span>
    ) : (
      <ul className="cs-people">
        {names.map((u) => {
          const user = s.users.find((x) => x.username === u);
          return (
            <li key={u}>
              {user ? tx(user.displayName) : u} <span className="cs-muted">{user?.email}</span>
              {user?.status === 'Disabled' && <span className="cs-muted"> ({t('userStatus.Disabled')})</span>}
            </li>
          );
        })}
      </ul>
    );
  const deploy = tx(app.deployInfo);
  const main = lines.find((l) => l.isMain)!;
  const mainLatest = latestVersion(s, main.id);

  return (
    <>
      <Container title={t('console.essentials')}>
        <KV
          rows={[
            {
              label: t('term.applicationKey'),
              value: (
                <>
                  <Mono>{app.key}</Mono> <CopyButton value={app.key} />
                </>
              ),
            },
            { label: t('term.codePlatform'), value: platform ? <>{platform.type} · <Mono>{platform.url}</Mono></> : '—' },
            { label: t('term.description'), value: tx(app.description) || '—' },
            { label: t('term.repository'), value: <Mono>{app.repo.path}</Mono> },
            { label: t('term.mainBranch'), value: <Mono>{app.mainBranch}</Mono> },
            { label: t('term.projectId'), value: <Mono>{app.repo.projectId}</Mono> },
            { label: t('term.owners'), value: people(app.owners) },
            { label: t('term.pes'), value: people(app.pes) },
          ]}
        />
      </Container>

      <Container
        title={t('term.deployInfo')}
        actions={
          role.editDeploy ? (
            <button type="button" className="cs-btn cs-btn-sm" onClick={onDeploy}>
              {t('common.edit')}
            </button>
          ) : undefined
        }
      >
        {deploy ? <pre className="cs-pre">{deploy}</pre> : <span className="cs-muted">{t('console.deploy.empty')}</span>}
      </Container>

      <Container
        flush
        title={t('term.lines')}
        actions={<LinkBtn onClick={() => navigate(appPath(app.key, 'lineage'))}>{t('console.openLineage')}</LinkBtn>}
      >
        <div style={{ display: 'flex', gap: 28, flexWrap: 'wrap', padding: '0 20px 14px', alignItems: 'center' }}>
          {(['Active', 'Frozen', 'Retired'] as const).map((st) => (
            <span key={st} style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}>
              <StatusText status={st} /> <b>{counts[st]}</b>
            </span>
          ))}
          <span className="cs-muted cs-small">
            {t('console.mainlineLatest')}: {mainLatest ? <Mono>{mainLatest.number}</Mono> : '—'}
            {mainLatest ? ` (${fmtDate(mainLatest.releasedAt)})` : ''}
          </span>
        </div>
        <div style={{ overflowX: 'auto', borderTop: '1px solid var(--c-line)' }}>
          <table className="cs-table">
            <thead>
              <tr>
                <th style={{ minWidth: 200 }}>{t('term.name')}</th>
                <th style={{ minWidth: 110 }}>{t('term.status')}</th>
                <th style={{ minWidth: 160 }}>{t('console.latestVersion')}</th>
              </tr>
            </thead>
            <tbody>
              {lines.map((l) => {
                const lv = latestVersion(s, l.id);
                return (
                  <tr key={l.id}>
                    <td className="nw">
                      <LinkBtn onClick={() => navigate(`${appPath(app.key, 'lineage')}?line=${encodeURIComponent(l.id)}`)}>{lineName(l)}</LinkBtn>
                    </td>
                    <td>
                      <StatusText status={l.status} />
                    </td>
                    <td className="nw">
                      {lv ? (
                        <>
                          <Mono>{lv.number}</Mono> <span className="cs-muted cs-small">{fmtDate(lv.releasedAt)}</span>
                        </>
                      ) : (
                        <span className="cs-muted">{t('console.noVersionsYet')}</span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </Container>
    </>
  );
}

// ---------------------------------------------------------------- History

function History({ appKey }: { appKey: string }) {
  const { t, tx, tm, fmtDateTime } = useT();
  const s = useStore();
  const rows = s.audit.filter((a) => a.appKey === appKey).sort((a, b) => b.at.localeCompare(a.at));
  const name = (u: string) => s.users.find((x) => x.username === u)?.displayName;
  return (
    <Container flush title={`${t('term.changeHistory')} (${rows.length})`}>
      {rows.length === 0 ? (
        <Empty title={t('console.history.empty')} />
      ) : (
        <div style={{ overflowX: 'auto', borderTop: '1px solid var(--c-line)' }}>
          <table className="cs-table">
            <thead>
              <tr>
                <th style={{ width: 190 }}>{t('console.time')}</th>
                <th style={{ width: 180 }}>{t('console.by')}</th>
                <th>{t('console.event')}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((a, i) => (
                <tr key={i}>
                  <td className="nw">{fmtDateTime(a.at)}</td>
                  <td className="nw">{name(a.actor) ? tx(name(a.actor)!) : a.actor}</td>
                  <td>{tm(a.event)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Container>
  );
}
