// Global Versions page: every application's versions, latest release date first, with the
// application and line of each. Filter by application, search by version number, paginated.
// The filters and the page live in the URL (?app=&q=&page=), so a view can be shared.
import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { ApplicationSummary, VersionPage } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button, Empty, Input, Select } from '../ui/controls.tsx';
import { appPath, linePath } from '../ui/links.ts';

const PAGE_SIZE = 20;

// Errors are kept raw and rendered at render time, so they follow a language switch.
type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; data: VersionPage };

export function VersionsPage() {
  const { t } = useT();
  const [params, setParams] = useSearchParams();
  const app = params.get('app') ?? '';
  const q = params.get('q') ?? '';
  const pageParam = Number(params.get('page'));
  const page = Number.isInteger(pageParam) && pageParam >= 1 ? pageParam : 1;

  const [apps, setApps] = useState<ApplicationSummary[]>([]);
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const [qInput, setQInput] = useState(q);

  // Changing a filter goes back to the first page.
  const update = (changes: Record<string, string>) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        for (const [k, v] of Object.entries({ page: '', ...changes })) {
          if (v) next.set(k, v);
          else next.delete(k);
        }
        return next;
      },
      { replace: true },
    );

  useEffect(() => {
    let live = true;
    api.applications.list().then(
      (r) => live && setApps(r.items),
      () => undefined, // the filter then only offers "all"; the list reports its own errors
    );
    return () => {
      live = false;
    };
  }, []);

  // The search box types freely and updates the URL after a pause.
  useEffect(() => {
    if (qInput.trim() === q) return;
    const id = setTimeout(() => update({ q: qInput.trim() }), 250);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [qInput, q]);

  useEffect(() => {
    let live = true;
    setLoad((prev) => (prev.status === 'ok' ? prev : { status: 'loading' })); // keep the rows while the next page loads
    api.versions.list({ app, q, page, pageSize: PAGE_SIZE }).then(
      (data) => live && setLoad({ status: 'ok', data }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, [app, q, page]);

  const data = load.status === 'ok' ? load.data : undefined;
  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const filtered = app !== '' || q !== '';
  const th = 'h-10 whitespace-nowrap border-b border-line bg-thead px-3 text-left text-[13px] font-semibold';
  const td = 'border-b border-sep px-3 py-2 align-top';
  const link = 'text-accent hover:underline';

  return (
    <>
      <div className="space-y-1">
        <h1 className="flex items-baseline gap-2 text-2xl font-bold">
          {t('term.versions')}
          {data && <span className="font-normal text-text-3">({total})</span>}
        </h1>
        <p className="text-text-2">{t('versions.global.subtitle')}</p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Input
          type="search"
          aria-label={t('versions.global.search.label')}
          placeholder={t('versions.global.search.placeholder')}
          value={qInput}
          onChange={(e) => setQInput(e.target.value)}
          className="w-64"
        />
        <Select aria-label={t('versions.global.filter.app')} className="w-56" value={app} onChange={(e) => update({ app: e.target.value })}>
          <option value="">{t('versions.global.filter.allApps')}</option>
          {app !== '' && !apps.some((a) => a.key === app) && <option value={app}>{app}</option>}
          {apps.map((a) => (
            <option key={a.key} value={a.key}>
              {a.name} · {a.key}
            </option>
          ))}
        </Select>
        {filtered && (
          <Button
            small
            onClick={() => {
              setQInput('');
              update({ app: '', q: '' });
            }}
          >
            {t('versions.filter.clear')}
          </Button>
        )}
        <span className="flex-1" />
        {data && total > 0 && (
          <span className="text-[13px] text-text-3">
            {t('versions.global.range', { from: (data.page - 1) * data.pageSize + 1, to: (data.page - 1) * data.pageSize + items.length, total })}
          </span>
        )}
      </div>

      <div className="overflow-x-auto rounded border border-line bg-white">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('versions.col.version')}</th>
              <th className={th}>{t('versions.global.col.application')}</th>
              <th className={th}>{t('versions.col.line')}</th>
              <th className={th}>{t('versions.col.ref')}</th>
              <th className={th}>{t('versions.col.released')}</th>
              <th className={th}>{t('versions.col.note')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((v) => (
              <tr key={v.id}>
                <td className={`${td} whitespace-nowrap font-mono text-[12.5px]`}>{v.number}</td>
                <td className={td}>
                  <Link to={appPath(v.application.key)} className={`${link} font-semibold`}>
                    {v.application.name}
                  </Link>{' '}
                  <span className="font-mono text-[12.5px] text-text-3">{v.application.key}</span>
                </td>
                <td className={td}>
                  <Link to={linePath(v.application.key, v.lineId)} className={link}>
                    {v.line.isMain ? t('term.mainline') : v.line.name}
                  </Link>{' '}
                  <span className="font-mono text-[12.5px] text-text-3">{v.line.branch}</span>
                </td>
                <td className={`${td} whitespace-nowrap`}>
                  <span className="text-text-3">{t(v.ref.kind === 'tag' ? 'versions.ref.tag' : 'versions.ref.commit')}</span>{' '}
                  <span className="font-mono text-[12.5px]">{v.ref.value}</span>
                </td>
                <td className={`${td} whitespace-nowrap`}>{v.releasedAt}</td>
                <td className={`${td} whitespace-pre-wrap`}>{v.note}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && <Empty title={errorsByField(load.error)[''] ?? t('versions.loadFailed')} />}
        {data && items.length === 0 && total > 0 && (
          // A page number past the end (an old link, or the list shrank).
          <Empty title={t('versions.global.pageOf', { page, pages })}>
            <Button small onClick={() => update({})}>
              {t('versions.global.first')}
            </Button>
          </Empty>
        )}
        {data && items.length === 0 && total === 0 && filtered && <Empty title={t('versions.noMatch')} />}
        {data && items.length === 0 && total === 0 && !filtered &&<Empty title={t('versions.global.empty.title')}>{t('versions.global.empty.body')}</Empty>}
      </div>

      {data && total > PAGE_SIZE && (
        <nav className="flex items-center justify-end gap-2" aria-label={t('versions.global.pageOf', { page, pages })}>
          <span className="text-[13px] text-text-3">{t('versions.global.pageOf', { page, pages })}</span>
          <Button small disabled={page <= 1} onClick={() => update({ page: String(page - 1) })}>
            {t('versions.global.prev')}
          </Button>
          <Button small disabled={page >= pages} onClick={() => update({ page: String(page + 1) })}>
            {t('versions.global.next')}
          </Button>
        </nav>
      )}
    </>
  );
}
