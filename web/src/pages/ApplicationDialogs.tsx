// Dialogs for maintaining an application (#7): basic info, Owner/PE lists, deployment info, deletion.
// Which buttons appear is decided by the overview page; the server decides what is allowed.
import { Dialog } from '@base-ui/react/dialog';
import { useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { Application, Member } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button, Field, Input, Textarea } from '../ui/controls.tsx';
import { UserPicker, type PickerUser } from './UserPicker.tsx';

function Frame({ title, onClose, onSubmit, children, submit }: { title: ReactNode; onClose: () => void; onSubmit: (e: FormEvent) => void; children: ReactNode; submit: ReactNode }) {
  const { t } = useT();
  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 bg-black/30" />
        <Dialog.Popup className="fixed left-1/2 top-[6vh] max-h-[88vh] w-[560px] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-4 text-xl font-bold">{title}</Dialog.Title>
          <form onSubmit={onSubmit} noValidate className="grid gap-3.5">
            {children}
            <div className="flex items-center justify-end gap-2 pt-1">
              <Dialog.Close render={<Button type="button" />}>{t('apps.cancel')}</Dialog.Close>
              {submit}
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function RequestError({ message }: { message?: string }) {
  if (!message) return null;
  return (
    <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
      {message}
    </p>
  );
}

/** Shared submit logic: keeps the raw error so messages follow a language switch. */
function useSubmit(run: () => Promise<void>) {
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await run();
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };
  return { submit, busy, errors: error ? errorsByField(error) : ({} as Record<string, string>) };
}

function SaveButton({ busy, disabled, danger, children }: { busy: boolean; disabled?: boolean; danger?: boolean; children: ReactNode }) {
  return (
    <Button kind={danger ? 'danger' : 'primary'} type="submit" disabled={busy || disabled}>
      {children}
    </Button>
  );
}

/** Name, description and mainline branch (Owners). Key and repository are shown but cannot be changed. */
export function EditApplicationDialog({ app, onClose, onSaved }: { app: Application; onClose: () => void; onSaved: (a: Application) => void }) {
  const { t } = useT();
  const [form, setForm] = useState({ name: app.name, description: app.description, mainBranch: app.mainBranch });
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [k]: e.target.value }));
  const { submit, busy, errors } = useSubmit(async () => onSaved(await api.applications.update(app.key, form)));
  return (
    <Frame
      title={t('apps.edit.title')}
      onClose={onClose}
      onSubmit={submit}
      submit={<SaveButton busy={busy}>{busy ? t('apps.saving.edit') : t('apps.save.edit')}</SaveButton>}
    >
      <RequestError message={errors['']} />
      <Field label={t('apps.field.key')}>
        <Input readOnly value={app.key} className="bg-subtle font-mono" />
        <p className="mt-1 text-[13px] text-text-3">{t('apps.hint.keyFixed')}</p>
      </Field>
      <Field label={t('apps.field.repoPath')}>
        <Input readOnly value={app.repo.path} className="bg-subtle font-mono" />
        <p className="mt-1 text-[13px] text-text-3">{t('apps.hint.repoFixed')}</p>
      </Field>
      <Field label={t('apps.field.name')} error={errors.name}>
        <Input name="name" autoFocus value={form.name} onChange={set('name')} />
      </Field>
      <Field label={t('apps.field.description')} error={errors.description}>
        <Textarea name="description" value={form.description} onChange={set('description')} />
      </Field>
      <Field label={t('apps.field.mainBranch')} error={errors.mainBranch}>
        <Input name="mainBranch" className="font-mono" value={form.mainBranch} onChange={set('mainBranch')} />
      </Field>
    </Frame>
  );
}

const asPicker = (m: Member): PickerUser => ({ id: m.id, username: m.username, displayName: m.displayName, disabled: m.status === 'disabled' });

/** Owner and PE lists (administrators and Owners). Only enabled users can be added; people already listed stay visible even if disabled since. */
export function EditMembersDialog({ app, onClose, onSaved }: { app: Application; onClose: () => void; onSaved: (a: Application) => void }) {
  const { t } = useT();
  const [candidates, setCandidates] = useState<PickerUser[] | null>(null);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [owners, setOwners] = useState(app.owners.map((m) => m.id));
  const [pes, setPes] = useState(app.pes.map((m) => m.id));

  useEffect(() => {
    let live = true;
    api.assignableUsers.list().then(
      (r) => {
        if (!live) return;
        const byId = new Map<string, PickerUser>(r.items.map((u) => [u.id, u]));
        for (const m of [...app.owners, ...app.pes]) if (!byId.has(m.id)) byId.set(m.id, asPicker(m));
        setCandidates([...byId.values()]);
      },
      (e: unknown) => live && setLoadError(e),
    );
    return () => {
      live = false;
    };
  }, [app]);

  const { submit, busy, errors } = useSubmit(async () => onSaved(await api.applications.setMembers(app.key, { owners, pes })));
  const loadMessage = loadError ? (errorsByField(loadError)[''] ?? t('apps.picker.loadFailed')) : undefined;
  return (
    <Frame
      title={t('apps.members.title')}
      onClose={onClose}
      onSubmit={submit}
      submit={
        <SaveButton busy={busy} disabled={candidates === null || owners.length === 0}>
          {busy ? t('apps.saving.edit') : t('apps.save.edit')}
        </SaveButton>
      }
    >
      <RequestError message={errors[''] ?? loadMessage} />
      {candidates === null && !loadError && <p className="text-text-2">{t('common.loading')}</p>}
      {candidates !== null && (
        <>
          <UserPicker label={t('apps.field.owners')} hint={t('apps.hint.ownersEdit')} users={candidates} selected={owners} onChange={setOwners} error={errors.owners} />
          <UserPicker label={t('apps.field.pes')} hint={t('apps.hint.pes')} users={candidates} selected={pes} onChange={setPes} error={errors.pes} />
        </>
      )}
    </Frame>
  );
}

/** Deployment info: free text, edited by Owners and PEs. */
export function EditDeployInfoDialog({ app, onClose, onSaved }: { app: Application; onClose: () => void; onSaved: (a: Application) => void }) {
  const { t } = useT();
  const [text, setText] = useState(app.deployInfo);
  const { submit, busy, errors } = useSubmit(async () => onSaved(await api.applications.setDeployInfo(app.key, text)));
  return (
    <Frame
      title={t('apps.deploy.title')}
      onClose={onClose}
      onSubmit={submit}
      submit={<SaveButton busy={busy}>{busy ? t('apps.saving.edit') : t('apps.save.edit')}</SaveButton>}
    >
      <RequestError message={errors['']} />
      <Field label={t('term.deploymentInfo')} error={errors.deployInfo}>
        <Textarea name="deployInfo" autoFocus rows={10} className="font-mono text-[13px]" value={text} onChange={(e) => setText(e.target.value)} />
        <p className="mt-1 text-[13px] text-text-3">{t('apps.hint.deployInfo')}</p>
      </Field>
    </Frame>
  );
}

/** Administrators delete an application after typing its key. */
export function DeleteApplicationDialog({ app, onClose, onDeleted }: { app: Application; onClose: () => void; onDeleted: () => void }) {
  const { t } = useT();
  const [typed, setTyped] = useState('');
  const { submit, busy, errors } = useSubmit(async () => {
    await api.applications.remove(app.key, typed);
    onDeleted();
  });
  return (
    <Frame
      title={t('apps.delete.title')}
      onClose={onClose}
      onSubmit={submit}
      submit={
        <SaveButton busy={busy} disabled={typed.trim() !== app.key} danger>
          {busy ? t('apps.delete.deleting') : t('apps.delete.confirm')}
        </SaveButton>
      }
    >
      <RequestError message={errors[''] ?? errors.confirmKey} />
      <p>{t('apps.delete.warning', { name: app.name })}</p>
      <Field label={t('apps.delete.confirmLabel', { key: app.key })}>
        <Input name="confirmKey" autoFocus className="font-mono" value={typed} onChange={(e) => setTyped(e.target.value)} />
      </Field>
    </Frame>
  );
}
