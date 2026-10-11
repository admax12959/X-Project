// Local-account login. #14 adds the LDAP / local switch.
import { useState, type FormEvent } from 'react';
import { Navigate, useNavigate, useSearchParams } from 'react-router';
import { errorsByField } from '../api/errors.ts';
import { useT } from '../i18n/index.ts';
import { useSession } from '../session/session.tsx';
import { LanguageSwitch } from '../shell/Shell.tsx';
import { Button, Field, Input } from '../ui/controls.tsx';

/** Only same-origin paths are accepted as redirect targets. */
function safeNext(v: string | null): string {
  return v && v.startsWith('/') && !v.startsWith('//') ? v : '/';
}

export function LoginPage() {
  const { t } = useT();
  const { state, login } = useSession();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const next = safeNext(params.get('next'));
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  // Keep the raw error; messages are rendered at render time so they follow a language switch.
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  if (state.status === 'authenticated') return <Navigate to={next} replace />;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await login({ method: 'local', username, password });
      navigate(next, { replace: true });
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center bg-subtle pt-[12vh]">
      <div className="mb-4 text-lg font-bold">{t('app.brand')}</div>
      <form onSubmit={submit} noValidate className="w-[380px] rounded border border-line bg-white p-6">
        <div className="mb-4 flex items-center justify-between">
          <h1 className="text-xl font-bold">{t('login.title')}</h1>
          <LanguageSwitch />
        </div>
        <div className="grid gap-3.5">
          {errors[''] && (
            <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
              {errors['']}
            </p>
          )}
          <Field label={t('login.username')} error={errors.username}>
            <Input name="username" autoComplete="username" autoFocus value={username} onChange={(e) => setUsername(e.target.value)} />
          </Field>
          <Field label={t('login.password')} error={errors.password}>
            <Input name="password" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </Field>
          <Button kind="primary" type="submit" disabled={busy}>
            {busy ? t('login.submitting') : t('login.submit')}
          </Button>
        </div>
      </form>
    </div>
  );
}
