// Overview tab: essentials, Owners/PEs and deployment info. #7 adds the maintenance line summary.
import type { Member } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Container, KV } from '../ui/controls.tsx';
import { PLATFORM_LABEL } from '../ui/labels.ts';
import { useApplication } from './ApplicationPage.tsx';

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

export function ApplicationOverview() {
  const { t } = useT();
  const app = useApplication();
  const mono = 'font-mono text-[12.5px]';
  return (
    <div className="space-y-5">
      <Container title={t('term.essentials')}>
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
      <Container title={t('term.deploymentInfo')}>
        {app.deployInfo ? <p className="whitespace-pre-wrap">{app.deployInfo}</p> : <p className="text-text-3">{t('apps.overview.noDeployInfo')}</p>}
      </Container>
    </div>
  );
}
