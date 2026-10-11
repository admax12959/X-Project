// Home page: the application list (search by name, key, repository path or Owner; "my applications"
// filter; latest version and maintenance line count) with the recent activity below it.
// Administrators create applications here. There is nothing about planned end dates on this page.
import { useEffect, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { ApplicationListItem } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { lineInSentence } from '../i18n/lines.ts';
import { Button, Empty, Input, Segmented } from '../ui/controls.tsx';
import { repoLabel } from '../ui/labels.ts';
import { appPath } from '../ui/links.ts';
import { CreateApplicationDialog } from './CreateApplicationDialog.tsx';
import { RecentActivity } from './RecentActivity.tsx';
import { th, td } from '../ui/table.ts';

// Errors are kept raw and rendered at render time, so they follow a language switch.
type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; items: ApplicationListItem[] };

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setV(value), ms);
    return () => clearTimeout(id);
  }, [value, ms]);
  return v;
}

export function ApplicationsPage() {
  const { t } = useT();
  const user = useCurrentUser();
  const navigate = useNavigate();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const [q, setQ] = useState('');
  const query = useDebounced(q.trim(), 250);
  // The filter is in the URL (?mine=true), so a reload or a shared link keeps it.
  const [params, setParams] = useSearchParams();
  const mine = params.get('mine') === 'true';
  const setMine = (value: boolean) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (value) next.set('mine', 'true');
        else next.delete('mine');
        return next;
      },
      { replace: true },
    );
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    let live = true;
    api.applications.list(query, mine).then(
      (r) => live && setLoad({ status: 'ok', items: r.items }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, [query, mine]);

  const items = load.status === 'ok' ? load.items : [];

  return (
    <>
      <div className="flex min-h-8 items-center justify-between gap-4">
        <h1 className="flex items-baseline gap-2 text-2xl font-bold">
          {t('term.applications')}
          {load.status === 'ok' && <span className="font-normal text-text-3">({items.length})</span>}
        </h1>
        {user.isAdmin && (
          <Button kind="primary" onClick={() => setCreating(true)}>
            {t('apps.create')}
          </Button>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Input
          type="search"
          aria-label={t('apps.search.label')}
          placeholder={t('apps.search.placeholder')}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          className="max-w-md"
        />
        <Segmented
          label={t('apps.filter.label')}
          value={mine ? 'mine' : 'all'}
          onChange={(v) => setMine(v === 'mine')}
          options={[
            { value: 'all', label: t('apps.filter.all') },
            { value: 'mine', label: t('apps.filter.mine') },
          ]}
        />
      </div>

      <div className="overflow-x-auto rounded border border-line bg-white">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('term.name')}</th>
              <th className={th}>{t('term.applicationKey')}</th>
              <th className={th}>{t('term.latestVersion')}</th>
              <th className={`${th} text-right`}>{t('term.maintenanceLines')}</th>
              <th className={th}>{t('term.owners')}</th>
              <th className={th}>{t('term.repository')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((a) => (
              <tr key={a.key} className="hover:bg-hover">
                <td className={td}>
                  <Link to={appPath(a.key)} className="font-semibold text-accent hover:underline">
                    {a.name}
                  </Link>
                </td>
                <td className={`${td} font-mono text-[12.5px]`}>{a.key}</td>
                <td className={`${td} whitespace-nowrap`}>
                  {a.latestVersion ? (
                    <span title={t('apps.latest.title', { date: a.latestVersion.releasedAt, line: lineInSentence(a.latestVersion.line) })}>
                      <span className="font-mono text-[12.5px]">{a.latestVersion.number}</span>{' '}
                      <span className="text-text-3">{a.latestVersion.releasedAt}</span>
                    </span>
                  ) : (
                    <span className="text-text-3">{t('apps.latest.none')}</span>
                  )}
                </td>
                <td className={`${td} text-right tabular-nums`}>{a.lineCount}</td>
                <td className={td}>{a.owners.map((o) => o.displayName).join(', ')}</td>
                <td className={`${td} font-mono text-[12.5px]`}>{repoLabel(a.repo)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && <Empty title={errorsByField(load.error)[''] ?? t('apps.loadFailed')} />}
        {load.status === 'ok' && items.length === 0 && query === '' && mine && (
          <Empty title={t('apps.mine.empty.title')}>{t('apps.mine.empty.body')}</Empty>
        )}
        {load.status === 'ok' && items.length === 0 && query !== '' && <Empty title={t('apps.noMatch.title')}>{t('apps.noMatch.body')}</Empty>}
        {load.status === 'ok' && items.length === 0 && query === '' && !mine && (
          <Empty title={t('apps.empty.title')}>{user.isAdmin ? t('apps.empty.bodyAdmin') : t('apps.empty.body')}</Empty>
        )}
      </div>

      <RecentActivity />

      {creating && (
        <CreateApplicationDialog
          onClose={() => setCreating(false)}
          onCreated={(a) => {
            setCreating(false);
            navigate(`/apps/${encodeURIComponent(a.key)}`);
          }}
        />
      )}
    </>
  );
}
