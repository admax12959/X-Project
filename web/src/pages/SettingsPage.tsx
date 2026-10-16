// Personal settings: profile, login identities (bind / unbind), password, interface language.
import { Dialog } from '@base-ui/react/dialog';
import { useEffect, useState, type ChangeEvent, type FormEvent, type ReactNode } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { AdminUser, IdentityKind, LoginMethod } from '../api/types.ts';
import { LOCALES, setLocale, useT, type Locale } from '../i18n/index.ts';
import { useSession } from '../session/session.tsx';
import { Avatar } from '../ui/Avatar.tsx';
import { Button, Empty, Field, Input, Segmented } from '../ui/controls.tsx';
import { BindIdentityDialog } from './BindIdentityDialog.tsx';
import { ChangePasswordForm } from './ChangePassword.tsx';
<<<<<<< HEAD
import { EmailLoginDialog } from './EmailLoginDialog.tsx';
=======
import { SecondFactorPanel } from './SecondFactorPanel.tsx';
>>>>>>> integration/spec-18
import { th, td } from '../ui/table.ts';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; me: AdminUser };

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="rounded border border-line bg-white">
      <h2 className="border-b border-line bg-thead px-4 py-2.5 text-[15px] font-bold">{title}</h2>
      <div className="p-4">{children}</div>
    </section>
  );
}

export function SettingsPage() {
  const { t } = useT();
  const { setUser } = useSession();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const [methods, setMethods] = useState<LoginMethod[]>(['email']);

  useEffect(() => {
    let live = true;
    api.me.get().then(
      (me) => live && setLoad({ status: 'ok', me }),
      (error: unknown) => live && setLoad({ status: 'error', error }),
    );
    api.session.methods().then(
      (r) => live && setMethods(r.items),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, []);

  if (load.status === 'loading') return <Empty title={t('common.loading')} />;
  if (load.status === 'error') return <Empty title={errorsByField(load.error)[''] ?? t('settings.loadFailed')} />;

  const me = load.me;
  // Keep the page and the session (top bar name, language) in step with what the server saved.
  const saved = (u: AdminUser) => {
    setLoad({ status: 'ok', me: u });
    setUser({ id: u.id, username: u.username, displayName: u.displayName, email: u.email, directoryDepartment: u.directoryDepartment, language: u.language, isAdmin: u.isAdmin, avatarUrl: u.avatarUrl });
  };

  return (
    <>
      <h1 className="text-2xl font-bold">{t('settings.title')}</h1>
      <ProfileSection me={me} onSaved={saved} />
      <AvatarSection me={me} onSaved={saved} />
      <IdentitiesSection me={me} methods={methods} onChanged={saved} />
      {me.identities.some((i) => i.kind === 'email') && <PasswordSection />}
      <Section title={t('settings.security.title')}>
        <SecondFactorPanel />
      </Section>
      <LanguageSection onSaved={saved} />
    </>
  );
}

// ---------------------------------------------------------------- profile

function ProfileSection({ me, onSaved }: { me: AdminUser; onSaved: (u: AdminUser) => void }) {
  const { t } = useT();
  // The directory owns the display name and email for a user with an LDAP identity (spec: read-only, LDAP wins).
  const ldapBound = me.identities.some((i) => i.kind === 'ldap');
  const [form, setForm] = useState({ displayName: me.displayName, phone: me.phone });
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => {
    setDone(false);
    setForm((f) => ({ ...f, [k]: e.target.value }));
  };

  // When the identities change (e.g. LDAP was just bound) the server may have replaced the values.
  useEffect(() => {
    setForm({ displayName: me.displayName, phone: me.phone });
  }, [me.displayName, me.phone]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setDone(false);
    try {
      // Send only what changed, so untouched LDAP-managed fields are never submitted.
      const changes: Partial<typeof form> = {};
      for (const k of ['displayName', 'phone'] as const) {
        if (form[k] !== me[k]) changes[k] = form[k];
      }
      onSaved(await api.me.update(changes));
      setDone(true);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Section title={t('settings.profile.title')}>
      <form onSubmit={submit} noValidate className="grid max-w-[480px] gap-3.5">
        {errors[''] && (
          <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
            {errors['']}
          </p>
        )}
        <Field label={t('settings.profile.username')}>
          <Input name="username" disabled value={me.username} readOnly />
        </Field>
        <Field label={t('users.field.displayName')} error={errors.displayName}>
          <Input name="displayName" disabled={ldapBound} value={form.displayName} onChange={set('displayName')} />
        </Field>
        <Field label={t('users.field.email')}>
          <Input name="email" type="email" disabled readOnly value={me.email} />
          <p className="mt-1 text-[12px] text-text-3">{ldapBound ? t('settings.profile.ldapManaged') : t('settings.profile.contactEmail')}</p>
        </Field>
        {me.directoryDepartment !== '' && (
          <Field label={t('users.field.directoryDepartment')}>
            <Input name="directoryDepartment" disabled readOnly value={me.directoryDepartment} />
          </Field>
        )}
        <Field label={t('users.field.phone')} error={errors.phone}>
          <Input name="phone" type="tel" value={form.phone} onChange={set('phone')} />
        </Field>
        <div className="flex items-center gap-3">
          <Button kind="primary" type="submit" disabled={busy}>
            {busy ? t('users.saving') : t('users.save')}
          </Button>
          {done && (
            <span role="status" className="text-[13px] text-ok">
              {t('settings.profile.saved')}
            </span>
          )}
        </div>
      </form>
    </Section>
  );
}

// ---------------------------------------------------------------- avatar

// The server accepts 2 MB; checking here only saves a pointless upload. Everything else about
// the file (type, pixels, orientation) is the server's decision and comes back as an error code.
const AVATAR_MAX_BYTES = 2 * 1024 * 1024;

function AvatarSection({ me, onSaved }: { me: AdminUser; onSaved: (u: AdminUser) => void }) {
  const { t } = useT();
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const [preview, setPreview] = useState<string | null>(null);
  const [tooBig, setTooBig] = useState(false);
  useEffect(() => () => { if (preview) URL.revokeObjectURL(preview); }, [preview]);

  const run = async (action: () => Promise<AdminUser>) => {
    setBusy(true);
    setError(null);
    try {
      onSaved(await action());
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const choose = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = ''; // the same file can be chosen again
    if (!file) return;
    setError(null);
    setTooBig(file.size > AVATAR_MAX_BYTES);
    if (file.size > AVATAR_MAX_BYTES) return;
    setPreview(URL.createObjectURL(file)); // shown while uploading
    await run(() => api.me.uploadAvatar(file));
    setPreview(null);
  };

  const message = tooBig ? t('error.avatarTooLarge') : error ? (errorsByField(error).avatar ?? errorsByField(error)[''] ?? '') : '';
  return (
    <Section title={t('settings.avatar.title')}>
      <div className="flex items-center gap-5">
        <Avatar size="xl" name={me.displayName} url={preview ?? me.avatarUrl} />
        <div className="grid gap-2">
          <div className="flex items-center gap-2">
            <label className={`inline-flex h-8 cursor-pointer items-center rounded border border-line bg-white px-3.5 font-semibold hover:bg-subtle has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-accent ${busy ? 'pointer-events-none opacity-60' : ''}`}>
              {busy ? t('settings.avatar.uploading') : me.avatarUrl ? t('settings.avatar.change') : t('settings.avatar.upload')}
              <input type="file" accept="image/jpeg,image/png,image/webp" className="sr-only" disabled={busy} onChange={choose} />
            </label>
            {me.avatarUrl && (
              <Button disabled={busy} onClick={() => run(() => api.me.removeAvatar())}>
                {t('settings.avatar.remove')}
              </Button>
            )}
          </div>
          <p className="text-[12px] text-text-3">{t('settings.avatar.hint')}</p>
          {message && (
            <p role="alert" className="text-[13px] text-danger">
              {message}
            </p>
          )}
        </div>
      </div>
    </Section>
  );
}

// ---------------------------------------------------------------- login identities

const hasEmailIdentity = (me: AdminUser) => me.identities.some((i) => i.kind === 'email');

function IdentitiesSection({ me, methods, onChanged }: { me: AdminUser; methods: LoginMethod[]; onChanged: (u: AdminUser) => void }) {
  const { t, locale } = useT();
  const [binding, setBinding] = useState<IdentityKind | null>(null);
  const [unbinding, setUnbinding] = useState<IdentityKind | null>(null);
  const [emailDialog, setEmailDialog] = useState<'bind' | 'change' | null>(null);
  // Adding an email login and changing the login email send a code by mail; without mail they are unavailable.
  const [mailOn, setMailOn] = useState<boolean | null>(null);
  useEffect(() => {
    let live = true;
    api.mail.status().then(
      (r) => live && setMailOn(r.configured),
      () => live && setMailOn(false),
    );
    return () => {
      live = false;
    };
  }, []);
  const emailMode = hasEmailIdentity(me) ? 'change' : 'bind';
  const have = new Set(me.identities.map((i) => i.kind));
  // The directory is bound with its password; an email identity has its own flow below (the address is verified by mail).
  const missing = methods.filter((m) => m === 'ldap' && !have.has(m));
  const onlyOne = me.identities.length <= 1;


  return (
    <Section title={t('settings.identities.title')}>
      <div className="overflow-x-auto rounded border border-line">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('settings.identities.col.type')}</th>
              <th className={th}>{t('settings.identities.col.loginName')}</th>
              <th className={th}>{t('settings.identities.col.boundAt')}</th>
              <th className={th}>{t('settings.identities.col.boundVia')}</th>
              <th className={th}>{t('settings.identities.col.actions')}</th>
            </tr>
          </thead>
          <tbody>
            {me.identities.map((i) => (
              <tr key={i.kind}>
                <td className={td}>{t(`users.identity.${i.kind}`)}</td>
                <td className={`${td} font-mono text-[12.5px]`}>{i.loginName}</td>
                <td className={`${td} whitespace-nowrap`}>{new Date(i.boundAt).toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-GB')}</td>
                <td className={td}>{t(`settings.via.${i.boundVia}`)}</td>
                <td className={`${td} whitespace-nowrap`}>
                  <Button small disabled={onlyOne} onClick={() => setUnbinding(i.kind)}>
                    {t('settings.unbind')}
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {onlyOne && <p className="mt-2 text-[13px] text-text-2">{t('settings.unbind.lastReason')}</p>}
      <div className="mt-3 flex flex-wrap gap-2">
        {missing.map((k) => (
          <Button key={k} onClick={() => setBinding(k)}>
            {t('settings.bind.ldap')}
          </Button>
        ))}
        <Button disabled={mailOn !== true} onClick={() => setEmailDialog(emailMode)}>
          {emailMode === 'bind' ? t('settings.emailLogin.add') : t('settings.emailLogin.change')}
        </Button>
      </div>
      {mailOn === false && <p className="mt-2 text-[13px] text-text-2">{t('settings.emailLogin.unavailable')}</p>}
      {emailDialog !== null && (
        <EmailLoginDialog
          mode={emailDialog}
          ldapBound={have.has('ldap')}
          onClose={() => setEmailDialog(null)}
          onDone={(u) => {
            setEmailDialog(null);
            onChanged(u);
          }}
        />
      )}

      {binding !== null && (
        <BindIdentityDialog
          audience="self"
          username={me.username}
          kinds={[binding]}
          bind={api.me.bindIdentity}
          onClose={() => setBinding(null)}
          onBound={(u) => {
            setBinding(null);
            onChanged(u);
          }}
        />
      )}
      {unbinding !== null && (
        <UnbindDialog
          kind={unbinding}
          onClose={() => setUnbinding(null)}
          onUnbound={(u) => {
            setUnbinding(null);
            onChanged(u);
          }}
        />
      )}
    </Section>
  );
}

function UnbindDialog({ kind, onClose, onUnbound }: { kind: IdentityKind; onClose: () => void; onUnbound: (u: AdminUser) => void }) {
  const { t } = useT();
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};
  const message = errors[''] ?? errors.kind;

  const confirm = async () => {
    setBusy(true);
    setError(null);
    try {
      onUnbound(await api.me.unbindIdentity(kind));
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
        <Dialog.Popup className="fixed left-1/2 top-[10vh] w-[440px] -translate-x-1/2 rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-2 text-lg font-bold">{t('settings.unbind.title', { kind: t(`users.identity.${kind}`) })}</Dialog.Title>
          <Dialog.Description className="text-text-2">{t('settings.unbind.body')}</Dialog.Description>
          {message && (
            <p role="alert" className="mt-3 rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
              {message}
            </p>
          )}
          <div className="mt-5 flex justify-end gap-2">
            <Button type="button" onClick={onClose}>
              {t('users.cancel')}
            </Button>
            <Button kind="danger" type="button" disabled={busy} onClick={confirm}>
              {t('settings.unbind')}
            </Button>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

// ---------------------------------------------------------------- password

function PasswordSection() {
  const { t } = useT();
  const [done, setDone] = useState(false);
  return (
    <Section title={t('password.title')}>
      <p className="mb-3 text-[13px] text-text-2">{t('password.hint')}</p>
      <ChangePasswordForm onDone={() => setDone(true)} />
      {done && (
        <p role="status" className="mt-2 text-[13px] text-ok">
          {t('password.changed')}
        </p>
      )}
    </Section>
  );
}

// ---------------------------------------------------------------- language

function LanguageSection({ onSaved }: { onSaved: (u: AdminUser) => void }) {
  const { t, locale } = useT();
  const [error, setError] = useState<unknown>(null);
  const change = async (l: Locale) => {
    setLocale(l);
    setError(null);
    try {
      onSaved(await api.me.update({ language: l }));
    } catch (err) {
      setError(err);
    }
  };
  return (
    <Section title={t('settings.language.title')}>
      <Segmented label={t('common.language')} value={locale} onChange={change} options={LOCALES.map((l) => ({ value: l, label: t(`lang.${l}`) }))} />
      <p className="mt-2 text-[13px] text-text-2">{t('settings.language.hint')}</p>
      {error !== null && (
        <p role="alert" className="mt-2 text-[13px] text-danger">
          {errorsByField(error)['']}
        </p>
      )}
    </Section>
  );
}
