// Create-application dialog (administrators). The repository must be verified
// first; the server verifies again on create and takes the project ID from the
// platform, never from this form.
import { Dialog } from '@base-ui/react/dialog';
import { useEffect, useState, type FormEvent } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { AdminUser, Application, CodePlatform, Repo } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button, Field, Input, Select, Textarea } from '../ui/controls.tsx';
import { PLATFORM_LABEL } from '../ui/labels.ts';
import { UserPicker } from './UserPicker.tsx';

type Choices =
  | { status: 'loading' }
  | { status: 'error'; error: unknown }
  | { status: 'ok'; platforms: CodePlatform[]; users: AdminUser[] };

export function CreateApplicationDialog({ onClose, onCreated }: { onClose: () => void; onCreated: (a: Application) => void }) {
  const { t } = useT();
  const [choices, setChoices] = useState<Choices>({ status: 'loading' });
  const [form, setForm] = useState({ key: '', name: '', description: '', platformId: '', path: '', mainBranch: '' });
  const [owners, setOwners] = useState<string[]>([]);
  const [pes, setPes] = useState<string[]>([]);
  const [verified, setVerified] = useState<Repo | null>(null);
  const [verifying, setVerifying] = useState(false);
  // Raw errors are kept and rendered at render time, so messages follow a language switch.
  const [verifyError, setVerifyError] = useState<unknown>(null);
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let live = true;
    Promise.all([api.codePlatforms.list(), api.users.list()]).then(
      ([p, u]) => {
        if (!live) return;
        setChoices({ status: 'ok', platforms: p.items, users: u.items.filter((x) => x.status === 'enabled') });
        setForm((f) => ({ ...f, platformId: p.items[0]?.id ?? '' }));
      },
      (e: unknown) => live && setChoices({ status: 'error', error: e }),
    );
    return () => {
      live = false;
    };
  }, []);

  const errors: Record<string, string> = { ...(verifyError ? errorsByField(verifyError) : {}), ...(error ? errorsByField(error) : {}) };
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => {
    setForm((f) => ({ ...f, [k]: e.target.value }));
    if (k === 'platformId' || k === 'path') {
      setVerified(null);
      setVerifyError(null);
    }
  };
  const repoVerified = verified !== null && verified.platformId === form.platformId && verified.path === form.path.trim().replace(/^\/+|\/+$/g, '');

  const verify = async () => {
    setVerifying(true);
    setVerifyError(null);
    setVerified(null);
    try {
      const r = await api.repos.verify({ platformId: form.platformId, path: form.path });
      setVerified(r.repo);
    } catch (err) {
      setVerifyError(err);
    } finally {
      setVerifying(false);
    }
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const a = await api.applications.create({
        key: form.key,
        name: form.name,
        description: form.description,
        repo: { platformId: form.platformId, path: form.path },
        mainBranch: form.mainBranch,
        owners,
        pes,
      });
      onCreated(a);
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  const ready = choices.status === 'ok';
  const platforms = ready ? choices.platforms : [];
  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 bg-black/30" />
        <Dialog.Popup className="fixed left-1/2 top-[6vh] max-h-[88vh] w-[560px] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-4 text-xl font-bold">{t('apps.create.title')}</Dialog.Title>
          {choices.status === 'loading' && <p className="text-text-2">{t('common.loading')}</p>}
          {choices.status === 'error' && (
            <p role="alert" className="text-danger">
              {errorsByField(choices.error)[''] ?? t('apps.picker.loadFailed')}
            </p>
          )}
          {ready && (
            <form onSubmit={submit} noValidate className="grid gap-3.5">
              {errors[''] && (
                <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                  {errors['']}
                </p>
              )}
              <Field label={t('apps.field.key')} error={errors.key}>
                <Input name="key" autoFocus className="font-mono" value={form.key} onChange={set('key')} />
                <p className="mt-1 text-[13px] text-text-3">{t('apps.hint.key')}</p>
              </Field>
              <Field label={t('apps.field.name')} error={errors.name}>
                <Input name="name" value={form.name} onChange={set('name')} />
              </Field>
              <Field label={t('apps.field.description')} error={errors.description}>
                <Textarea name="description" value={form.description} onChange={set('description')} />
              </Field>

              <Field label={t('apps.field.platform')} error={errors['repo.platformId']}>
                {platforms.length === 0 ? (
                  <p className="text-text-2">{t('apps.noPlatforms')}</p>
                ) : (
                  <Select name="platformId" value={form.platformId} onChange={set('platformId')}>
                    {platforms.map((p) => (
                      <option key={p.id} value={p.id}>
                        {PLATFORM_LABEL[p.kind]} · {p.baseUrl}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
              <Field label={t('apps.field.repoPath')} error={errors['repo.path']}>
                <div className="flex gap-2">
                  <Input name="repoPath" className="font-mono" placeholder="group/repo" value={form.path} onChange={set('path')} />
                  <Button type="button" onClick={verify} disabled={verifying || !form.platformId || !form.path.trim()}>
                    {verifying ? t('apps.verifying') : t('apps.verify')}
                  </Button>
                </div>
                {repoVerified && verified ? (
                  <p role="status" className="mt-1 text-[13px] text-ok">
                    {t('apps.verified', { id: verified.projectId })}
                  </p>
                ) : (
                  <p className="mt-1 text-[13px] text-text-3">{t('apps.hint.repoPath')}</p>
                )}
              </Field>

              <Field label={t('apps.field.mainBranch')} error={errors.mainBranch}>
                <Input name="mainBranch" className="font-mono" placeholder="main" value={form.mainBranch} onChange={set('mainBranch')} />
                <p className="mt-1 text-[13px] text-text-3">{t('apps.hint.mainBranch')}</p>
              </Field>
              <UserPicker label={t('apps.field.owners')} hint={t('apps.hint.owners')} users={choices.users} selected={owners} onChange={setOwners} error={errors.owners} />
              <UserPicker label={t('apps.field.pes')} hint={t('apps.hint.pes')} users={choices.users} selected={pes} onChange={setPes} error={errors.pes} />

              <div className="flex items-center justify-end gap-2 pt-1">
                {!repoVerified && <span className="mr-auto text-[13px] text-text-3">{t('apps.verifyFirst')}</span>}
                <Dialog.Close render={<Button type="button" />}>{t('apps.cancel')}</Dialog.Close>
                <Button kind="primary" type="submit" disabled={busy || !repoVerified}>
                  {busy ? t('apps.saving') : t('apps.save')}
                </Button>
              </div>
            </form>
          )}
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
