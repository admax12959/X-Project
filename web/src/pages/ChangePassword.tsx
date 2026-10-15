// Changing your own password: the form (settings page) and the page a user with an
// administrator-set password gets instead of the app until they have changed it.
import { useState, type FormEvent } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import { useT } from '../i18n/index.ts';
import { useSession } from '../session/session.tsx';
import { LanguageSwitch } from '../shell/Shell.tsx';
import { Button, Field, Input } from '../ui/controls.tsx';

export function ChangePasswordForm({ onDone }: { onDone: () => void }) {
  const { t } = useT();
  const [form, setForm] = useState({ currentPassword: '', newPassword: '', confirm: '' });
  const [error, setError] = useState<unknown>(null);
  const [mismatch, setMismatch] = useState(false);
  const [busy, setBusy] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [k]: e.target.value }));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setError(null);
    setMismatch(form.newPassword !== form.confirm);
    if (form.newPassword !== form.confirm) return;
    setBusy(true);
    try {
      await api.me.changePassword({ currentPassword: form.currentPassword, newPassword: form.newPassword });
      setForm({ currentPassword: '', newPassword: '', confirm: '' });
      onDone();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} noValidate className="grid max-w-[380px] gap-3.5">
      {errors[''] && (
        <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
          {errors['']}
        </p>
      )}
      <Field label={t('password.current')} error={errors.currentPassword}>
        <Input name="currentPassword" type="password" autoComplete="current-password" value={form.currentPassword} onChange={set('currentPassword')} />
      </Field>
      <Field label={t('password.new')} error={errors.newPassword}>
        <Input name="newPassword" type="password" autoComplete="new-password" value={form.newPassword} onChange={set('newPassword')} />
        <p className="mt-1 text-[12px] text-text-3">{t('users.hint.password')}</p>
      </Field>
      <Field label={t('password.confirm')} error={mismatch ? t('password.mismatch') : undefined}>
        <Input name="confirm" type="password" autoComplete="new-password" value={form.confirm} onChange={set('confirm')} />
      </Field>
      <div>
        <Button kind="primary" type="submit" disabled={busy}>
          {busy ? t('users.saving') : t('password.submit')}
        </Button>
      </div>
    </form>
  );
}

/** Shown instead of the whole app while the session was opened with an administrator-set password. */
export function ForcedChangePasswordPage({ onDone }: { onDone: () => void }) {
  const { t } = useT();
  const { logout } = useSession();
  return (
    <div className="flex min-h-screen flex-col items-center bg-subtle pt-[12vh]">
      <div className="mb-4 text-lg font-bold">{t('app.brand')}</div>
      <div className="w-[420px] rounded border border-line bg-white p-6">
        <div className="mb-2 flex items-center justify-between">
          <h1 className="text-xl font-bold">{t('password.forced.title')}</h1>
          <LanguageSwitch />
        </div>
        <p className="mb-4 text-[13px] text-text-2">{t('password.forced.body')}</p>
        <ChangePasswordForm onDone={onDone} />
        <button type="button" className="mt-4 text-[13px] text-accent hover:underline" onClick={() => void logout()}>
          {t('password.forced.logout')}
        </button>
      </div>
    </div>
  );
}
