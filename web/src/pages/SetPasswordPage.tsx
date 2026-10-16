// The set-password page, shared by invitations and password recovery: the link in the mail
// carries a one-time token. It opens no session; after success the user signs in.
import { useState, type FormEvent } from 'react';
import { Link, useParams } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import { useT } from '../i18n/index.ts';
import { LanguageSwitch } from '../shell/Shell.tsx';
import { Button, Field, Input } from '../ui/controls.tsx';

export function SetPasswordPage() {
  const { t } = useT();
  const { token = '' } = useParams();
  const [form, setForm] = useState({ password: '', confirm: '' });
  const [error, setError] = useState<unknown>(null);
  const [submitted, setSubmitted] = useState(false);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};
  // Derived from the current values, so the error clears once the fields match again.
  const mismatch = submitted && form.password !== form.confirm;
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [k]: e.target.value }));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setSubmitted(true);
    if (form.password !== form.confirm) return;
    setBusy(true);
    try {
      await api.session.setPassword(token, form.password);
      setDone(true);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center bg-subtle pt-[12vh]">
      <div className="mb-4 text-lg font-bold">{t('app.brand')}</div>
      <form onSubmit={submit} noValidate className="w-full max-w-[380px] rounded border border-line bg-white p-6">
        <div className="mb-4 flex items-center justify-between">
          <h1 className="text-xl font-bold">{t('setPassword.title')}</h1>
          <LanguageSwitch />
        </div>
        {done ? (
          <div className="grid gap-3.5">
            <p role="status" className="text-text-2">
              {t('setPassword.done')}
            </p>
            <Link to="/login" className="text-accent hover:underline">
              {t('setPassword.toLogin')}
            </Link>
          </div>
        ) : (
          <div className="grid gap-3.5">
            <p className="text-text-2">{t('setPassword.hint')}</p>
            {(errors[''] || errors.token) && (
              <div role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                <p>{errors[''] ?? errors.token}</p>
                {errors.token && (
                  <p className="mt-1">
                    <Link to="/forgot-password" className="text-accent hover:underline">
                      {t('setPassword.askAgain')}
                    </Link>
                  </p>
                )}
              </div>
            )}
            <Field label={t('password.new')} error={errors.password}>
              <Input name="password" type="password" autoComplete="new-password" autoFocus value={form.password} onChange={set('password')} />
            </Field>
            <Field label={t('password.confirm')} error={mismatch ? t('password.mismatch') : undefined}>
              <Input name="confirm" type="password" autoComplete="new-password" value={form.confirm} onChange={set('confirm')} />
            </Field>
            <Button kind="primary" type="submit" disabled={busy}>
              {busy ? t('setPassword.submitting') : t('setPassword.submit')}
            </Button>
          </div>
        )}
      </form>
    </div>
  );
}
