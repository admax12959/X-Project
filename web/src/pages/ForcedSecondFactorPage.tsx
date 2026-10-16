// Shown instead of the whole app while the user is required to have a second factor and has none (#23): the setup flow
// of the Security tab, nothing else. Continuing is offered once it is on and the recovery codes were saved.
import { useState } from 'react';
import { useT } from '../i18n/index.ts';
import { useSession } from '../session/session.tsx';
import { LanguageSwitch } from '../shell/Shell.tsx';
import { Button } from '../ui/controls.tsx';
import { SecondFactorPanel } from './SecondFactorPanel.tsx';

export function ForcedSecondFactorPage({ onDone }: { onDone: () => void }) {
  const { t } = useT();
  const { logout } = useSession();
  const [ready, setReady] = useState(false);
  return (
    <div className="flex min-h-screen flex-col items-center bg-subtle pt-[12vh]">
      <div className="mb-4 text-lg font-bold">{t('app.brand')}</div>
      <div className="w-[520px] rounded border border-line bg-white p-6">
        <div className="mb-2 flex items-center justify-between">
          <h1 className="text-xl font-bold">{t('secondFactor.forced.title')}</h1>
          <LanguageSwitch />
        </div>
        <p className="mb-4 text-[13px] text-text-2">{t('secondFactor.forced.body')}</p>
        <SecondFactorPanel onReady={setReady} />
        {ready && (
          <div className="mt-4">
            <Button kind="primary" onClick={onDone}>
              {t('secondFactor.forced.continue')}
            </Button>
          </div>
        )}
        <button type="button" className="mt-4 block text-[13px] text-accent hover:underline" onClick={() => void logout()}>
          {t('password.forced.logout')}
        </button>
      </div>
    </div>
  );
}
