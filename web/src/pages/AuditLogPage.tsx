// Administration > Audit log (#27): identity and security related operations, newest first. Filter by operator,
// object (a user or an application), action and date range; paginated. Administrators only.
// The filters and the page live in the URL (?actor=&user=&app=&action=&from=&to=&page=), so a view can be shared.
import { useEffect, useState } from 'react';
import { Navigate, useSearchParams } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { AdminUser, AuditPage, AuditQuery } from '../api/types.ts';
import { auditActionName, auditActions, auditOperatorName, renderAuditEvent } from '../audit/render.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Empty, Input, Select } from '../ui/controls.tsx';
import { th, tdTop as td } from '../ui/table.ts';

const PAGE_SIZE = 20;

// Errors are kept raw and rendered at render time, so they follow a language switch.
type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; data: AuditPage };

/** The page is for administrators; anyone else is sent home (the API also answers 403). */
export function AuditLogPage() {
  const user = useCurrentUser();
  if (!user.isAdmin) return <Navigate to="/" replace />;
  return <AuditLog />;
}

/** A date input value (YYYY-MM-DD) as the start or the end of that day in the browser's time zone. */
function dayBound(date: string, end: boolean): string | undefined {
  if (!date) return undefined;
  const d = new Date(`${date}T${end ? '23:59:59.999' : '00:00:00.000'}`);
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString();
}

function AuditLog() {
  const { t, locale } = useT();
  const [params, setParams] = useSearchParams();
  const actor = params.get('actor') ?? '';
  const userId = params.get('user') ?? '';
  const app = params.get('app') ?? '';
  const action = params.get('action') ?? '';
  const from = params.get('from') ?? '';
  const to = params.get('to') ?? '';
  const pageParam = Number(params.get('page'));
  const page = Number.isInteger(pageParam) && pageParam >= 1 ? pageParam : 1;

  const [users, setUsers] = useState<AdminUser[]>([]);
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const [appInput, setAppInput] = useState(app);
  const [attempt, setAttempt] = useState(0); // bumped by Retry

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
    api.users.list().then(
      (r) => live && setUsers(r.items),
      () => undefined, // the filters then only offer "all"; the list reports its own errors
    );
    return () => {
      live = false;
    };
  }, []);

  // The application key box types freely and updates the URL after a pause.
  useEffect(() => {
    if (appInput.trim() === app) return;
    const id = setTimeout(() => update({ app: appInput.trim(), user: '' }), 250);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [appInput, app]);

  useEffect(() => {
    let live = true;
    setLoad((prev) => (prev.status === 'ok' ? prev : { status: 'loading' })); // keep the rows while the next page loads
    const query: AuditQuery = { actor, action, from: dayBound(from, false), to: dayBound(to, true), page, pageSize: PAGE_SIZE };
    if (userId) Object.assign(query, { targetType: 'user', targetId: userId });
    else if (app) Object.assign(query, { targetType: 'application', targetId: app });
    api.audit.list(query).then(
      (data) => live && setLoad({ status: 'ok', data }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, [actor, userId, app, action, from, to, page, attempt]);

  const data = load.status === 'ok' ? load.data : undefined;
  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const filtered = actor !== '' || userId !== '' || app !== '' || action !== '' || from !== '' || to !== '';
  const label = (u: AdminUser) => `${u.displayName} · ${u.username}`;

  return (
    <>
      <div className="space-y-1">
        <h1 className="flex items-baseline gap-2 text-2xl font-bold">
          {t('audit.title')}
          {data && <span className="font-normal text-text-3">({total})</span>}
        </h1>
        <p className="text-text-2">{t('audit.intro')}</p>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Select aria-label={t('audit.filter.operator')} className="w-52" value={actor} onChange={(e) => update({ actor: e.target.value })}>
          <option value="">{t('audit.filter.allOperators')}</option>
          <option value="cli">{t('audit.filter.cli')}</option>
          {users.map((u) => (
            <option key={u.id} value={u.id}>
              {label(u)}
            </option>
          ))}
        </Select>
        <Select aria-label={t('audit.filter.objectUser')} className="w-52" value={userId} onChange={(e) => (setAppInput(''), update({ user: e.target.value, app: '' }))}>
          <option value="">{t('audit.filter.allUsers')}</option>
          {users.map((u) => (
            <option key={u.id} value={u.id}>
              {label(u)}
            </option>
          ))}
        </Select>
        <Input
          type="search"
          aria-label={t('audit.filter.objectApp')}
          placeholder={t('audit.filter.objectApp')}
          value={appInput}
          onChange={(e) => setAppInput(e.target.value)}
          className="w-44"
        />
        <Select aria-label={t('audit.filter.action')} className="w-52" value={action} onChange={(e) => update({ action: e.target.value })}>
          <option value="">{t('audit.filter.allActions')}</option>
          {auditActions.map((a) => (
            <option key={a} value={a}>
              {auditActionName(a)}
            </option>
          ))}
        </Select>
        <label className="flex items-center gap-1.5 text-[13px] text-text-2">
          {t('audit.filter.from')}
          <Input type="date" value={from} max={to || undefined} onChange={(e) => update({ from: e.target.value })} className="w-36" />
        </label>
        <label className="flex items-center gap-1.5 text-[13px] text-text-2">
          {t('audit.filter.to')}
          <Input type="date" value={to} min={from || undefined} onChange={(e) => update({ to: e.target.value })} className="w-36" />
        </label>
        {filtered && (
          <Button
            small
            onClick={() => {
              setAppInput('');
              update({ actor: '', user: '', app: '', action: '', from: '', to: '' });
            }}
          >
            {t('audit.filter.clear')}
          </Button>
        )}
        <span className="flex-1" />
        {data && total > 0 && (
          <span className="text-[13px] text-text-3">
            {t('audit.range', { from: (data.page - 1) * data.pageSize + 1, to: (data.page - 1) * data.pageSize + items.length, total })}
          </span>
        )}
      </div>

      <div className="overflow-x-auto rounded border border-line bg-white">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('audit.col.time')}</th>
              <th className={th}>{t('audit.col.operator')}</th>
              <th className={th}>{t('audit.col.event')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((e) => (
              <tr key={e.id} className="hover:bg-hover">
                <td className={`${td} whitespace-nowrap`}>{new Date(e.at).toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-GB')}</td>
                <td className={`${td} whitespace-nowrap`}>
                  {auditOperatorName(e.actor)}
                  {e.actor.kind === 'user' && <span className="ml-2 text-[12.5px] text-text-3">{e.actor.username}</span>}
                </td>
                <td className={`${td} break-words`}>{renderAuditEvent(e)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && (
          <Empty title={errorsByField(load.error)[''] ?? t('audit.loadFailed')}>
            <Button small onClick={() => setAttempt((n) => n + 1)}>
              {t('common.retry')}
            </Button>
          </Empty>
        )}
        {data && items.length === 0 && total > 0 && (
          // A page number past the end (an old link, or the log is filtered differently now).
          <Empty title={t('audit.pageOf', { page, pages })}>
            <Button small onClick={() => update({})}>
              {t('audit.first')}
            </Button>
          </Empty>
        )}
        {data && items.length === 0 && total === 0 && filtered && <Empty title={t('audit.noMatch')} />}
        {data && items.length === 0 && total === 0 && !filtered && <Empty title={t('audit.empty.title')} />}
      </div>

      {data && total > PAGE_SIZE && (
        <nav className="flex items-center justify-end gap-2" aria-label={t('audit.pageOf', { page, pages })}>
          <span className="text-[13px] text-text-3">{t('audit.pageOf', { page, pages })}</span>
          <Button small disabled={page <= 1} onClick={() => update({ page: String(page - 1) })}>
            {t('audit.prev')}
          </Button>
          <Button small disabled={page >= pages} onClick={() => update({ page: String(page + 1) })}>
            {t('audit.next')}
          </Button>
        </nav>
      )}
    </>
  );
}
