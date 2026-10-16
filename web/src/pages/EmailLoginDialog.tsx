// Add an email login (LDAP users) or change the login email: enter the address, get a code by
// mail, enter the code (and, when adding, the password to log in with).
import { Dialog } from '@base-ui/react/dialog';
import { useState, type FormEvent } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { AdminUser } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button, Field, Input } from '../ui/controls.tsx';

export function EmailLoginDialog({
  mode,
  ldapBound,
  onClose,
  onDone,
}: {
  mode: 'bind' | 'change';
  /** The directory owns the contact email, so a changed login email does not move it. */
  ldapBound: boolean;
  onClose: () => void;
  onDone: (u: AdminUser) => void;
}) {
  const { t } = useT();
  const [email, setEmail] = useState('');
  const [sentTo, setSentTo] = useState<string | null>(null);
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try {
      await action();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const sendCode = (e: FormEvent) => {
    e.preventDefault();
    return run(async () => {
      await api.me.requestEmailCode(mode, email);
      setSentTo(email.trim());
      setCode('');
    });
  };

  const confirm = (e: FormEvent) => {
    e.preventDefault();
    return run(async () => {
      const target = sentTo ?? email;
      onDone(mode === 'bind' ? await api.me.bindEmailIdentity({ email: target, code, password }) : await api.me.changeLoginEmail({ email: target, code }));
    });
  };

  const title = mode === 'bind' ? t('settings.emailLogin.addTitle') : t('settings.emailLogin.changeTitle');
  const emailError = errors.email ?? errors.loginName ?? errors.kind;
  const note = ldapBound ? t('settings.emailLogin.contactNote') : t('settings.emailLogin.contactFollows');

  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 bg-black/30" />
        <Dialog.Popup className="fixed left-1/2 top-[10vh] max-h-[80vh] w-[480px] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-2 text-lg font-bold">{title}</Dialog.Title>
          <p className="mb-4 text-[13px] text-text-2">
            {mode === 'bind' ? t('settings.emailLogin.addHint') : t('settings.emailLogin.changeHint')} {mode === 'change' || ldapBound ? note : ''}
          </p>
          {errors[''] && (
            <p role="alert" className="mb-3 rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
              {errors['']}
            </p>
          )}
          <form onSubmit={sentTo === null ? sendCode : confirm} noValidate className="grid gap-3.5">
            <Field label={t('settings.emailLogin.email')} error={emailError}>
              <Input name="email" type="email" autoComplete="off" autoFocus={sentTo === null} disabled={sentTo !== null} value={sentTo ?? email} onChange={(e) => setEmail(e.target.value)} />
            </Field>
            {sentTo !== null && (
              <>
                <p role="status" className="text-[13px] text-ok">
                  {t('settings.emailLogin.sent', { email: sentTo })}
                </p>
                <Field label={t('settings.emailLogin.code')} error={errors.code}>
                  <Input name="code" inputMode="numeric" autoComplete="one-time-code" autoFocus value={code} onChange={(e) => setCode(e.target.value)} />
                </Field>
                {mode === 'bind' && (
                  <Field label={t('settings.emailLogin.password')} error={errors.password}>
                    <Input name="password" type="password" autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />
                  </Field>
                )}
              </>
            )}
            <div className="mt-2 flex justify-end gap-2">
              <Button type="button" onClick={onClose}>
                {t('users.cancel')}
              </Button>
              {sentTo !== null && (
                <Button
                  type="button"
                  disabled={busy}
                  onClick={() => {
                    setSentTo(null);
                    setError(null);
                  }}
                >
                  {t('settings.emailLogin.resend')}
                </Button>
              )}
              <Button kind="primary" type="submit" disabled={busy}>
                {sentTo === null
                  ? busy
                    ? t('settings.emailLogin.sending')
                    : t('settings.emailLogin.sendCode')
                  : busy
                    ? t('settings.emailLogin.confirming')
                    : mode === 'bind'
                      ? t('settings.emailLogin.confirmAdd')
                      : t('settings.emailLogin.confirmChange')}
              </Button>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
