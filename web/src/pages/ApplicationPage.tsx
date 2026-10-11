// Application page: header and tab strip shared by the tabs (routes under /apps/:key).
// Tabs are child routes in main.tsx; later tickets add Lineage (#7) and Versions (#10)
// by adding a <Route> there and an entry in TABS below. Tabs read the application
// through useApplication().
import { useEffect, useState } from 'react';
import { Link, NavLink, Outlet, useOutletContext, useParams } from 'react-router';
import { api, ApiError } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { Application } from '../api/types.ts';
import { useT, type MessageKey } from '../i18n/index.ts';
import { Empty } from '../ui/controls.tsx';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; app: Application };

const TABS: { to: string; label: MessageKey; end?: boolean }[] = [
  { to: '', label: 'term.overview', end: true },
  { to: 'history', label: 'term.changeHistory' },
];

export function useApplication(): Application {
  return useOutletContext<{ app: Application }>().app;
}

export function ApplicationPage() {
  const { t } = useT();
  const { key = '' } = useParams();
  const [load, setLoad] = useState<Load>({ status: 'loading' });

  useEffect(() => {
    let live = true;
    setLoad({ status: 'loading' });
    api.applications.get(key).then(
      (app) => live && setLoad({ status: 'ok', app }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, [key]);

  if (load.status === 'loading') return <Empty title={t('common.loading')} />;
  if (load.status === 'error') {
    const missing = load.error instanceof ApiError && load.error.status === 404;
    return (
      <Empty title={missing ? t('apps.notFound.title') : (errorsByField(load.error)[''] ?? t('apps.loadFailed'))}>
        <Link to="/" className="text-accent hover:underline">
          {t('notFound.back')}
        </Link>
      </Empty>
    );
  }

  const { app } = load;
  return (
    <>
      <nav aria-label="breadcrumb" className="text-[13px] text-text-3">
        <Link to="/" className="text-accent hover:underline">
          {t('term.applications')}
        </Link>
        <span aria-hidden> / </span>
        {app.name}
      </nav>
      <h1 className="flex items-baseline gap-3 text-2xl font-bold">
        {app.name}
        <span className="font-mono text-base font-normal text-text-3">{app.key}</span>
      </h1>
      <div role="navigation" aria-label={t('apps.tabs')} className="flex gap-1 border-b border-line">
        {TABS.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to}
            end={tab.end}
            className={({ isActive }) =>
              `-mb-px border-b-2 px-3 py-2 ${isActive ? 'border-accent font-semibold text-accent' : 'border-transparent text-text hover:text-accent'}`
            }
          >
            {t(tab.label)}
          </NavLink>
        ))}
      </div>
      <Outlet context={{ app }} />
    </>
  );
}
