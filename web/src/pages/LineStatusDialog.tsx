// Confirmation dialog for a status change of a maintenance line (Owners). It shows the
// change (from -> to) and says what the target status means, so the effect is known
// before it is applied. Retired is final and says so in an emphasised block.
import { Dialog } from '@base-ui/react/dialog';
import { useState } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { LineStatus, LineWithLog } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button } from '../ui/controls.tsx';
import { LineStatusLabel } from '../ui/LineStatusLabel.tsx';

export function LineStatusDialog({
  appKey,
  line,
  target,
  onClose,
  onChanged,
}: {
  appKey: string;
  line: { id: string; name: string; status?: LineStatus };
  target: LineStatus;
  onClose: () => void;
  onChanged: (line: LineWithLog) => void;
}) {
  const { t } = useT();
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const errors = error ? errorsByField(error) : {};
  const message = errors[''] ?? errors.status;

  const confirm = async () => {
    setBusy(true);
    setError(null);
    try {
      onChanged(await api.applications.setLineStatus(appKey, line.id, target));
    } catch (err) {
      setError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog.Root open onOpenChange={(open) => !open && !busy && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 bg-black/30" />
        <Dialog.Popup className="fixed left-1/2 top-[12vh] max-h-[84vh] w-[480px] max-w-[95vw] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-3 text-xl font-bold">{t(`lineage.statusDialog.title.${target}`, { name: line.name })}</Dialog.Title>
          <div className="mb-3 flex items-center gap-2 rounded border border-sep bg-subtle px-3 py-2 text-[13px]">
            <LineStatusLabel status={line.status} />
            <span aria-hidden className="text-text-3">
              →
            </span>
            <span className="sr-only">{t('lineage.statusDialog.to')}</span>
            <LineStatusLabel status={target} />
          </div>
          <p className="mb-2 text-text-2">{t(`lineage.statusDialog.meaning.${target}`)}</p>
          <ul className="mb-3 list-disc space-y-1 pl-5 text-[13px] text-text-2">
            <li>{t(`lineage.statusDialog.effect.${target}.1`)}</li>
            <li>{t(`lineage.statusDialog.effect.${target}.2`)}</li>
          </ul>
          {target === 'retired' && (
            <p className="mb-3 rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] font-semibold text-danger">{t('lineage.statusDialog.final')}</p>
          )}
          {message && (
            <p role="alert" className="mb-3 rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
              {message}
            </p>
          )}
          <div className="flex items-center justify-end gap-2 pt-1">
            <Dialog.Close render={<Button type="button" />}>{t('lineage.statusDialog.cancel')}</Dialog.Close>
            <Button kind={target === 'retired' ? 'danger' : 'primary'} disabled={busy} onClick={confirm}>
              {busy ? t('lineage.statusDialog.saving') : t(`lineage.action.${target}`)}
            </Button>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
