// Administration > Users: list, create local users, edit, disable / enable. Administrators only.
import { useEffect, useState } from 'react';
import { Link } from 'react-router';
import { api, ApiError } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { AdminUser, LoginMethod } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Empty } from '../ui/controls.tsx';
import { BindIdentityDialog } from './BindIdentityDialog.tsx';
import { UserDialog } from './UserDialog.tsx';
import { th, td } from '../ui/table.ts';

/** The applications blocking a disable (422 soleOwner), or undefined for any other error. */
function soleOwnerApps(err: unknown): { key: string; name: string }[] | undefined {
  if (!(err instanceof ApiError)) return undefined;
  const e = err.errors.find((x) => x.code === 'soleOwner');
  return e ? (e.params?.applications as { key: string; name: string }[] | undefined) : undefined;
}

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; items: AdminUser[] };
// null = closed, 'create' = new local user, otherwise the user being edited.
type Dialog = null | 'create' | AdminUser;

export function UsersPage() {
  const { t } = useT();
  const me = useCurrentUser();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const [dialog, setDialog] = useState<Dialog>(null);
  const [actionError, setActionError] = useState<unknown>(null);
  const [actionUser, setActionUser] = useState<AdminUser | null>(null);
  const [bindFor, setBindFor] = useState<AdminUser | null>(null);
  // Login identity kinds the server offers (ldap only when a directory is configured).
  const [methods, setMethods] = useState<LoginMethod[]>(['email']);
  const missingKinds = (u: AdminUser): LoginMethod[] => methods.filter((m) => !u.identities.some((i) => i.kind === m));

  useEffect(() => {
    if (!me.isAdmin) return;
    let live = true;
    api.session.methods().then(
      (r) => live && setMethods(r.items),
      () => undefined,
    );
    api.users.list().then(
      (r) => live && setLoad({ status: 'ok', items: r.items }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    return () => {
      live = false;
    };
  }, [me.isAdmin]);

  if (!me.isAdmin) return <Empty title={t('users.forbidden')} />;

  const items = load.status === 'ok' ? load.items : [];
  const replace = (u: AdminUser) => setLoad({ status: 'ok', items: items.map((x) => (x.id === u.id ? u : x)) });
  const append = (u: AdminUser) => setLoad({ status: 'ok', items: [...items, u] });

  const changeStatus = async (u: AdminUser) => {
    setActionError(null);
    setActionUser(u);
    try {
      replace(await (u.status === 'enabled' ? api.users.disable(u.id) : api.users.enable(u.id)));
    } catch (err) {
      setActionError(err);
    }
  };


  return (
    <>
      <div className="flex min-h-8 items-center justify-between gap-4">
        <h1 className="flex items-baseline gap-2 text-2xl font-bold">
          {t('users.title')}
          {load.status === 'ok' && <span className="font-normal text-text-3">({items.length})</span>}
        </h1>
        <Button kind="primary" onClick={() => setDialog('create')}>
          {t('users.create')}
        </Button>
      </div>

      {actionError !== null && (
        <div role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
          {soleOwnerApps(actionError) && actionUser ? (
            <>
              <p>{t('users.soleOwner.intro', { name: actionUser.displayName })}</p>
              <ul className="mt-1 list-disc pl-5">
                {soleOwnerApps(actionError)?.map((a) => (
                  <li key={a.key}>
                    <Link to={`/apps/${encodeURIComponent(a.key)}`} className="text-accent hover:underline">
                      {a.name}
                    </Link>{' '}
                    <span className="font-mono text-[12.5px]">{a.key}</span>
                  </li>
                ))}
              </ul>
              <p className="mt-1">{t('users.soleOwner.hint')}</p>
            </>
          ) : (
            errorsByField(actionError)['']
          )}
        </div>
      )}

      <div className="overflow-x-auto rounded border border-line bg-white">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('users.col.username')}</th>
              <th className={th}>{t('users.col.displayName')}</th>
              <th className={th}>{t('users.col.email')}</th>
              <th className={th}>{t('users.col.department')}</th>
              <th className={th}>{t('users.col.status')}</th>
              <th className={th}>{t('users.col.identities')}</th>
              <th className={th}>{t('users.col.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((u) => (
              <tr key={u.id} className="hover:bg-hover">
                <td className={`${td} font-mono text-[12.5px]`}>{u.username}</td>
                <td className={td}>
                  {u.displayName}
                  {u.isAdmin && <span className="ml-2 text-[13px] text-text-3">{t('term.administrator')}</span>}
                </td>
                <td className={td}>{u.email}</td>
                <td className={td}>{u.department}</td>
                <td className={`${td} ${u.status === 'enabled' ? 'text-ok' : 'text-text-3'}`}>{t(`users.status.${u.status}`)}</td>
                <td className={td}>{[...new Set(u.identities.map((i) => i.kind))].map((k) => t(`users.identity.${k}`)).join(' · ')}</td>
                <td className={`${td} whitespace-nowrap`}>
                  <span className="inline-flex gap-2">
                    <Button small onClick={() => setDialog(u)}>
                      {t('users.edit')}
                    </Button>
                    {missingKinds(u).length > 0 && (
                      <Button small onClick={() => setBindFor(u)}>
                        {t('users.bindIdentity')}
                      </Button>
                    )}
                    <Button small onClick={() => changeStatus(u)}>
                      {u.status === 'enabled' ? t('users.disable') : t('users.enable')}
                    </Button>
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && <Empty title={errorsByField(load.error)[''] ?? t('users.loadFailed')} />}
        {load.status === 'ok' && items.length === 0 && <Empty title={t('users.empty')} />}
      </div>

      {bindFor !== null && (
        <BindIdentityDialog
          audience="admin"
          username={bindFor.username}
          kinds={missingKinds(bindFor)}
          bind={(input) => api.users.bindIdentity(bindFor.id, input)}
          onClose={() => setBindFor(null)}
          onBound={(u) => {
            replace(u);
            setBindFor(null);
          }}
        />
      )}

      {dialog !== null && (
        <UserDialog
          user={dialog === 'create' ? null : dialog}
          onClose={() => setDialog(null)}
          onSaved={(u) => {
            if (dialog === 'create') append(u);
            else replace(u);
            setDialog(null);
          }}
        />
      )}
    </>
  );
}
