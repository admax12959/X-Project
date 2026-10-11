// Bind a login identity: used by the user themselves (settings) and by an administrator (users page).
import { Dialog } from '@base-ui/react/dialog';
import { useState, type FormEvent } from 'react';
import { errorsByField } from '../api/errors.ts';
import type { AdminUser, BindIdentityInput, IdentityKind } from '../api/types.ts';
import { useT, type MessageKey } from '../i18n/index.ts';
import { Button, Field, Input, Segmented } from '../ui/controls.tsx';

type Labels = {
  kind: MessageKey;
  ldapHint: MessageKey;
  localHint: MessageKey;
  loginName: MessageKey;
  ldapPassword: MessageKey;
  localPassword: MessageKey;
  submit: MessageKey;
  submitting: MessageKey;
};

const selfLabels: Labels = {
  kind: 'users.bind.kind',
  ldapHint: 'settings.bind.ldapHint',
  localHint: 'settings.bind.localHint',
  loginName: 'settings.bind.loginName',
  ldapPassword: 'settings.bind.ldapPassword',
  localPassword: 'settings.bind.localPassword',
  submit: 'settings.bind.submit',
  submitting: 'settings.bind.submitting',
};

const adminLabels: Labels = {
  kind: 'users.bind.kind',
  ldapHint: 'users.bind.ldapHint',
  localHint: 'users.bind.localHint',
  loginName: 'users.bind.loginName',
  ldapPassword: 'users.bind.password',
  localPassword: 'users.bind.password',
  submit: 'users.bind.submit',
  submitting: 'users.bind.submitting',
};

/**
 * `kinds` are the identity kinds the user does not have yet (and that the server offers).
 * Self-binding asks for the LDAP password (you authenticate with the new identity once);
 * an administrator only names the LDAP login name, or sets a local password.
 */
export function BindIdentityDialog({
  audience,
  username,
  kinds,
  bind,
  onClose,
  onBound,
}: {
  audience: 'self' | 'admin';
  username: string;
  kinds: IdentityKind[];
  bind: (input: BindIdentityInput) => Promise<AdminUser>;
  onClose: () => void;
  onBound: (u: AdminUser) => void;
}) {
  const { t } = useT();
  const labels = audience === 'self' ? selfLabels : adminLabels;
  const [kind, setKind] = useState<IdentityKind>(kinds[0] ?? 'local');
  const [loginName, setLoginName] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  const title =
    audience === 'admin'
      ? t('users.bindTitle', { username })
      : kind === 'ldap'
        ? t('settings.bind.ldapTitle')
        : t('settings.bind.localTitle');

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      onBound(await bind(kind === 'ldap' ? { kind, loginName, ...(audience === 'self' ? { password } : {}) } : { kind, password }));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const needsLdapPassword = kind === 'ldap' && audience === 'self';

  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 bg-black/30" />
        <Dialog.Popup className="fixed left-1/2 top-[10vh] max-h-[80vh] w-[480px] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-4 text-lg font-bold">{title}</Dialog.Title>
          <form onSubmit={submit} noValidate className="grid gap-3.5">
            {errors[''] && (
              <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                {errors['']}
              </p>
            )}
            {kinds.length > 1 && (
              <Field label={t(labels.kind)} error={errors.kind}>
                <Segmented
                  label={t(labels.kind)}
                  value={kind}
                  onChange={setKind}
                  options={kinds.map((k) => ({ value: k, label: t(`users.identity.${k}`) }))}
                />
              </Field>
            )}
            <p className="text-[13px] text-text-2">
              {kind === 'ldap' ? t(labels.ldapHint) : t(labels.localHint, { username })}
            </p>
            {kind === 'ldap' && (
              <Field label={t(labels.loginName)} error={errors.loginName}>
                <Input name="loginName" autoComplete="off" autoFocus value={loginName} onChange={(e) => setLoginName(e.target.value)} />
              </Field>
            )}
            {(kind === 'local' || needsLdapPassword) && (
              <Field label={t(kind === 'ldap' ? labels.ldapPassword : labels.localPassword)} error={errors.password}>
                <Input
                  name="password"
                  type="password"
                  autoComplete={kind === 'local' ? 'new-password' : 'off'}
                  autoFocus={kind === 'local'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
              </Field>
            )}
            <div className="mt-2 flex justify-end gap-2">
              <Button type="button" onClick={onClose}>
                {t('users.cancel')}
              </Button>
              <Button kind="primary" type="submit" disabled={busy}>
                {busy ? t(labels.submitting) : t(labels.submit)}
              </Button>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
