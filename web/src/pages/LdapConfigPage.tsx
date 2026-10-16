// Administration > LDAP (#19, ADR-0005): the corporate directory, edited in the database instead of the config file.
// The service account password is write-only: the page shows whether one is set, never the value, and a
// password typed here is dropped from the state as soon as it was sent. "Test connection" sends the unsaved form.
// This page is the pattern for the other system configuration pages (mail, storage): load -> form -> test -> save.
import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { Navigate } from 'react-router';
import { api } from '../api/client.ts';
import { errorMessage, errorsByField } from '../api/errors.ts';
import type { ConnectionTest, LdapConfig, LdapConfigInput } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Container, Empty, Field, Input } from '../ui/controls.tsx';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; config: LdapConfig };

/** The form: the config minus what the server computes, plus the password being typed. */
type Form = LdapConfigInput & { bindPassword: string };

function toForm(c: LdapConfig): Form {
  return {
    enabled: c.enabled, url: c.url, startTls: c.startTls, skipVerify: c.skipVerify, bindDn: c.bindDn, bindPassword: '',
    baseDn: c.baseDn, userFilter: c.userFilter, loginAttr: c.loginAttr, nameAttr: c.nameAttr, emailAttr: c.emailAttr,
    departmentAttr: c.departmentAttr, disabledFilter: c.disabledFilter, timeoutSeconds: c.timeoutSeconds,
  };
}

/** The page is for administrators; anyone else is sent home (the API also answers 403). */
export function LdapConfigPage() {
  const user = useCurrentUser();
  if (!user.isAdmin) return <Navigate to="/" replace />;
  return <LdapConfigView />;
}

function LdapConfigView() {
  const { t } = useT();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const reload = useCallback(() => {
    setLoad({ status: 'loading' });
    api.systemConfig.ldap.get().then(
      (config) => setLoad({ status: 'ok', config }),
      (error: unknown) => setLoad({ status: 'error', error }),
    );
  }, []);
  useEffect(reload, [reload]);

  return (
    <>
      <div>
        <h1 className="text-2xl font-bold">{t('ldap.title')}</h1>
        <p className="mt-1 max-w-[760px] text-text-2">{t('ldap.intro')}</p>
      </div>
      {load.status === 'loading' && <Empty title={t('common.loading')} />}
      {load.status === 'error' && (
        <Empty title={errorsByField(load.error)[''] ?? t('ldap.loadFailed')}>
          <Button small onClick={reload}>
            {t('common.retry')}
          </Button>
        </Empty>
      )}
      {load.status === 'ok' && <LdapForm config={load.config} onSaved={(config) => setLoad({ status: 'ok', config })} />}
    </>
  );
}

function LdapForm({ config, onSaved }: { config: LdapConfig; onSaved: (c: LdapConfig) => void }) {
  const { t } = useT();
  const [form, setForm] = useState<Form>(() => toForm(config));
  const [error, setError] = useState<unknown>(null); // raw, rendered at render time so it follows a language switch
  const [busy, setBusy] = useState<'save' | 'test' | null>(null);
  const [saved, setSaved] = useState(false);
  const [test, setTest] = useState<ConnectionTest | null>(null);
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  const text = (k: keyof Form) => (e: { target: { value: string } }) => {
    setForm((f) => ({ ...f, [k]: e.target.value }));
    setSaved(false);
  };
  const flag = (k: 'enabled' | 'startTls' | 'skipVerify') => (e: { target: { checked: boolean } }) => {
    setForm((f) => ({ ...f, [k]: e.target.checked }));
    setSaved(false);
  };
  const body = (): LdapConfigInput => ({ ...form, timeoutSeconds: Number(form.timeoutSeconds) || 0 });

  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy('save');
    setError(null);
    setTest(null);
    try {
      const next = await api.systemConfig.ldap.update(body());
      setForm(toForm(next)); // the typed password is not kept
      onSaved(next);
      setSaved(true);
    } catch (err) {
      setError(err);
      setSaved(false);
    } finally {
      setBusy(null);
    }
  };

  const runTest = async () => {
    setBusy('test');
    setError(null);
    setSaved(false);
    setTest(null);
    try {
      setTest(await api.systemConfig.ldap.test(body()));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(null);
    }
  };

  const passwordHint = config.secretsUnreadable
    ? t('ldap.password.unreadable')
    : config.bindPasswordSet && form.bindDn
      ? t('ldap.password.set')
      : t('ldap.password.unset');

  return (
    <form onSubmit={save} noValidate className="max-w-[760px] space-y-5">
      {errors[''] && (
        <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
          {errors['']}
        </p>
      )}

      <Container title={t('ldap.section.connection')}>
        <div className="grid gap-3.5">
          <Check label={t('ldap.field.enabled')} text={t('ldap.enabled.label')} checked={form.enabled} onChange={flag('enabled')} error={errors.enabled} />
          <Field label={t('ldap.field.url')} error={errors.url}>
            <Input name="url" value={form.url} onChange={text('url')} placeholder="ldaps://ldap.example.com:636" autoComplete="off" spellCheck={false} />
            <Hint>{t('ldap.hint.url')}</Hint>
          </Field>
          <Check label={t('ldap.field.startTls')} text={t('ldap.field.startTls')} checked={form.startTls} onChange={flag('startTls')} error={errors.startTls} />
          <Check label={t('ldap.field.skipVerify')} text={t('ldap.field.skipVerify')} checked={form.skipVerify} onChange={flag('skipVerify')} error={errors.skipVerify}>
            {t('ldap.hint.skipVerify')}
          </Check>
          <Field label={t('ldap.field.bindDn')} error={errors.bindDn}>
            <Input name="bindDn" value={form.bindDn} onChange={text('bindDn')} autoComplete="off" spellCheck={false} />
            <Hint>{t('ldap.hint.bindDn')}</Hint>
          </Field>
          <Field label={t('ldap.field.bindPassword')} error={errors.bindPassword}>
            <Input name="bindPassword" type="password" autoComplete="new-password" value={form.bindPassword} onChange={text('bindPassword')} />
            <Hint>{passwordHint}</Hint>
          </Field>
          <Field label={t('ldap.field.timeoutSeconds')} error={errors.timeoutSeconds}>
            <Input name="timeoutSeconds" type="number" min={1} className="w-28" value={String(form.timeoutSeconds)} onChange={text('timeoutSeconds')} />
          </Field>
        </div>
      </Container>

      <Container title={t('ldap.section.attributes')}>
        <div className="grid gap-3.5">
          <Field label={t('ldap.field.baseDn')} error={errors.baseDn}>
            <Input name="baseDn" value={form.baseDn} onChange={text('baseDn')} placeholder="ou=people,dc=example,dc=com" autoComplete="off" spellCheck={false} />
            <Hint>{t('ldap.hint.baseDn')}</Hint>
          </Field>
          <Field label={t('ldap.field.userFilter')} error={errors.userFilter}>
            <Input name="userFilter" value={form.userFilter} onChange={text('userFilter')} autoComplete="off" spellCheck={false} />
            <Hint>{t('ldap.hint.userFilter')}</Hint>
          </Field>
          <div className="grid grid-cols-2 gap-3.5">
            <Field label={t('ldap.field.loginAttr')} error={errors.loginAttr}>
              <Input name="loginAttr" value={form.loginAttr} onChange={text('loginAttr')} autoComplete="off" spellCheck={false} />
            </Field>
            <Field label={t('ldap.field.nameAttr')} error={errors.nameAttr}>
              <Input name="nameAttr" value={form.nameAttr} onChange={text('nameAttr')} autoComplete="off" spellCheck={false} />
            </Field>
            <Field label={t('ldap.field.emailAttr')} error={errors.emailAttr}>
              <Input name="emailAttr" value={form.emailAttr} onChange={text('emailAttr')} autoComplete="off" spellCheck={false} />
            </Field>
            <Field label={t('ldap.field.departmentAttr')} error={errors.departmentAttr}>
              <Input name="departmentAttr" value={form.departmentAttr} onChange={text('departmentAttr')} autoComplete="off" spellCheck={false} />
            </Field>
          </div>
          <Field label={t('ldap.field.disabledFilter')} error={errors.disabledFilter}>
            <Input name="disabledFilter" value={form.disabledFilter} onChange={text('disabledFilter')} autoComplete="off" spellCheck={false} />
            <Hint>{t('ldap.hint.disabledFilter')}</Hint>
          </Field>
        </div>
      </Container>

      <div className="flex flex-wrap items-center gap-3">
        <Button kind="primary" type="submit" disabled={busy !== null}>
          {busy === 'save' ? t('ldap.saving') : t('ldap.save')}
        </Button>
        <Button type="button" onClick={() => void runTest()} disabled={busy !== null}>
          {busy === 'test' ? t('ldap.testing') : t('ldap.test')}
        </Button>
        {saved && (
          <span role="status" className="text-[13px] text-ok">
            {t('ldap.saved')}
          </span>
        )}
        {test?.ok && (
          <span role="status" className="text-[13px] text-ok">
            {t('ldap.test.ok')}
          </span>
        )}
        {test && !test.ok && (
          <span role="alert" className="text-[13px] text-danger">
            {t('ldap.test.failed', { reason: errorMessage({ code: test.code ?? 'internal' }) })}
          </span>
        )}
      </div>
    </form>
  );
}

function Hint({ children }: { children: ReactNode }) {
  return <p className="mt-1 text-[12px] text-text-3">{children}</p>;
}

/** A labelled checkbox row (the label is the field's name, the text is what ticking it means). */
function Check({
  label, text, checked, onChange, error, children,
}: {
  label: string;
  text: string;
  checked: boolean;
  onChange: (e: { target: { checked: boolean } }) => void;
  error?: string;
  children?: ReactNode;
}) {
  return (
    <div>
      <div className="mb-1 text-[13px] font-semibold">{label}</div>
      <label className="flex items-center gap-2">
        <input type="checkbox" checked={checked} onChange={onChange} />
        {text}
      </label>
      {children && <Hint>{children}</Hint>}
      {error && <p className="mt-1 text-[13px] text-danger">{error}</p>}
    </div>
  );
}
