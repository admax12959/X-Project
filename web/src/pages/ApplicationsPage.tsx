// Application list. #6 adds creation and rows; #12 adds search, "my applications", latest version, recent activity.
import { useEffect, useState } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { ApplicationSummary } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Empty } from '../ui/controls.tsx';

// Errors are kept raw and rendered at render time, so they follow a language switch.
type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; items: ApplicationSummary[] };

export function ApplicationsPage() {
  const { t } = useT();
  const user = useCurrentUser();
  const [load, setLoad] = useState<Load>({ status: 'loading' });

  useEffect(() => {
    let live = true;
    api.applications.list().then(
      (r) => live && setLoad({ status: 'ok', items: r.items }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, []);

  const items = load.status === 'ok' ? load.items : [];
  const th = 'h-10 whitespace-nowrap border-b border-line bg-thead px-3 text-left text-[13px] font-semibold';

  return (
    <>
      <div className="flex min-h-8 items-center justify-between gap-4">
        <h1 className="flex items-baseline gap-2 text-2xl font-bold">
          {t('term.applications')}
          {load.status === 'ok' && <span className="font-normal text-text-3">({items.length})</span>}
        </h1>
      </div>

      <div className="overflow-x-auto rounded border border-line bg-white">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('term.name')}</th>
              <th className={th}>{t('term.applicationKey')}</th>
              <th className={th}>{t('term.description')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((a) => (
              <tr key={a.key} className="hover:bg-[#fafbfc]">
                <td className="h-10 border-b border-sep px-3">{a.name}</td>
                <td className="h-10 border-b border-sep px-3 font-mono text-[12.5px]">{a.key}</td>
                <td className="h-10 border-b border-sep px-3">{a.description}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && <Empty title={errorsByField(load.error)[''] ?? t('apps.loadFailed')} />}
        {load.status === 'ok' && items.length === 0 && (
          <Empty title={t('apps.empty.title')}>{user.isAdmin ? t('apps.empty.bodyAdmin') : t('apps.empty.body')}</Empty>
        )}
      </div>
    </>
  );
}
