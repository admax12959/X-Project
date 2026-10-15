// Administration > Storage (#24, ADR-0005, ADR-0007): where avatars and other files are kept. Built like the LDAP page:
// load -> form -> test -> save. Both parts of the access key are write-only: the page shows whether they are stored,
// never the value, and anything typed here is dropped from the state as soon as it was sent. "Test connection" sends
// the unsaved form (key fields left empty fall back to the stored key on the server).
import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { Navigate } from 'react-router';
import { api } from '../api/client.ts';
import { errorMessage, errorsByField } from '../api/errors.ts';
import type { ConnectionTest, StorageConfig, StorageConfigInput, StorageDriver } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Container, Empty, Field, Input, Segmented, Select } from '../ui/controls.tsx';

type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; config: StorageConfig };

/** The form: the config minus what the server computes, plus the key being typed. */
type Form = StorageConfigInput & { accessKeyId: string; secretAccessKey: string };

function toForm(c: StorageConfig): Form {
  return {
    driver: c.driver, endpoint: c.endpoint, region: c.region, bucket: c.bucket, addressing: c.addressing,
    accessKeyId: '', secretAccessKey: '',
  };
}

/** The page is for administrators; anyone else is sent home (the API also answers 403). */
export function StorageConfigPage() {
  const user = useCurrentUser();
  if (!user.isAdmin) return <Navigate to="/" replace />;
  return <StorageConfigView />;
}

function StorageConfigView() {
  const { t } = useT();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const reload = useCallback(() => {
    setLoad({ status: 'loading' });
    api.systemConfig.storage.get().then(
      (config) => setLoad({ status: 'ok', config }),
      (error: unknown) => setLoad({ status: 'error', error }),
    );
  }, []);
  useEffect(reload, [reload]);

  return (
    <>
      <div>
        <h1 className="text-2xl font-bold">{t('storage.title')}</h1>
        <p className="mt-1 max-w-[760px] text-text-2">{t('storage.intro')}</p>
      </div>
      {load.status === 'loading' && <Empty title={t('common.loading')} />}
      {load.status === 'error' && (
        <Empty title={errorsByField(load.error)[''] ?? t('storage.loadFailed')}>
          <Button small onClick={reload}>
            {t('common.retry')}
          </Button>
        </Empty>
      )}
      {load.status === 'ok' && <StorageForm config={load.config} onSaved={(config) => setLoad({ status: 'ok', config })} />}
    </>
  );
}

function StorageForm({ config, onSaved }: { config: StorageConfig; onSaved: (c: StorageConfig) => void }) {
  const { t } = useT();
  const [form, setForm] = useState<Form>(() => toForm(config));
  const [error, setError] = useState<unknown>(null); // raw, rendered at render time so it follows a language switch
  const [busy, setBusy] = useState<'save' | 'test' | null>(null);
  const [saved, setSaved] = useState(false);
  const [test, setTest] = useState<ConnectionTest | null>(null);
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  const set = <K extends keyof Form>(k: K, v: Form[K]) => {
    setForm((f) => ({ ...f, [k]: v }));
    setSaved(false);
  };
  const text = (k: 'endpoint' | 'region' | 'bucket' | 'accessKeyId' | 'secretAccessKey') => (e: { target: { value: string } }) => set(k, e.target.value);

  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy('save');
    setError(null);
    setTest(null);
    try {
      const next = await api.systemConfig.storage.update(form);
      setForm(toForm(next)); // the typed key is not kept
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
      setTest(await api.systemConfig.storage.test(form));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(null);
    }
  };

  const keyHint = (isSet: boolean) => (config.secretsUnreadable ? t('storage.key.unreadable') : isSet ? t('storage.key.set') : t('storage.key.unset'));

  return (
    <form onSubmit={save} noValidate className="max-w-[760px] space-y-5">
      {errors[''] && (
        <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
          {errors['']}
        </p>
      )}

      <Container title={t('storage.section.driver')}>
        <div className="grid gap-3.5">
          <Field label={t('storage.field.driver')} error={errors.driver}>
            <div>
              <Segmented<StorageDriver>
                label={t('storage.field.driver')}
                value={form.driver}
                onChange={(v) => set('driver', v)}
                options={[
                  { value: 'local', label: t('storage.driver.local') },
                  { value: 's3', label: t('storage.driver.s3') },
                ]}
              />
            </div>
          </Field>
          {form.driver === 'local' && <Hint>{t('storage.local.info', { dir: config.localDir })}</Hint>}
        </div>
      </Container>

      {form.driver === 's3' && (
        <Container title={t('storage.section.s3')}>
          <div className="grid gap-3.5">
            <Field label={t('storage.field.endpoint')} error={errors.endpoint}>
              <Input name="endpoint" value={form.endpoint} onChange={text('endpoint')} placeholder="https://oss-cn-hangzhou.aliyuncs.com" autoComplete="off" spellCheck={false} />
              <Hint>{t('storage.hint.endpoint')}</Hint>
            </Field>
            <div className="grid grid-cols-2 gap-3.5">
              <Field label={t('storage.field.region')} error={errors.region}>
                <Input name="region" value={form.region} onChange={text('region')} placeholder="oss-cn-hangzhou" autoComplete="off" spellCheck={false} />
              </Field>
              <Field label={t('storage.field.bucket')} error={errors.bucket}>
                <Input name="bucket" value={form.bucket} onChange={text('bucket')} autoComplete="off" spellCheck={false} />
              </Field>
            </div>
            <Hint>{t('storage.hint.region')}</Hint>
            <Hint>{t('storage.hint.bucket')}</Hint>
            <Field label={t('storage.field.addressing')} error={errors.addressing}>
              <Select name="addressing" className="w-64" value={form.addressing} onChange={(e) => set('addressing', e.target.value as Form['addressing'])}>
                <option value="path">{t('storage.addressing.path')}</option>
                <option value="virtualHost">{t('storage.addressing.virtualHost')}</option>
              </Select>
              <Hint>{t('storage.hint.addressing')}</Hint>
            </Field>
            <Field label={t('storage.field.accessKeyId')} error={errors.accessKeyId}>
              <Input name="accessKeyId" type="password" autoComplete="new-password" value={form.accessKeyId} onChange={text('accessKeyId')} />
              <Hint>{keyHint(config.accessKeyIdSet)}</Hint>
            </Field>
            <Field label={t('storage.field.secretAccessKey')} error={errors.secretAccessKey}>
              <Input name="secretAccessKey" type="password" autoComplete="new-password" value={form.secretAccessKey} onChange={text('secretAccessKey')} />
              <Hint>{keyHint(config.secretAccessKeySet)}</Hint>
            </Field>
          </div>
        </Container>
      )}

      <div className="flex flex-wrap items-center gap-3">
        <Button kind="primary" type="submit" disabled={busy !== null}>
          {busy === 'save' ? t('storage.saving') : t('storage.save')}
        </Button>
        <Button type="button" onClick={() => void runTest()} disabled={busy !== null}>
          {busy === 'test' ? t('storage.testing') : t('storage.test')}
        </Button>
        {saved && (
          <span role="status" className="text-[13px] text-ok">
            {t('storage.saved')}
          </span>
        )}
        {test?.ok && (
          <span role="status" className="text-[13px] text-ok">
            {t('storage.test.ok')}
          </span>
        )}
        {test && !test.ok && (
          <span role="alert" className="text-[13px] text-danger">
            {t('storage.test.failed', { reason: errorMessage({ code: test.code ?? 'internal' }) })}
          </span>
        )}
      </div>
    </form>
  );
}

function Hint({ children }: { children: ReactNode }) {
  return <p className="mt-1 text-[12px] text-text-3">{children}</p>;
}
