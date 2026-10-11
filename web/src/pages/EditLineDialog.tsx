// Edit dialog for a maintenance line (Owners): name, note and planned end date. The Git
// branch, the source version and the fork reason are not offered: they describe the fork
// as it happened and have no edit path. The planned end date is information only.
import { Dialog } from '@base-ui/react/dialog';
import { useState, type FormEvent } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { LineDetail, LineWithLog } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button, Field, Input, Textarea } from '../ui/controls.tsx';

export function EditLineDialog({
  appKey,
  line,
  onClose,
  onSaved,
}: {
  appKey: string;
  line: LineDetail;
  onClose: () => void;
  onSaved: (line: LineWithLog) => void;
}) {
  const { t } = useT();
  const [name, setName] = useState(line.name);
  const [note, setNote] = useState(line.note ?? '');
  const [plannedEnd, setPlannedEnd] = useState(line.plannedEnd ?? '');
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors = error ? errorsByField(error) : {};

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      onSaved(await api.applications.updateLine(appKey, line.id, { name, note, plannedEnd }));
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
        <Dialog.Popup className="fixed left-1/2 top-[10vh] max-h-[84vh] w-[520px] max-w-[95vw] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-1 text-xl font-bold">{t('lineage.edit.title')}</Dialog.Title>
          <p className="mb-4 text-[13px] text-text-3">{t('lineage.edit.fixed', { branch: line.branch })}</p>
          <form onSubmit={submit} noValidate className="grid gap-3.5">
            {errors[''] && (
              <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                {errors['']}
              </p>
            )}
            <Field label={t('lineage.fork.name')} error={errors.name}>
              <Input name="name" autoFocus value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Field label={t('lineage.fork.plannedEnd')} error={errors.plannedEnd}>
              <Input name="plannedEnd" type="date" value={plannedEnd} onChange={(e) => setPlannedEnd(e.target.value)} />
              <p className="mt-1 text-[13px] text-text-3">{t('lineage.fork.plannedEndHint')}</p>
            </Field>
            <Field label={t('lineage.fork.note')} error={errors.note}>
              <Textarea name="note" value={note} onChange={(e) => setNote(e.target.value)} />
            </Field>
            <div className="flex items-center justify-end gap-2 pt-1">
              <Dialog.Close render={<Button type="button" />}>{t('lineage.fork.cancel')}</Dialog.Close>
              <Button kind="primary" type="submit" disabled={busy}>
                {busy ? t('lineage.edit.saving') : t('lineage.edit.save')}
              </Button>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
