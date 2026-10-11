// Fork dialog (Owners): creates an active maintenance line from any version of the
// application, including a version on a retired line. It says where the source version
// lives (line, Git branch, status of that line) and when it was released, so a fork from
// a frozen or retired line is a conscious choice. The name of the line is user input.
import { Dialog } from '@base-ui/react/dialog';
import { useState, type FormEvent } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { LineDetail, Version } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button, Field, Input, Select, Textarea } from '../ui/controls.tsx';
import { LineStatusLabel } from '../ui/LineStatusLabel.tsx';

export function ForkDialog({
  appKey,
  versions,
  lines,
  sourceId,
  onClose,
  onForked,
}: {
  appKey: string;
  /** All versions of the application (any line can hold the source). */
  versions: Version[];
  /** All lines, to look up the status of the line that holds a version. */
  lines: LineDetail[];
  /** The version preselected as the source. */
  sourceId: string;
  onClose: () => void;
  onForked: (line: LineDetail) => void;
}) {
  const { t } = useT();
  const [source, setSource] = useState(sourceId);
  const [name, setName] = useState('');
  const [branch, setBranch] = useState('');
  const [reason, setReason] = useState('');
  const [plannedEnd, setPlannedEnd] = useState('');
  const [note, setNote] = useState('');
  // The raw error is kept and rendered at render time, so messages follow a language switch.
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors = error ? errorsByField(error) : {};

  const sourceVersion = versions.find((v) => v.id === source);
  const sourceLine = lines.find((l) => l.id === sourceVersion?.lineId);
  const lineName = (l: { isMain: boolean; name: string }) => (l.isMain ? t('term.mainline') : l.name);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      onForked(await api.applications.fork(appKey, { sourceVersionId: source, name, branch, reason, plannedEnd, note }));
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
        <Dialog.Popup className="fixed left-1/2 top-[6vh] max-h-[88vh] w-[620px] max-w-[95vw] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-4 text-xl font-bold">{t('lineage.fork.title')}</Dialog.Title>
          <form onSubmit={submit} noValidate className="grid gap-3.5">
            {errors[''] && (
              <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                {errors['']}
              </p>
            )}
            <Field label={t('lineage.fork.source')} error={errors.sourceVersionId}>
              <Select name="sourceVersionId" value={source} onChange={(e) => setSource(e.target.value)}>
                {lines
                  .filter((l) => versions.some((v) => v.lineId === l.id))
                  .map((l) => (
                    <optgroup key={l.id} label={`${lineName(l)} · ${l.branch}`}>
                      {versions
                        .filter((v) => v.lineId === l.id)
                        .map((v) => (
                          <option key={v.id} value={v.id}>
                            {v.number}
                          </option>
                        ))}
                    </optgroup>
                  ))}
              </Select>
            </Field>
            {sourceVersion && sourceLine && (
              <dl className="grid grid-cols-[150px_1fr] gap-x-4 gap-y-1.5 rounded border border-sep bg-subtle px-3 py-2.5 text-[13px]">
                <dt className="text-text-3">{t('lineage.fork.sourceLine')}</dt>
                <dd>{lineName(sourceLine)}</dd>
                <dt className="text-text-3">{t('lineage.fork.sourceBranch')}</dt>
                <dd className="font-mono text-[12.5px]">{sourceLine.branch}</dd>
                <dt className="text-text-3">{t('lineage.fork.sourceStatus')}</dt>
                <dd>{sourceLine.isMain ? <span className="text-text-3">—</span> : <LineStatusLabel status={sourceLine.status} />}</dd>
                <dt className="text-text-3">{t('lineage.fork.sourceReleased')}</dt>
                <dd>{sourceVersion.releasedAt}</dd>
                {sourceLine.status === 'retired' && <dd className="col-span-2 text-text-2">{t('lineage.fork.retiredNote')}</dd>}
              </dl>
            )}
            <Field label={t('lineage.fork.name')} error={errors.name}>
              <Input name="name" autoFocus value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Field label={t('lineage.fork.branch')} error={errors.branch}>
              <Input name="branch" className="font-mono" placeholder="cust/acme" value={branch} onChange={(e) => setBranch(e.target.value)} />
              <p className="mt-1 text-[13px] text-text-3">{t('lineage.fork.branchHint')}</p>
            </Field>
            <Field label={t('lineage.fork.reason')} error={errors.reason}>
              <Textarea name="reason" value={reason} onChange={(e) => setReason(e.target.value)} />
              <p className="mt-1 text-[13px] text-text-3">{t('lineage.fork.reasonHint')}</p>
            </Field>
            <details className="rounded border border-sep px-3 py-2" open={!!(errors.plannedEnd || errors.note)}>
              <summary className="cursor-pointer text-[13px] font-semibold">{t('lineage.fork.optional')}</summary>
              <div className="mt-3 grid gap-3.5">
                <Field label={t('lineage.fork.plannedEnd')} error={errors.plannedEnd}>
                  <Input name="plannedEnd" type="date" value={plannedEnd} onChange={(e) => setPlannedEnd(e.target.value)} />
                  <p className="mt-1 text-[13px] text-text-3">{t('lineage.fork.plannedEndHint')}</p>
                </Field>
                <Field label={t('lineage.fork.note')} error={errors.note}>
                  <Textarea name="note" value={note} onChange={(e) => setNote(e.target.value)} />
                </Field>
              </div>
            </details>
            <p className="text-[13px] text-text-3">{t('lineage.fork.newLine')}</p>
            <div className="flex items-center justify-end gap-2 pt-1">
              <Dialog.Close render={<Button type="button" />}>{t('lineage.fork.cancel')}</Dialog.Close>
              <Button kind="primary" type="submit" disabled={busy}>
                {busy ? t('lineage.fork.saving') : t('lineage.fork.submit')}
              </Button>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
