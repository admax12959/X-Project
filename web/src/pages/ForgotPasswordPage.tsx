// "Forgot password": ask for a recovery link by mail. The answer is the same whether or not the
// address belongs to an account, so the page can only say "if it does, a mail is on its way".
import { useEffect, useState, type FormEvent } from 'react';
import { Link } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import { useT } from '../i18n/index.ts';
import { LanguageSwitch } from '../shell/Shell.tsx';
import { Button, Field, Input } from '../ui/controls.tsx';

export function ForgotPasswordPage() {
  const { t } = useT();
  // null until GET /api/login-methods answers.
  const [available, setAvailable] = useState<boolean | null>(null);
  const [email, setEmail] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  useEffect(() => {
    api.session.methods().then(
      (r) => setAvailable(r.passwordRecovery),
      () => setAvailable(false),
    );
  }, []);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.session.requestPasswordRecovery(email);
      setSent(true);
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
          <h1 className="text-xl font-bold">{t('recovery.title')}</h1>
          <LanguageSwitch />
        </div>
        {available === null ? (
          <p className="text-text-2">{t('common.loading')}</p>
        ) : !available ? (
          <p role="status" className="text-text-2">
            {t('recovery.unavailable')}
          </p>
        ) : sent ? (
          <p role="status" className="text-text-2">
            {t('recovery.sent')}
          </p>
        ) : (
          <div className="grid gap-3.5">
            <p className="text-text-2">{t('recovery.hint')}</p>
            {errors[''] && (
              <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                {errors['']}
              </p>
            )}
            <Field label={t('login.email')} error={errors.email}>
              <Input name="email" type="email" autoComplete="username" autoFocus value={email} onChange={(e) => setEmail(e.target.value)} />
            </Field>
            <Button kind="primary" type="submit" disabled={busy}>
              {busy ? t('recovery.submitting') : t('recovery.submit')}
            </Button>
          </div>
        )}
        <p className="mt-4 text-[13px]">
          <Link to="/login" className="text-accent hover:underline">
            {t('recovery.backToLogin')}
          </Link>
        </p>
      </form>
    </div>
  );
}
