// Create-user and edit-user dialog for the administrator's user page.
import { Dialog } from '@base-ui/react/dialog';
import { useState, type FormEvent } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { AdminUser, UpdateUserInput } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button, Field, Input } from '../ui/controls.tsx';

/** `user` set = edit that user; `null` = create a user with an email identity. */
export function UserDialog({ user, onClose, onSaved }: { user: AdminUser | null; onClose: () => void; onSaved: (u: AdminUser) => void }) {
  const { t } = useT();
  const editing = user !== null;
  // For an LDAP-bound user the directory owns these fields and the username is fixed (ADR-0004).
  const ldapBound = !!user?.identities.some((i) => i.kind === 'ldap');
  // The login email lives on the email identity; a directory-only user has none to edit.
  const loginEmail = user?.identities.find((i) => i.kind === 'email')?.loginName;
  const [form, setForm] = useState({
    username: user?.username ?? '',
    displayName: user?.displayName ?? '',
    loginEmail: loginEmail ?? '',
    phone: user?.phone ?? '',
    department: user?.department ?? '',
    password: '',
  });
  const [isAdmin, setIsAdmin] = useState(user?.isAdmin ?? false);
  // Keep the raw error and render at render time so messages follow a language switch.
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [k]: e.target.value }));

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      let saved: AdminUser;
      if (user) {
        // Send only what changed, so untouched LDAP-managed fields are never submitted.
        const changes: UpdateUserInput = {};
        for (const k of ['username', 'displayName', 'phone', 'department'] as const) {
          if (form[k] !== user[k]) changes[k] = form[k];
        }
        if (loginEmail !== undefined && form.loginEmail !== loginEmail) changes.loginEmail = form.loginEmail;
        if (isAdmin !== user.isAdmin) changes.isAdmin = isAdmin;
        saved = await api.users.update(user.id, changes);
      } else {
        saved = await api.users.create(form);
      }
      onSaved(saved);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 bg-black/30" />
        <Dialog.Popup className="fixed left-1/2 top-[10vh] max-h-[80vh] w-[480px] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-4 text-lg font-bold">
            {user ? t('users.editTitle', { username: user.username }) : t('users.createTitle')}
          </Dialog.Title>
          <form onSubmit={submit} noValidate className="grid gap-3.5">
            {errors[''] && (
              <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                {errors['']}
              </p>
            )}
            <Field label={t('users.field.username')} error={errors.username}>
              <Input name="username" autoComplete="off" autoFocus={!ldapBound} disabled={ldapBound} value={form.username} onChange={set('username')} />
              <Hint>{ldapBound ? t('users.hint.usernameLocked') : t('users.hint.username')}</Hint>
            </Field>
            <Field label={t('users.field.displayName')} error={errors.displayName}>
              <Input name="displayName" disabled={ldapBound} value={form.displayName} onChange={set('displayName')} />
              {ldapBound && <Hint>{t('users.hint.ldapManaged')}</Hint>}
            </Field>
            {(!editing || loginEmail !== undefined) && (
              <Field label={t('users.field.loginEmail')} error={errors.loginEmail}>
                <Input name="loginEmail" type="email" autoComplete="off" value={form.loginEmail} onChange={set('loginEmail')} />
                <Hint>{t('users.hint.loginEmail')}</Hint>
              </Field>
            )}
            {editing && (
              <Field label={t('users.field.email')} error={errors.email}>
                <Input name="email" type="email" disabled value={user.email} readOnly />
                <Hint>{ldapBound ? t('users.hint.ldapManaged') : t('users.hint.contactEmail')}</Hint>
              </Field>
            )}
            <Field label={t('users.field.department')} error={errors.department}>
              <Input name="department" disabled={ldapBound} value={form.department} onChange={set('department')} />
              <Hint>{ldapBound ? t('users.hint.ldapManaged') : t('users.hint.optional')}</Hint>
            </Field>
            <Field label={t('users.field.phone')} error={errors.phone}>
              <Input name="phone" type="tel" value={form.phone} onChange={set('phone')} />
              <Hint>{t('users.hint.optional')}</Hint>
            </Field>
            {editing && (
              <Field label={t('term.administrator')} error={errors.isAdmin}>
                <label className="flex items-center gap-2">
                  <input type="checkbox" name="isAdmin" checked={isAdmin} onChange={(e) => setIsAdmin(e.target.checked)} />
                  {t('users.field.isAdmin')}
                </label>
                <Hint>{t('users.hint.isAdmin')}</Hint>
              </Field>
            )}
            {!editing && (
              <Field label={t('users.field.password')} error={errors.password}>
                <Input name="password" type="password" autoComplete="new-password" value={form.password} onChange={set('password')} />
                <Hint>{t('users.hint.tempPassword')}</Hint>
              </Field>
            )}
            <div className="mt-2 flex justify-end gap-2">
              <Button type="button" onClick={onClose}>
                {t('users.cancel')}
              </Button>
              <Button kind="primary" type="submit" disabled={busy}>
                {busy ? t('users.saving') : t('users.save')}
              </Button>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function Hint({ children }: { children: string }) {
  return <p className="mt-1 text-[12px] text-text-3">{children}</p>;
}
