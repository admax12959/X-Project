// Register-version dialog (Owners). It names the target line (its Git branch) and the
// version currently latest on it, so the wrong line is easy to spot. The line is a
// prop: the dialog works for any line; the Versions tab offers the mainline for now.
import { Dialog } from '@base-ui/react/dialog';
import { useState, type FormEvent } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { Line, RefKind, Version } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button, Field, Input, Segmented, Textarea } from '../ui/controls.tsx';

/** Today's date in the browser's own time zone, "YYYY-MM-DD" (toISOString would give the UTC date). */
export function localToday(now = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`;
}

export function RegisterVersionDialog({
  appKey,
  line,
  previous,
  onClose,
  onRegistered,
}: {
  appKey: string;
  line: Line;
  /** The latest version on this line, if any. */
  previous: Version | undefined;
  onClose: () => void;
  onRegistered: (v: Version) => void;
}) {
  const { t } = useT();
  const [number, setNumber] = useState('');
  const [kind, setKind] = useState<RefKind>('tag');
  const [value, setValue] = useState('');
  const [releasedAt, setReleasedAt] = useState(localToday);
  const [note, setNote] = useState('');
  // The raw error is kept and rendered at render time, so messages follow a language switch.
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors = error ? errorsByField(error) : {};

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      onRegistered(await api.applications.registerVersion(appKey, line.id, { number, ref: { kind, value }, releasedAt, note }));
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
        <Dialog.Popup className="fixed left-1/2 top-[8vh] max-h-[84vh] w-[520px] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-4 text-xl font-bold">{t('versions.register.title')}</Dialog.Title>
          <dl className="mb-4 grid grid-cols-[120px_1fr] gap-x-4 gap-y-1.5 rounded border border-sep bg-subtle px-3 py-2.5 text-[13px]">
            <dt className="text-text-3">{t('versions.dialog.line')}</dt>
            <dd>{line.isMain ? t('term.mainline') : line.name}</dd>
            <dt className="text-text-3">{t('versions.dialog.branch')}</dt>
            <dd className="font-mono text-[12.5px]">{line.branch}</dd>
            <dt className="text-text-3">{t('versions.dialog.previous')}</dt>
            <dd>
              {previous ? (
                <>
                  <span className="font-mono text-[12.5px]">{previous.number}</span>
                  <span className="text-text-3"> · {previous.releasedAt}</span>
                </>
              ) : (
                <span className="text-text-3">{t('versions.dialog.noVersions')}</span>
              )}
            </dd>
          </dl>
          <form onSubmit={submit} noValidate className="grid gap-3.5">
            {errors[''] && (
              <p role="alert" className="rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
                {errors['']}
              </p>
            )}
            <Field label={t('versions.field.number')} error={errors.number}>
              <Input name="number" autoFocus className="font-mono" value={number} onChange={(e) => setNumber(e.target.value)} />
              <p className="mt-1 text-[13px] text-text-3">{t('versions.hint.number')}</p>
            </Field>
            <Field label={t('versions.field.ref')} error={errors['ref.kind'] ?? errors['ref.value']}>
              <div className="flex gap-2">
                <Segmented
                  label={t('versions.field.ref')}
                  value={kind}
                  onChange={setKind}
                  options={[
                    { value: 'tag', label: t('versions.ref.tag') },
                    { value: 'commit', label: t('versions.ref.commit') },
                  ]}
                />
                <Input name="refValue" className="font-mono" value={value} onChange={(e) => setValue(e.target.value)} />
              </div>
              <p className="mt-1 text-[13px] text-text-3">{t('versions.hint.ref')}</p>
            </Field>
            <Field label={t('versions.field.releasedAt')} error={errors.releasedAt}>
              <Input name="releasedAt" type="date" value={releasedAt} onChange={(e) => setReleasedAt(e.target.value)} />
            </Field>
            <Field label={t('versions.field.note')} error={errors.note}>
              <Textarea name="note" value={note} onChange={(e) => setNote(e.target.value)} />
            </Field>
            <div className="flex items-center justify-end gap-2 pt-1">
              <Dialog.Close render={<Button type="button" />}>{t('versions.cancel')}</Dialog.Close>
              <Button kind="primary" type="submit" disabled={busy}>
                {busy ? t('versions.saving') : t('versions.save')}
              </Button>
            </div>
          </form>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
