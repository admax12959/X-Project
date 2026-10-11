// Application list: search by name, key, repository path or Owner; administrators create applications here.
// #12 adds the "my applications" filter, latest version, line counts and recent activity.
import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { ApplicationSummary } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Empty, Input } from '../ui/controls.tsx';
import { repoLabel } from '../ui/labels.ts';
import { CreateApplicationDialog } from './CreateApplicationDialog.tsx';

// Errors are kept raw and rendered at render time, so they follow a language switch.
type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; items: ApplicationSummary[] };

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
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    let live = true;
    api.applications.list(query).then(
      (r) => live && setLoad({ status: 'ok', items: r.items }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, [query]);

  const items = load.status === 'ok' ? load.items : [];
  const th = 'h-10 whitespace-nowrap border-b border-line bg-thead px-3 text-left text-[13px] font-semibold';
  const td = 'h-10 border-b border-sep px-3';

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

      <Input
        type="search"
        aria-label={t('apps.search.label')}
        placeholder={t('apps.search.placeholder')}
        value={q}
        onChange={(e) => setQ(e.target.value)}
        className="max-w-md"
      />

      <div className="overflow-x-auto rounded border border-line bg-white">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('term.name')}</th>
              <th className={th}>{t('term.applicationKey')}</th>
              <th className={th}>{t('term.owners')}</th>
              <th className={th}>{t('term.repository')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((a) => (
              <tr key={a.key} className="hover:bg-[#fafbfc]">
                <td className={td}>
                  <Link to={`/apps/${encodeURIComponent(a.key)}`} className="font-semibold text-accent hover:underline">
                    {a.name}
                  </Link>
                </td>
                <td className={`${td} font-mono text-[12.5px]`}>{a.key}</td>
                <td className={td}>{a.owners.map((o) => o.displayName).join(', ')}</td>
                <td className={`${td} font-mono text-[12.5px]`}>{repoLabel(a.repo)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && <Empty title={errorsByField(load.error)[''] ?? t('apps.loadFailed')} />}
        {load.status === 'ok' && items.length === 0 && query !== '' && <Empty title={t('apps.noMatch.title')}>{t('apps.noMatch.body')}</Empty>}
        {load.status === 'ok' && items.length === 0 && query === '' && (
          <Empty title={t('apps.empty.title')}>{user.isAdmin ? t('apps.empty.bodyAdmin') : t('apps.empty.body')}</Empty>
        )}
      </div>

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
