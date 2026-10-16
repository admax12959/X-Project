// Administration > Security policy (#23, ADR-0006): require two-factor authentication of every user. Administrators are always
// required; the page says so instead of offering a switch for them.
import { useCallback, useEffect, useState } from 'react';
import { Navigate } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { SecurityPolicy } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Container, Empty } from '../ui/controls.tsx';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; policy: SecurityPolicy };

/** The page is for administrators; anyone else is sent home (the API also answers 403). */
export function SecurityPolicyPage() {
  const user = useCurrentUser();
  if (!user.isAdmin) return <Navigate to="/" replace />;
  return <SecurityPolicyView />;
}

function SecurityPolicyView() {
  const { t } = useT();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const reload = useCallback(() => {
    setLoad({ status: 'loading' });
    api.systemConfig.security.get().then(
      (policy) => setLoad({ status: 'ok', policy }),
      (error: unknown) => setLoad({ status: 'error', error }),
    );
  }, []);
  useEffect(reload, [reload]);

  return (
    <>
      <div>
        <h1 className="text-2xl font-bold">{t('security.title')}</h1>
        <p className="mt-1 max-w-[760px] text-text-2">{t('security.intro')}</p>
      </div>
      {load.status === 'loading' && <Empty title={t('common.loading')} />}
      {load.status === 'error' && (
        <Empty title={errorsByField(load.error)[''] ?? t('security.loadFailed')}>
          <Button small onClick={reload}>
            {t('common.retry')}
          </Button>
        </Empty>
      )}
      {load.status === 'ok' && <PolicyForm policy={load.policy} onSaved={(policy) => setLoad({ status: 'ok', policy })} />}
    </>
  );
}

function PolicyForm({ policy, onSaved }: { policy: SecurityPolicy; onSaved: (p: SecurityPolicy) => void }) {
  const { t } = useT();
  const [force, setForce] = useState(policy.forceSecondFactor);
  const [error, setError] = useState<unknown>(null); // raw, rendered at render time so it follows a language switch
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState(false);
  const message = error ? errorsByField(error)[''] : undefined;

  const save = async (e: { preventDefault: () => void }) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const next = await api.systemConfig.security.update(force);
      onSaved(next);
      setSaved(true);
    } catch (err) {
      setError(err);
      setSaved(false);
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={save} noValidate className="max-w-[760px] space-y-5">
      {message && (
        <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
          {message}
        </p>
      )}
      <Container title={t('security.field.forceSecondFactor')}>
        <div className="grid gap-2">
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              name="forceSecondFactor"
              checked={force}
              disabled={busy}
              onChange={(e) => {
                setForce(e.target.checked);
                setSaved(false);
              }}
            />
            {t('security.force.label')}
          </label>
          <p className="text-[12px] text-text-3">{t('security.force.help')}</p>
          <p className="text-[13px] text-text-2">{t('security.admins.note')}</p>
        </div>
      </Container>
      <div className="flex flex-wrap items-center gap-3">
        <Button kind="primary" type="submit" disabled={busy}>
          {busy ? t('security.saving') : t('security.save')}
        </Button>
        {saved && (
          <span role="status" className="text-[13px] text-ok">
            {t('security.saved')}
          </span>
        )}
      </div>
    </form>
  );
}
