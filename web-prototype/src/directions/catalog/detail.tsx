import { useState } from 'react';
import { GitFork, Pencil, Plus } from 'lucide-react';
import { actions, appLines, appVersions, can, latestVersion, useStore, userName, type Application } from '../../data';
import { useT } from '../../i18n';
import { appPath, navigate, type Tab } from '../../router';
import { DeleteAppDialog, EditAppDialog, ForkDialog, lineHref, RegisterVersionDialog } from './dialogs';
import { Lineage } from './lineage';
import { Shell } from './shell';
import { A, Btn, CopyButton, Empty, hostOf, Mono, OverflowMenu, Panel, Status, Textarea } from './ui';
import { VersionsTab } from './versions';

export type Ctx = { openFork: (versionId: string) => void; openRegister: (lineId: string) => void };

export function AppDetail({ appKey, tab }: { appKey: string; tab: Tab }) {
  const { t, tx } = useT();
  const s = useStore();
  const app = s.apps.find((a) => a.key === appKey);
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [fork, setFork] = useState<string | null>(null);
  const [register, setRegister] = useState<string | null>(null);

  if (!app) {
    return (
      <Shell crumbs={[{ label: t('term.applications'), to: '/' }, { label: appKey }]}>
        <Panel bodyClass={null} data-cg="not-found">
          <Empty title={t('catalog.appNotFound')}>
            <p style={{ marginBottom: 12 }}>{t('catalog.appNotFoundBody', { key: appKey })}</p>
            <A to="/">{t('catalog.backToList')}</A>
          </Empty>
        </Panel>
      </Shell>
    );
  }

  const perms = can(s.role);
  const lines = appLines(s, app.key);
  const main = lines.find((l) => l.isMain)!;
  const latest = latestVersion(s, main.id);
  const all = appVersions(s, app.key);
  const platform = s.platforms.find((p) => p.id === app.repo.platformId);
  const defaultSource = latest ?? all.at(-1);
  const ctx: Ctx = { openFork: setFork, openRegister: setRegister };

  const tabs: { id: Tab; label: string }[] = [
    { id: 'overview', label: t('term.overview') },
    { id: 'lineage', label: t('term.lineage') },
    { id: 'versions', label: t('term.versions') },
    { id: 'history', label: t('term.changeHistory') },
  ];
  const copyKey = async () => {
    try {
      await navigator.clipboard.writeText(app.key);
    } catch {
      /* best effort */
    }
  };

  return (
    <Shell crumbs={[{ label: t('term.applications'), to: '/' }, { label: tx(app.name), to: tab === 'overview' ? undefined : appPath(app.key) }, ...(tab === 'overview' ? [] : [{ label: tabs.find((x) => x.id === tab)!.label }])]}>
      <div className="cg-page-head" style={{ alignItems: 'flex-start', marginBottom: 0 }}>
        <div style={{ minWidth: 0 }}>
          <div className="cg-eyebrow">{t('term.application')}</div>
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 12, flexWrap: 'wrap' }}>
            <h1 className="cg-h1">{tx(app.name)}</h1>
            <Mono className="cg-grey" title={t('catalog.immutable')}>
              {app.key}
            </Mono>
          </div>
        </div>
        <div className="cg-actions" data-cg="detail-actions">
          {perms.manageLines && (
            <>
              <Btn onClick={() => defaultSource && setFork(defaultSource.id)} disabled={!defaultSource} title={defaultSource ? undefined : t('catalog.noVersionToFork')} data-cg="fork">
                <GitFork size={15} aria-hidden />
                {t('term.fork')}
              </Btn>
              <Btn onClick={() => setRegister(main.id)} data-cg="register">
                <Plus size={15} aria-hidden />
                {t('catalog.registerVersion')}
              </Btn>
            </>
          )}
          {perms.editApp && (
            <Btn onClick={() => setEditing(true)} data-cg="edit-app">
              <Pencil size={14} aria-hidden />
              {t('common.edit')}
            </Btn>
          )}
          <OverflowMenu
            label={t('catalog.moreActions')}
            items={[
              { label: t('catalog.copyKey'), onClick: copyKey, dataCg: 'copy-key' },
              ...(perms.admin ? [{ label: t('catalog.deleteAppShort'), onClick: () => setDeleting(true), danger: true, dataCg: 'delete-app' }] : []),
            ]}
          />
        </div>
      </div>

      <dl className="cg-strip" data-cg="strip">
        <div>
          <dt>{t('term.owners')}</dt>
          <dd>{app.owners.map(userName).join(', ')}</dd>
        </div>
        <div>
          <dt>{t('term.pes')}</dt>
          <dd>{app.pes.length ? app.pes.map(userName).join(', ') : t('common.none')}</dd>
        </div>
        <div>
          <dt>{t('term.codePlatform')}</dt>
          <dd>{platform?.type}</dd>
        </div>
        <div>
          <dt>{t('term.mainBranch')}</dt>
          <dd>
            <Mono>{app.mainBranch}</Mono>
          </dd>
        </div>
        <div>
          <dt>{t('catalog.latestVersion')}</dt>
          <dd>{latest ? <Mono>{latest.number}</Mono> : '—'}</dd>
        </div>
      </dl>

      <nav className="cg-tabs" aria-label={t('catalog.tabs')}>
        {tabs.map((x) => (
          <A key={x.id} to={appPath(app.key, x.id)} aria-current={tab === x.id ? 'page' : undefined} data-cg={`tab-${x.id}`}>
            {x.label}
          </A>
        ))}
      </nav>

      {tab === 'overview' && <Overview app={app} onEdit={() => setEditing(true)} />}
      {tab === 'lineage' && <Lineage app={app} ctx={ctx} />}
      {tab === 'versions' && <VersionsTab app={app} ctx={ctx} />}
      {tab === 'history' && <History app={app} />}

      {editing && <EditAppDialog app={app} onClose={() => setEditing(false)} />}
      {deleting && <DeleteAppDialog app={app} onClose={() => setDeleting(false)} />}
      {fork && <ForkDialog appKey={app.key} sourceVersionId={fork} onClose={() => setFork(null)} onCreated={(id) => navigate(lineHref(app.key, id))} />}
      {register && <RegisterVersionDialog appKey={app.key} lineId={register} onClose={() => setRegister(null)} />}
    </Shell>
  );
}

// ---------------------------------------------------------------- overview

function Overview({ app, onEdit }: { app: Application; onEdit: () => void }) {
  const { t, tx, fmtDate } = useT();
  const s = useStore();
  const perms = can(s.role);
  const platform = s.platforms.find((p) => p.id === app.repo.platformId);
  const lines = appLines(s, app.key);
  const main = lines.find((l) => l.isMain)!;
  const latest = latestVersion(s, main.id);
  const maint = lines.filter((l) => !l.isMain);
  const [editDeploy, setEditDeploy] = useState(false);
  const [deployText, setDeployText] = useState('');
  const person = (u: string) => {
    const user = s.users.find((x) => x.username === u);
    return (
      <div className="cg-person" key={u}>
        {userName(u)}
        {user?.email && <small>{user.email}</small>}
      </div>
    );
  };

  return (
    <div className="cg-grid-2" data-cg="overview">
      <Panel title={t('catalog.about')} actions={perms.editApp && <Btn sm onClick={onEdit} data-cg="about-edit">{t('common.edit')}</Btn>}>
        <dl className="cg-kv">
          <div>
            <dt>{t('term.applicationKey')}</dt>
            <dd style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <Mono>{app.key}</Mono>
              <CopyButton text={app.key} label={`${t('common.copy')} ${t('term.applicationKey')}`} />
            </dd>
          </div>
          <div>
            <dt>{t('term.name')}</dt>
            <dd>{tx(app.name)}</dd>
          </div>
          <div>
            <dt>{t('term.mainBranch')}</dt>
            <dd>
              <Mono>{app.mainBranch}</Mono>
            </dd>
          </div>
          <div className="span-3">
            <dt>{t('term.description')}</dt>
            <dd>{tx(app.description) || '—'}</dd>
          </div>
          <div>
            <dt>{t('term.codePlatform')}</dt>
            <dd>
              {platform?.type}
              <div className="cg-meta">{platform ? hostOf(platform.url) : ''}</div>
            </dd>
          </div>
          <div className="span-2">
            <dt>{t('term.repository')}</dt>
            <dd style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
              <Mono>{app.repo.path}</Mono>
              <CopyButton text={app.repo.path} label={`${t('common.copy')} ${t('term.repository')}`} />
              <span className="cg-meta" style={{ marginLeft: 8 }}>
                {t('term.projectId')} <Mono>{app.repo.projectId}</Mono>
              </span>
            </dd>
          </div>
          <div className="span-2">
            <dt>{t('term.owners')}</dt>
            <dd style={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 8 }}>{app.owners.map(person)}</dd>
          </div>
          <div>
            <dt>{t('term.pes')}</dt>
            <dd>{app.pes.length ? app.pes.map(person) : <span className="cg-grey">{t('common.none')}</span>}</dd>
          </div>
        </dl>
      </Panel>

      <div>
        <Panel
          title={t('term.deployInfo')}
          actions={
            perms.editDeploy &&
            !editDeploy && (
              <Btn
                sm
                onClick={() => {
                  setDeployText(tx(app.deployInfo));
                  setEditDeploy(true);
                }}
                data-cg="deploy-edit"
              >
                {t('common.edit')}
              </Btn>
            )
          }
          data-cg="deploy"
        >
          {editDeploy ? (
            <div style={{ display: 'grid', gap: 10 }}>
              <Textarea rows={6} value={deployText} onChange={(e) => setDeployText(e.target.value)} aria-label={t('term.deployInfo')} autoFocus data-cg="deploy-text" />
              <div className="cg-actions">
                <Btn
                  kind="primary"
                  sm
                  onClick={() => {
                    actions.setDeployInfo(app.key, deployText);
                    setEditDeploy(false);
                  }}
                  data-cg="deploy-save"
                >
                  {t('common.save')}
                </Btn>
                <Btn sm onClick={() => setEditDeploy(false)} data-cg="deploy-cancel">
                  {t('common.cancel')}
                </Btn>
              </div>
            </div>
          ) : tx(app.deployInfo) ? (
            <div className="cg-pre" data-cg="deploy-body">{tx(app.deployInfo)}</div>
          ) : (
            <p className="cg-note">{t('catalog.noDeployInfo')}</p>
          )}
          {!perms.editDeploy && <p className="cg-hint" style={{ marginTop: 12 }}>{t('catalog.deployReadOnly')}</p>}
        </Panel>

        <Panel title={t('term.lines')} actions={<A to={appPath(app.key, 'lineage')} data-cg="open-lineage">{t('catalog.openLineage')}</A>} data-cg="lines-summary">
          <dl className="cg-kv-rows">
            <dt>{t('catalog.latestMainline')}</dt>
            <dd>
              {latest ? (
                <>
                  <Mono>{latest.number}</Mono> <span className="cg-meta">· {fmtDate(latest.releasedAt)}</span>
                </>
              ) : (
                '—'
              )}
            </dd>
            {(['Active', 'Frozen', 'Retired'] as const).map((st) => (
              <div key={st} style={{ display: 'contents' }}>
                <dt>
                  <Status status={st} />
                </dt>
                <dd data-cg={`count-${st}`}>{maint.filter((l) => l.status === st).length}</dd>
              </div>
            ))}
          </dl>
          {maint.length === 0 && <p className="cg-hint" style={{ marginTop: 12 }}>{t('catalog.noLinesYet')}</p>}
        </Panel>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------- history

function History({ app }: { app: Application }) {
  const { t, tm, fmtDateTime } = useT();
  const s = useStore();
  const rows = s.audit.filter((a) => a.appKey === app.key).sort((a, b) => b.at.localeCompare(a.at));
  return (
    <Panel title={t('term.changeHistory')} bodyClass={null} data-cg="history">
      {rows.length === 0 ? (
        <Empty title={t('catalog.noHistory')} />
      ) : (
        <table className="cg-table">
          <thead>
            <tr>
              <th style={{ width: 190 }}>{t('catalog.time')}</th>
              <th style={{ width: 200 }}>{t('catalog.actor')}</th>
              <th>{t('catalog.event')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r, i) => (
              <tr key={i} data-cg="history-row">
                <td className="cg-nowrap">{fmtDateTime(r.at)}</td>
                <td>{userName(r.actor)}</td>
                <td>{tm(r.event)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Panel>
  );
}
