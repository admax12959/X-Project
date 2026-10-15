// Administration > Roles (#26): the built-in roles and the permissions each includes. Read-only. Administrators only.
import { useCallback, useEffect, useState } from 'react';
import { Navigate } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { Role } from '../api/types.ts';
import { useT, type MessageKey } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Empty } from '../ui/controls.tsx';
import { td, th } from '../ui/table.ts';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; items: Role[] };

/** The page is for administrators; anyone else is sent home (the API also answers 403). */
export function RolesPage() {
  const user = useCurrentUser();
  if (!user.isAdmin) return <Navigate to="/" replace />;
  return <Roles />;
}

function Roles() {
  const { t } = useT();
  const [load, setLoad] = useState<Load>({ status: 'loading' });

  const reload = useCallback(() => {
    api.roles.list().then(
      (r) => setLoad({ status: 'ok', items: r.items }),
      (error: unknown) => setLoad({ status: 'error', error }),
    );
  }, []);
  useEffect(reload, [reload]);

  return (
    <>
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">{t('roles.title')}</h1>
        <p className="text-text-2">{t('roles.intro')}</p>
      </div>
      {load.status === 'loading' && <Empty title={t('common.loading')} />}
      {load.status === 'error' && (
        <Empty title={errorsByField(load.error)[''] ?? t('roles.loadFailed')}>
          <Button small onClick={reload}>
            {t('common.retry')}
          </Button>
        </Empty>
      )}
      {load.status === 'ok' && load.items.map((role) => <RoleCard key={role.id} role={role} />)}
    </>
  );
}

function RoleCard({ role }: { role: Role }) {
  const { t } = useT();
  return (
    <section aria-labelledby={`role-${role.id}`} className="overflow-x-auto rounded border border-line bg-white">
      <div className="space-y-1 border-b border-line px-3 py-3">
        <h2 id={`role-${role.id}`} className="flex items-baseline gap-3 text-[15px] font-bold">
          {t(`roles.role.${role.id}.name`)}
          <span className="text-[13px] font-normal text-text-3">
            {t(`roles.scope.${role.scope}`)} · {t('roles.permissionCount', { count: role.permissions.length })}
          </span>
        </h2>
        <p className="text-text-2">{t(`roles.role.${role.id}.desc`)}</p>
      </div>
      <table className="w-full border-collapse">
        <thead>
          <tr>
            <th className={`${th} w-72`}>{t('roles.col.permission')}</th>
            <th className={th}>{t('roles.col.description')}</th>
          </tr>
        </thead>
        <tbody>
          {role.permissions.map((p) => (
            <tr key={p} className="hover:bg-hover">
              <td className={`${td} font-mono text-[12.5px]`}>{p}</td>
              <td className={td}>{t(`perm.${p}` as MessageKey)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}
