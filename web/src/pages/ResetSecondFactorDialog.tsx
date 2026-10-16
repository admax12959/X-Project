// Administrator resets the two-factor authentication of a user (#23): a confirmation, then POST .../second-factor/reset.
import { Dialog } from '@base-ui/react/dialog';
import { useState } from 'react';
import { api } from '../api/client.ts';
import { errorsByField } from '../api/errors.ts';
import type { AdminUser } from '../api/types.ts';
import { useT } from '../i18n/index.ts';
import { Button } from '../ui/controls.tsx';

export function ResetSecondFactorDialog({ user, onClose, onReset }: { user: AdminUser; onClose: () => void; onReset: (u: AdminUser) => void }) {
  const { t } = useT();
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);
  const message = error ? errorsByField(error)[''] : undefined;

  const reset = async () => {
    setBusy(true);
    setError(null);
    try {
      onReset(await api.users.resetSecondFactor(user.id));
    } catch (err) {
      setError(err);
      setBusy(false);
    }
  };

  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className="fixed inset-0 bg-black/30" />
        <Dialog.Popup className="fixed left-1/2 top-[10vh] max-h-[80vh] w-[480px] -translate-x-1/2 overflow-y-auto rounded border border-line bg-white p-6">
          <Dialog.Title className="mb-3 text-lg font-bold">{t('users.secondFactor.reset.title', { name: user.displayName })}</Dialog.Title>
          {message && (
            <p role="alert" className="mb-3 rounded border border-danger/40 bg-danger/5 px-3 py-2 text-[13px] text-danger">
              {message}
            </p>
          )}
          <p className="text-[13px] text-text-2">{t('users.secondFactor.reset.body')}</p>
          <div className="mt-5 flex justify-end gap-2">
            <Button type="button" onClick={onClose}>
              {t('users.cancel')}
            </Button>
            <Button kind="danger" type="button" disabled={busy} onClick={() => void reset()} autoFocus>
              {busy ? t('users.secondFactor.reset.submitting') : t('users.secondFactor.reset.submit')}
            </Button>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
