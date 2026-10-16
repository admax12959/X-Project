// Administration > Email (#20, ADR-0005): the SMTP server, the site's external address mail links are built
// from, and the outgoing mail queue. Same pattern as the LDAP page: load -> form -> test -> save. The password
// is write-only: the page shows whether one is set, never the value, and a password typed here is dropped from
// the state as soon as it was sent. "Send test email" sends the unsaved form to the administrator's own address.
import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { Navigate } from 'react-router';
import { api } from '../api/client.ts';
import { errorMessage, errorsByField } from '../api/errors.ts';
import type { MailTestResult, Outbox, SmtpConfig, SmtpConfigInput, SmtpSecurity } from '../api/types.ts';
import { hasKey, useT, type MessageKey } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Container, Empty, Field, Input, Select, Textarea } from '../ui/controls.tsx';
import { td, th } from '../ui/table.ts';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; config: SmtpConfig };

/** The form: the config minus what the server computes, plus the password being typed. */
type Form = Omit<SmtpConfigInput, 'password' | 'port'> & { password: string; port: string };

const DEFAULT_PORT: Record<SmtpSecurity, number> = { tls: 465, starttls: 587, none: 25 };

function toForm(c: SmtpConfig): Form {
  return {
    enabled: c.enabled, host: c.host, port: String(c.port), security: c.security, allowPlaintext: c.allowPlaintext, username: c.username,
    password: '', from: c.from, caCert: c.caCert, siteUrl: c.siteUrl,
  };
}

/** The page is for administrators; anyone else is sent home (the API also answers 403). */
export function MailConfigPage() {
  const user = useCurrentUser();
  if (!user.isAdmin) return <Navigate to="/" replace />;
  return <MailConfigView />;
}

function MailConfigView() {
  const { t } = useT();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const reload = useCallback(() => {
    setLoad({ status: 'loading' });
    api.systemConfig.smtp.get().then(
      (config) => setLoad({ status: 'ok', config }),
      (error: unknown) => setLoad({ status: 'error', error }),
    );
  }, []);
  useEffect(reload, [reload]);

  return (
    <>
      <div>
        <h1 className="text-2xl font-bold">{t('smtp.title')}</h1>
        <p className="mt-1 max-w-[760px] text-text-2">{t('smtp.intro')}</p>
      </div>
      {load.status === 'loading' && <Empty title={t('common.loading')} />}
      {load.status === 'error' && (
        <Empty title={errorsByField(load.error)[''] ?? t('smtp.loadFailed')}>
          <Button small onClick={reload}>
            {t('common.retry')}
          </Button>
        </Empty>
      )}
      {load.status === 'ok' && (
        <>
          {!load.config.configured && (
            <p role="status" className="max-w-[760px] rounded border border-line bg-white px-3 py-2 text-[13px] text-text-2">
              {t('smtp.notConfigured')}
            </p>
          )}
          <MailForm config={load.config} onSaved={(config) => setLoad({ status: 'ok', config })} />
          <OutboxPanel />
        </>
      )}
    </>
  );
}

function MailForm({ config, onSaved }: { config: SmtpConfig; onSaved: (c: SmtpConfig) => void }) {
  const { t } = useT();
  const [form, setForm] = useState<Form>(() => toForm(config));
  const [error, setError] = useState<unknown>(null); // raw, rendered at render time so it follows a language switch
  const [busy, setBusy] = useState<'save' | 'test' | null>(null);
  const [saved, setSaved] = useState(false);
  const [test, setTest] = useState<MailTestResult | null>(null);
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  const text = (k: 'host' | 'port' | 'username' | 'password' | 'from' | 'siteUrl' | 'caCert') => (e: { target: { value: string } }) => {
    setForm((f) => ({ ...f, [k]: e.target.value }));
    setSaved(false);
  };
  const flag = (k: 'enabled' | 'allowPlaintext') => (e: { target: { checked: boolean } }) => {
    setForm((f) => ({ ...f, [k]: e.target.checked }));
    setSaved(false);
  };
  // Changing the encryption moves the port to its usual value, unless the administrator typed another one.
  const security = (e: { target: { value: string } }) => {
    const next = e.target.value as SmtpSecurity;
    setForm((f) => ({
      ...f,
      security: next,
      allowPlaintext: next === 'none' ? f.allowPlaintext : false,
      port: Number(f.port) === DEFAULT_PORT[f.security] ? String(DEFAULT_PORT[next]) : f.port,
    }));
    setSaved(false);
  };
  const body = (): SmtpConfigInput => ({ ...form, port: Number(form.port) || 0 });

  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy('save');
    setError(null);
    setTest(null);
    try {
      const next = await api.systemConfig.smtp.update(body());
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
      setTest(await api.systemConfig.smtp.test(body()));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(null);
    }
  };

  const passwordHint = config.secretsUnreadable
    ? t('smtp.password.unreadable')
    : config.passwordSet && form.username
      ? t('smtp.password.set')
      : t('smtp.password.unset');

  return (
    <form onSubmit={save} noValidate className="max-w-[760px] space-y-5">
      {errors[''] && (
        <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
          {errors['']}
        </p>
      )}

      <Container title={t('smtp.section.server')}>
        <div className="grid gap-3.5">
          <Check label={t('smtp.field.enabled')} text={t('smtp.enabled.label')} checked={form.enabled} onChange={flag('enabled')} error={errors.enabled} />
          <div className="grid grid-cols-[1fr_120px] gap-3.5">
            <Field label={t('smtp.field.host')} error={errors.host}>
              <Input name="host" value={form.host} onChange={text('host')} placeholder="smtp.example.com" autoComplete="off" spellCheck={false} />
              <Hint>{t('smtp.hint.host')}</Hint>
            </Field>
            <Field label={t('smtp.field.port')} error={errors.port}>
              <Input name="port" type="number" min={1} max={65535} value={form.port} onChange={text('port')} className="w-28" />
            </Field>
          </div>
          <Field label={t('smtp.field.security')} error={errors.security}>
            <Select name="security" value={form.security} onChange={security}>
              {(['tls', 'starttls', 'none'] as const).map((s) => (
                <option key={s} value={s}>
                  {t(`smtp.security.${s}`)}
                </option>
              ))}
            </Select>
            <Hint>{t('smtp.hint.security')}</Hint>
          </Field>
          {form.security === 'none' && (
            <Check label={t('smtp.field.allowPlaintext')} text={t('smtp.plaintext.label')} checked={form.allowPlaintext} onChange={flag('allowPlaintext')} error={errors.allowPlaintext}>
              {t('smtp.hint.plaintext')}
            </Check>
          )}
          <div className="grid grid-cols-2 gap-3.5">
            <Field label={t('smtp.field.username')} error={errors.username}>
              <Input name="username" value={form.username} onChange={text('username')} autoComplete="off" spellCheck={false} />
              <Hint>{t('smtp.hint.username')}</Hint>
            </Field>
            <Field label={t('smtp.field.password')} error={errors.password}>
              <Input name="password" type="password" autoComplete="new-password" value={form.password} onChange={text('password')} />
              <Hint>{passwordHint}</Hint>
            </Field>
          </div>
          <Field label={t('smtp.field.from')} error={errors.from}>
            <Input name="from" value={form.from} onChange={text('from')} placeholder="X-Project <noreply@example.com>" autoComplete="off" spellCheck={false} />
            <Hint>{t('smtp.hint.from')}</Hint>
          </Field>
          <Field label={t('smtp.field.caCert')} error={errors.caCert}>
            <Textarea name="caCert" value={form.caCert} onChange={text('caCert')} rows={4} className="font-mono text-[12px]" placeholder="-----BEGIN CERTIFICATE-----" spellCheck={false} />
            <Hint>{t('smtp.hint.caCert')}</Hint>
          </Field>
        </div>
      </Container>

      <Container title={t('smtp.section.site')}>
        <Field label={t('smtp.field.siteUrl')} error={errors.siteUrl}>
          <Input name="siteUrl" value={form.siteUrl} onChange={text('siteUrl')} placeholder="https://xp.example.com" autoComplete="off" spellCheck={false} />
          <Hint>{t('smtp.hint.siteUrl')}</Hint>
        </Field>
      </Container>

      <div className="flex flex-wrap items-center gap-3">
        <Button kind="primary" type="submit" disabled={busy !== null}>
          {busy === 'save' ? t('smtp.saving') : t('smtp.save')}
        </Button>
        <Button type="button" onClick={() => void runTest()} disabled={busy !== null}>
          {busy === 'test' ? t('smtp.testing') : t('smtp.test')}
        </Button>
        {saved && (
          <span role="status" className="text-[13px] text-ok">
            {t('smtp.saved')}
          </span>
        )}
        {test?.ok && (
          <span role="status" className="text-[13px] text-ok">
            {t('smtp.test.ok', { to: test.to })}
          </span>
        )}
      </div>
      {test && !test.ok && (
        <div role="alert" className="text-[13px] text-danger">
          <p>{t('smtp.test.failed', { reason: errorMessage({ code: test.code ?? 'internal' }) })}</p>
          {test.detail && <p className="mt-1 break-words font-mono text-[12px]">{t('smtp.test.detail', { detail: test.detail })}</p>}
        </div>
      )}
    </form>
  );
}

type OutboxLoad = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; failed: Outbox };

/** The outgoing mail queue: how much is waiting, and the mail that was given up on. */
function OutboxPanel() {
  const { t, locale } = useT();
  const [load, setLoad] = useState<OutboxLoad>({ status: 'loading' });
  const reload = useCallback(() => {
    api.mail.outbox('failed').then(
      (failed) => setLoad({ status: 'ok', failed }),
      (error: unknown) => setLoad({ status: 'error', error }),
    );
  }, []);
  useEffect(reload, [reload]);
  const when = new Intl.DateTimeFormat(locale === 'zh' ? 'zh-CN' : 'en', { dateStyle: 'medium', timeStyle: 'short' });
  const templateLabel = (name: string) => (hasKey(`smtp.template.${name}`) ? t(`smtp.template.${name}` as MessageKey) : name);

  return (
    <div className="max-w-[1100px]">
      <Container
        title={t('smtp.section.queue')}
        actions={
          <Button small onClick={reload}>
            {t('smtp.queue.refresh')}
          </Button>
        }
      >
        <p className="max-w-[760px] text-text-2">{t('smtp.queue.intro')}</p>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && <Empty title={errorsByField(load.error)[''] ?? t('smtp.queue.loadFailed')} />}
        {load.status === 'ok' && (
          <>
            <p className="mt-2 text-[13px] text-text-2">
              {t('smtp.queue.counts', { pending: load.failed.counts.pending, sent: load.failed.counts.sent, failed: load.failed.counts.failed })}
            </p>
            <h3 className="mb-1 mt-4 text-[13px] font-semibold">{t('smtp.queue.failedTitle')}</h3>
            {load.failed.items.length === 0 ? (
              <p className="text-text-3">{t('smtp.queue.noneFailed')}</p>
            ) : (
              <div className="overflow-x-auto rounded border border-line">
                <table className="w-full border-collapse">
                  <thead>
                    <tr>
                      <th className={th}>{t('smtp.queue.col.to')}</th>
                      <th className={th}>{t('smtp.queue.col.template')}</th>
                      <th className={th}>{t('smtp.queue.col.subject')}</th>
                      <th className={th}>{t('smtp.queue.col.attempts')}</th>
                      <th className={th}>{t('smtp.queue.col.error')}</th>
                      <th className={th}>{t('smtp.queue.col.failedAt')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {load.failed.items.map((m) => (
                      <tr key={m.id} className="hover:bg-hover">
                        <td className={td}>{m.to}</td>
                        <td className={`${td} whitespace-nowrap`}>{templateLabel(m.template)}</td>
                        <td className={td}>{m.subject}</td>
                        <td className={td}>{m.attempts}</td>
                        <td className={`${td} break-words text-danger`}>
                          {m.lastErrorCode && hasKey(`error.${m.lastErrorCode}`) ? errorMessage({ code: m.lastErrorCode }) : ''}
                          {m.lastError && <div className="font-mono text-[12px] text-text-2">{m.lastError}</div>}
                        </td>
                        <td className={`${td} whitespace-nowrap`}>{m.failedAt ? when.format(new Date(m.failedAt)) : ''}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </>
        )}
      </Container>
    </div>
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
