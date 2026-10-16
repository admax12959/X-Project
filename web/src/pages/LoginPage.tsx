// Login page: email + password, or LDAP when the server has a directory configured.
import { useEffect, useState, type FormEvent } from 'react';
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router';
import { api, ApiError } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { LoginMethod } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useSession } from '../session/session.tsx';
import { LanguageSwitch } from '../shell/Shell.tsx';
import { Button, Field, Input, Segmented } from '../ui/controls.tsx';

/** Only same-origin paths are accepted as redirect targets. */
function safeNext(v: string | null): string {
  return v && v.startsWith('/') && !v.startsWith('//') ? v : '/';
}

export function LoginPage() {
  const { t } = useT();
  const { state, login, submitSecondFactor } = useSession();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const next = safeNext(params.get('next'));
  // null until GET /api/login-methods answers; if it fails, offer email login only.
  const [methods, setMethods] = useState<LoginMethod[] | null>(null);
  // Whether "forgot password" can work (it needs mail).
  const [recovery, setRecovery] = useState(false);
  const [chosen, setChosen] = useState<LoginMethod | null>(null);
  // The login email for the email method, the directory login name for LDAP.
  const [identifier, setIdentifier] = useState('');
  const [password, setPassword] = useState('');
  // Keep the raw error; messages are rendered at render time so they follow a language switch.
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  // The password was right and the user has a second factor: ask for the code.
  const [needsCode, setNeedsCode] = useState(false);
  const [code, setCode] = useState('');
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  useEffect(() => {
    api.session.methods().then(
      (r) => (setMethods(r.items), setRecovery(r.passwordRecovery)),
      () => setMethods(['email']),
    );
  }, []);

  const hasLdap = methods?.includes('ldap') ?? false;
  const method: LoginMethod = chosen ?? (hasLdap ? 'ldap' : 'email');

  if (state.status === 'authenticated') return <Navigate to={next} replace />;

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const outcome = await login(method === 'email' ? { method, email: identifier, password } : { method, username: identifier, password });
      if (outcome === 'secondFactor') {
        setNeedsCode(true);
        setCode('');
        return;
      }
      navigate(next, { replace: true });
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const backToPassword = () => {
    setNeedsCode(false);
    setPassword('');
    setError(null);
  };

  const submitCode = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await submitSecondFactor(code.trim());
      navigate(next, { replace: true });
    } catch (err) {
      // 401: the short-lived token is gone (expired); the password has to be typed again.
      if (err instanceof ApiError && err.status === 401) backToPassword();
      else setCode('');
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  if (needsCode) {
    return (
      <div className="flex min-h-screen flex-col items-center bg-subtle pt-[12vh]">
        <div className="mb-4 text-lg font-bold">{t('app.brand')}</div>
        <form onSubmit={submitCode} noValidate className="w-[380px] rounded border border-line bg-white p-6">
          <div className="mb-2 flex items-center justify-between">
            <h1 className="text-xl font-bold">{t('login.secondFactor.title')}</h1>
            <LanguageSwitch />
          </div>
          <p className="mb-4 text-[13px] text-text-2">{t('login.secondFactor.hint')}</p>
          <div className="grid gap-3.5">
            {errors[''] && (
              <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                {errors['']}
              </p>
            )}
            <Field label={t('login.secondFactor.code')} error={errors.code}>
              <Input
                name="code"
                inputMode="text"
                autoComplete="one-time-code"
                autoCapitalize="characters"
                autoCorrect="off"
                spellCheck={false}
                autoFocus
                maxLength={20}
                value={code}
                onChange={(e) => setCode(e.target.value)}
              />
            </Field>
            <Button kind="primary" type="submit" disabled={busy}>
              {busy ? t('login.secondFactor.submitting') : t('login.secondFactor.submit')}
            </Button>
            <button type="button" className="justify-self-start text-[13px] text-accent hover:underline" onClick={backToPassword}>
              {t('login.secondFactor.back')}
            </button>
          </div>
        </form>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen flex-col items-center bg-subtle pt-[12vh]">
      <div className="mb-4 text-lg font-bold">{t('app.brand')}</div>
      <form onSubmit={submit} noValidate className="w-[380px] rounded border border-line bg-white p-6">
        <div className="mb-4 flex items-center justify-between">
          <h1 className="text-xl font-bold">{t('login.title')}</h1>
          <LanguageSwitch />
        </div>
        {methods === null ? (
          <p className="text-text-2">{t('common.loading')}</p>
        ) : (
          <div className="grid gap-3.5">
            {hasLdap && (
              <Segmented
                label={t('login.method')}
                value={method}
                options={[
                  { value: 'ldap', label: t('login.method.ldap') },
                  { value: 'email', label: t('login.method.email') },
                ]}
                onChange={(m) => {
                  setChosen(m);
                  setError(null);
                }}
              />
            )}
            {errors[''] && (
              <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                {errors['']}
              </p>
            )}
            {method === 'email' ? (
              <Field key="email" label={t('login.email')} error={errors.email}>
                <Input name="email" type="email" autoComplete="username" autoFocus value={identifier} onChange={(e) => setIdentifier(e.target.value)} />
              </Field>
            ) : (
              <Field key="username" label={t('login.username')} error={errors.username}>
                <Input name="username" autoComplete="username" autoFocus value={identifier} onChange={(e) => setIdentifier(e.target.value)} />
              </Field>
            )}
            <Field label={t('login.password')} error={errors.password}>
              <Input name="password" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </Field>
            <Button kind="primary" type="submit" disabled={busy}>
              {busy ? t('login.submitting') : t('login.submit')}
            </Button>
            {method === 'email' &&
              (recovery ? (
                <Link to="/forgot-password" className="justify-self-start text-[13px] text-accent hover:underline">
                  {t('login.forgot')}
                </Link>
              ) : (
                <p className="text-[13px] text-text-3">
                  {t('login.forgot')} {t('login.forgot.unavailable')}
                </p>
              ))}
          </div>
        )}
      </form>
    </div>
  );
}
