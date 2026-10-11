// Overview tab: essentials, Owners/PEs and deployment info, with the maintenance actions the
// current user may use (the server enforces the same rules: spec #2 permission matrix).
// #8 adds the maintenance line summary.
import { useState } from 'react';
import { useNavigate } from 'react-router';
import type { Member } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Container, KV } from '../ui/controls.tsx';
import { PLATFORM_LABEL } from '../ui/labels.ts';
import { DeleteApplicationDialog, EditApplicationDialog, EditDeployInfoDialog, EditMembersDialog } from './ApplicationDialogs.tsx';
import { useApplication, useSetApplication } from './ApplicationPage.tsx';

function People({ list }: { list: Member[] }) {
  const { t } = useT();
  if (list.length === 0) return <span className="text-text-3">{t('apps.overview.none')}</span>;
  return (
    <ul>
      {list.map((m) => (
        <li key={m.id}>
          {m.displayName} <span className="font-mono text-[12.5px] text-text-3">{m.username}</span>
          {m.status === 'disabled' && <span className="text-text-3"> {t('apps.disabledSuffix')}</span>}
        </li>
      ))}
    </ul>
  );
}

type Dialog = null | 'info' | 'members' | 'deploy' | 'delete';

export function ApplicationOverview() {
  const { t } = useT();
  const app = useApplication();
  const setApp = useSetApplication();
  const me = useCurrentUser();
  const navigate = useNavigate();
  const [dialog, setDialog] = useState<Dialog>(null);
  const mono = 'font-mono text-[12.5px]';

  const isOwner = app.owners.some((m) => m.id === me.id);
  const isPe = app.pes.some((m) => m.id === me.id);
  const canEditInfo = isOwner;
  const canEditMembers = isOwner || me.isAdmin; // ADR-0003
  const canEditDeploy = isOwner || isPe;
  const canDelete = me.isAdmin;
  const saved = (a: typeof app) => {
    setApp(a);
    setDialog(null);
  };

  return (
    <div className="space-y-5">
      <Container
        title={t('term.essentials')}
        actions={
          <>
            {canEditInfo && (
              <Button small onClick={() => setDialog('info')}>
                {t('apps.edit')}
              </Button>
            )}
            {canEditMembers && (
              <Button small onClick={() => setDialog('members')}>
                {t('apps.editMembers')}
              </Button>
            )}
            {canDelete && (
              <Button small kind="danger" onClick={() => setDialog('delete')}>
                {t('apps.delete')}
              </Button>
            )}
          </>
        }
      >
        <KV
          rows={[
            { label: t('term.applicationKey'), value: <span className={mono}>{app.key}</span> },
            { label: t('term.description'), value: app.description ? <span className="whitespace-pre-wrap">{app.description}</span> : <span className="text-text-3">—</span> },
            {
              label: t('term.codePlatform'),
              value: (
                <>
                  {PLATFORM_LABEL[app.repo.kind]} <span className={`${mono} text-text-3`}>{app.repo.baseUrl}</span>
                </>
              ),
            },
            { label: t('term.repository'), value: <span className={mono}>{app.repo.path}</span> },
            { label: t('term.projectId'), value: <span className={mono}>{app.repo.projectId}</span> },
            { label: t('term.mainlineBranch'), value: <span className={mono}>{app.mainBranch}</span> },
            { label: t('term.owners'), value: <People list={app.owners} /> },
            { label: t('term.pes'), value: <People list={app.pes} /> },
          ]}
        />
      </Container>
      <Container
        title={t('term.deploymentInfo')}
        actions={
          canEditDeploy && (
            <Button small onClick={() => setDialog('deploy')}>
              {t('apps.edit')}
            </Button>
          )
        }
      >
        {app.deployInfo ? <p className="whitespace-pre-wrap">{app.deployInfo}</p> : <p className="text-text-3">{t('apps.overview.noDeployInfo')}</p>}
      </Container>

      {dialog === 'info' && <EditApplicationDialog app={app} onClose={() => setDialog(null)} onSaved={saved} />}
      {dialog === 'members' && <EditMembersDialog app={app} onClose={() => setDialog(null)} onSaved={saved} />}
      {dialog === 'deploy' && <EditDeployInfoDialog app={app} onClose={() => setDialog(null)} onSaved={saved} />}
      {dialog === 'delete' && <DeleteApplicationDialog app={app} onClose={() => setDialog(null)} onDeleted={() => navigate('/')} />}
    </div>
  );
}
