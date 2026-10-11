// Administration > Code platforms (#4): list registered platforms and register a new one. Administrators only.
import { Dialog } from '@base-ui/react/dialog';
import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { Navigate } from 'react-router';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { CodePlatform, PlatformKind } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { useCurrentUser } from '../session/session.tsx';
import { Button, Empty, Field, Input, Segmented } from '../ui/controls.tsx';
import { th, td } from '../ui/table.ts';

// Errors are kept raw and rendered at render time, so they follow a language switch.
type Load = { status: 'loading' } | { status: 'error'; error: unknown } | { status: 'ok'; items: CodePlatform[] };

const KIND_LABEL: Record<PlatformKind, string> = { gitlab: 'GitLab', github: 'GitHub' };

/** The page is for administrators; anyone else is sent home (the API also answers 403). */
export function PlatformsPage() {
  const user = useCurrentUser();
  if (!user.isAdmin) return <Navigate to="/" replace />;
  return <Platforms />;
}

function Platforms() {
  const { t, locale } = useT();
  const [load, setLoad] = useState<Load>({ status: 'loading' });
  const [dialogOpen, setDialogOpen] = useState(false);

  const reload = useCallback(() => {
    api.codePlatforms.list().then(
      (r) => setLoad({ status: 'ok', items: r.items }),
      (error: unknown) => setLoad({ status: 'error', error }),
    );
  }, []);
  useEffect(reload, [reload]);

  const items = load.status === 'ok' ? load.items : [];
  const registered = new Intl.DateTimeFormat(locale === 'zh' ? 'zh-CN' : 'en', { dateStyle: 'medium', timeStyle: 'short' });

  return (
    <>
      <div className="flex min-h-8 items-center justify-between gap-4">
        <h1 className="flex items-baseline gap-2 text-2xl font-bold">
          {t('platforms.title')}
          {load.status === 'ok' && <span className="font-normal text-text-3">({items.length})</span>}
        </h1>
        <Button kind="primary" onClick={() => setDialogOpen(true)}>
          {t('platforms.register')}
        </Button>
      </div>

      <div className="overflow-x-auto rounded border border-line bg-white">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <th className={th}>{t('platforms.col.kind')}</th>
              <th className={th}>{t('platforms.col.baseUrl')}</th>
              <th className={th}>{t('platforms.col.token')}</th>
              <th className={th}>{t('platforms.col.createdAt')}</th>
            </tr>
          </thead>
          <tbody>
            {items.map((p) => (
              <tr key={p.id} className="hover:bg-hover">
                <td className={td}>{KIND_LABEL[p.kind] ?? p.kind}</td>
                <td className={`${td} font-mono text-[12.5px]`}>{p.baseUrl}</td>
                <td className={`${td} font-mono text-[12.5px]`}>{`••••••••${p.tokenTail}`}</td>
                <td className={`${td} whitespace-nowrap`}>{registered.format(new Date(p.createdAt))}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {load.status === 'loading' && <Empty title={t('common.loading')} />}
        {load.status === 'error' && (
          <Empty title={errorsByField(load.error)[''] ?? t('platforms.loadFailed')}>
            <Button small onClick={reload}>
              {t('common.retry')}
            </Button>
          </Empty>
        )}
        {load.status === 'ok' && items.length === 0 && <Empty title={t('platforms.empty.title')}>{t('platforms.empty.body')}</Empty>}
      </div>

      <RegisterDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onRegistered={() => {
          setDialogOpen(false);
          reload();
        }}
      />
    </>
  );
}

function RegisterDialog({ open, onOpenChange, onRegistered }: { open: boolean; onOpenChange: (open: boolean) => void; onRegistered: () => void }) {
  const { t } = useT();
  const [kind, setKind] = useState<PlatformKind>('gitlab');
  const [baseUrl, setBaseUrl] = useState('');
  const [token, setToken] = useState('');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors: Record<string, string> = error ? errorsByField(error) : {};

  // A fresh form every time the dialog opens; the token is never kept around.
  useEffect(() => {
    if (open) {
      setKind('gitlab');
      setBaseUrl('');
      setToken('');
      setError(null);
    }
  }, [open]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.codePlatforms.create({ kind, baseUrl, token });
      setToken('');
      onRegistered();
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 bg-black/30" />
        <Dialog.Popup className="fixed left-1/2 top-[14vh] w-[480px] -translate-x-1/2 rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-4 text-xl font-bold">{t('platforms.register')}</Dialog.Title>
          <form onSubmit={submit} noValidate className="grid gap-3.5">
            {errors[''] && (
              <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                {errors['']}
              </p>
            )}
            <Field label={t('platforms.field.kind')} error={errors.kind}>
              <div>
                <Segmented
                  label={t('platforms.field.kind')}
                  value={kind}
                  onChange={setKind}
                  options={[
                    { value: 'gitlab', label: KIND_LABEL.gitlab },
                    { value: 'github', label: KIND_LABEL.github },
                  ]}
                />
              </div>
            </Field>
            <Field label={t('platforms.field.baseUrl')} error={errors.baseUrl}>
              <Input name="baseUrl" autoFocus inputMode="url" placeholder="https://gitlab.example.com" value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} />
              <p className="mt-1 text-[13px] text-text-3">{t('platforms.hint.baseUrl')}</p>
            </Field>
            <Field label={t('platforms.field.token')} error={errors.token}>
              <Input name="token" type="password" autoComplete="off" value={token} onChange={(e) => setToken(e.target.value)} />
              <p className="mt-1 text-[13px] text-text-3">{t('platforms.hint.token')}</p>
            </Field>
            <div className="flex justify-end gap-2 pt-1">
              <Dialog.Close render={<Button type="button" />}>{t('platforms.cancel')}</Dialog.Close>
              <Button kind="primary" type="submit" disabled={busy}>
                {busy ? t('platforms.saving') : t('platforms.save')}
              </Button>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
